package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	Host     string
	User     string
	Password string
	APIKey   string
	Insecure bool
}

type Client struct {
	httpClient *http.Client
	config     Config
}

type QueryRequest struct {
	Query string `json:"query"`
}

type QueryResponse struct {
	Columns []Column        `json:"columns"`
	Values  [][]interface{} `json:"values"`
	Took    int             `json:"took"`
}

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ESError represents a structured Elasticsearch error response.
type ESError struct {
	Status int
	Type   string
	Reason string
	Raw    string
}

func (e *ESError) Error() string {
	if e.Type != "" && e.Reason != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Status, e.Type, e.Reason)
	}
	return fmt.Sprintf("[%d] %s", e.Status, e.Raw)
}

func New(cfg Config) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
		},
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		config: cfg,
	}
}

// ClusterInfo is returned by Ping to verify the connection.
type ClusterInfo struct {
	Name    string `json:"name"`
	Cluster string `json:"cluster_name"`
	Version struct {
		Number string `json:"number"`
	} `json:"version"`
}

// Ping checks connectivity to the Elasticsearch cluster.
func (c *Client) Ping() (*ClusterInfo, error) {
	url := strings.TrimRight(c.config.Host, "/") + "/"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.config.APIKey)
	} else if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseESError(resp.StatusCode, body)
	}

	var info ClusterInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &info, nil
}

func (c *Client) Query(esql string) (*QueryResponse, error) {
	reqBody := QueryRequest{Query: esql}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := strings.TrimRight(c.config.Host, "/") + "/_query"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.config.APIKey)
	} else if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseESError(resp.StatusCode, respBody)
	}

	var result QueryResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &result, nil
}

func parseESError(status int, body []byte) *ESError {
	var parsed struct {
		Error struct {
			RootCause []struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"root_cause"`
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Type != "" {
		reason := parsed.Error.Reason
		typ := parsed.Error.Type
		if len(parsed.Error.RootCause) > 0 {
			reason = parsed.Error.RootCause[0].Reason
			typ = parsed.Error.RootCause[0].Type
		}
		return &ESError{Status: status, Type: typ, Reason: reason, Raw: string(body)}
	}

	return &ESError{Status: status, Raw: string(body)}
}
