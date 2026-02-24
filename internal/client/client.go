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
	Timeout  time.Duration
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

// ConnectionError wraps errors caused by network/connection failures.
type ConnectionError struct {
	Err error
}

func (e *ConnectionError) Error() string { return e.Err.Error() }
func (e *ConnectionError) Unwrap() error { return e.Err }

// ESError represents a structured Elasticsearch error response (query errors).
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
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
		},
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
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
		return nil, &ConnectionError{Err: fmt.Errorf("connection failed: %w", err)}
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

// IndexInfo represents an index entry from _cat/indices.
type IndexInfo struct {
	Health    string `json:"health"`
	Status    string `json:"status"`
	Index     string `json:"index"`
	UUID      string `json:"uuid"`
	DocsCount string `json:"docs.count"`
	StoreSize string `json:"store.size"`
}

// FieldInfo represents a flattened field from an index mapping.
type FieldInfo struct {
	Name string
	Type string
}

// ListIndices returns index metadata via _cat/indices.
// An optional pattern filters the results (e.g. "logs-*").
func (c *Client) ListIndices(pattern string) ([]IndexInfo, error) {
	path := "/_cat/indices"
	if pattern != "" {
		path += "/" + pattern
	}
	path += "?format=json&h=health,status,index,uuid,docs.count,store.size&s=index"

	body, err := c.doGet(path)
	if err != nil {
		return nil, err
	}

	var indices []IndexInfo
	if err := json.Unmarshal(body, &indices); err != nil {
		return nil, fmt.Errorf("parsing indices response: %w", err)
	}

	return indices, nil
}

// GetFields returns the flattened field list for an index via _mapping.
func (c *Client) GetFields(index string) ([]FieldInfo, error) {
	path := "/" + index + "/_mapping"

	body, err := c.doGet(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]struct {
		Mappings struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing mapping response: %w", err)
	}

	var fields []FieldInfo
	for _, idx := range raw {
		fields = flattenProperties("", idx.Mappings.Properties, fields)
		break
	}

	return fields, nil
}

func flattenProperties(prefix string, props map[string]json.RawMessage, fields []FieldInfo) []FieldInfo {
	for name, rawField := range props {
		fullName := name
		if prefix != "" {
			fullName = prefix + "." + name
		}

		var field struct {
			Type       string                     `json:"type"`
			Properties map[string]json.RawMessage `json:"properties"`
			Fields     map[string]json.RawMessage `json:"fields"`
		}
		if err := json.Unmarshal(rawField, &field); err != nil {
			continue
		}

		if field.Type != "" {
			fields = append(fields, FieldInfo{Name: fullName, Type: field.Type})
		}

		if field.Properties != nil {
			if field.Type == "" {
				fields = append(fields, FieldInfo{Name: fullName, Type: "object"})
			}
			fields = flattenProperties(fullName, field.Properties, fields)
		}

		for subName, subRaw := range field.Fields {
			var sub struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(subRaw, &sub); err == nil && sub.Type != "" {
				fields = append(fields, FieldInfo{Name: fullName + "." + subName, Type: sub.Type})
			}
		}
	}
	return fields
}

// doGet performs an authenticated GET request and returns the response body.
func (c *Client) doGet(path string) ([]byte, error) {
	url := strings.TrimRight(c.config.Host, "/") + path
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
		return nil, &ConnectionError{Err: fmt.Errorf("connection failed: %w", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseESError(resp.StatusCode, body)
	}

	return body, nil
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
