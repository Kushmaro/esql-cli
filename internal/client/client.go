package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// QueryRequest is the body sent to the ES|QL _query endpoint.
type QueryRequest struct {
	Query  string `json:"query"`
	Format string `json:"format,omitempty"`
}

// QueryResponse represents the ES|QL _query response.
type QueryResponse struct {
	Columns []Column        `json:"columns"`
	Values  [][]interface{} `json:"values"`
}

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
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

// Query executes an ES|QL query and returns the parsed response.
func (c *Client) Query(esql string) (*QueryResponse, error) {
	reqBody := QueryRequest{Query: esql}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("%s/_query", c.config.Host)
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
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("elasticsearch returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result QueryResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &result, nil
}
