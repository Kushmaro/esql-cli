package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
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
	Verbose  bool
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

// IsPrivilegeError reports whether err is a 403 from Elasticsearch,
// typically caused by insufficient API key / role privileges.
func IsPrivilegeError(err error) bool {
	var esErr *ESError
	return errors.As(err, &esErr) && esErr.Status == http.StatusForbidden
}

const PrivilegeHint = `Your credentials lack the required index privileges for this command.
Add "view_index_metadata" to your role to use metadata commands (data, fields, \d).
Queries (esql query) will still work with read-only credentials.`

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var transport http.RoundTripper = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
		},
	}

	if cfg.Verbose {
		transport = &verboseTransport{inner: transport}
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
// It first tries GET / for cluster info. If that returns 403 (common with
// read-only API keys that lack cluster:monitor privileges), it falls back
// to running a trivial ES|QL query to verify the connection works.
func (c *Client) Ping() (*ClusterInfo, error) {
	info, err := c.clusterInfo()
	if err == nil {
		return info, nil
	}

	var esErr *ESError
	if !errors.As(err, &esErr) || esErr.Status != http.StatusForbidden {
		return nil, err
	}

	// GET / returned 403 -- the credentials likely lack cluster:monitor.
	// Fall back to a lightweight ES|QL query to confirm the connection works.
	if _, qErr := c.Query("ROW x = 1"); qErr != nil {
		return nil, fmt.Errorf("connection failed: %w (cluster info also unavailable: %w)", qErr, err)
	}

	return &ClusterInfo{Name: "n/a", Cluster: "n/a"}, nil
}

func (c *Client) clusterInfo() (*ClusterInfo, error) {
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

// DataStreamInfo represents a data stream from the _data_stream API.
type DataStreamInfo struct {
	Name               string   `json:"name"`
	TimestampField     string   `json:"timestamp_field"`
	Status             string   `json:"status"`
	Template           string   `json:"template"`
	IndexMode          string   `json:"index_mode"`
	BackingIndices     int      `json:"backing_indices"`
	BackingIndexNames  []string `json:"backing_index_names,omitempty"`
}

// ListDataStreams returns data stream metadata via _data_stream.
func (c *Client) ListDataStreams(pattern string) ([]DataStreamInfo, error) {
	path := "/_data_stream"
	if pattern != "" {
		path += "/" + pattern
	}

	body, err := c.doGet(path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		DataStreams []struct {
			Name           string `json:"name"`
			TimestampField struct {
				Name string `json:"name"`
			} `json:"timestamp_field"`
			Indices   []json.RawMessage `json:"indices"`
			Status    string            `json:"status"`
			Template  string            `json:"template"`
			IndexMode string            `json:"index_mode"`
		} `json:"data_streams"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing data streams response: %w", err)
	}

	streams := make([]DataStreamInfo, len(raw.DataStreams))
	for i, ds := range raw.DataStreams {
		var indexNames []string
		for _, rawIdx := range ds.Indices {
			var idx struct {
				Name string `json:"index_name"`
			}
			if json.Unmarshal(rawIdx, &idx) == nil && idx.Name != "" {
				indexNames = append(indexNames, idx.Name)
			}
		}
		streams[i] = DataStreamInfo{
			Name:              ds.Name,
			TimestampField:    ds.TimestampField.Name,
			Status:            strings.ToLower(ds.Status),
			Template:          ds.Template,
			IndexMode:         ds.IndexMode,
			BackingIndices:    len(ds.Indices),
			BackingIndexNames: indexNames,
		}
	}

	return streams, nil
}

// ListIndices returns index metadata via _cat/indices.
// An optional pattern filters the results (e.g. "logs-*").
func (c *Client) ListIndices(pattern string) ([]IndexInfo, error) {
	path := "/_cat/indices"
	if pattern != "" {
		path += "/" + pattern
	}
	path += "?format=json&h=health,status,index,uuid,docs.count,store.size&bytes=b&s=index"

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

// RemoteClusterInfo represents a configured remote cluster from _remote/info.
type RemoteClusterInfo struct {
	Name            string `json:"name"`
	Connected       bool   `json:"connected"`
	Mode            string `json:"mode"`
	SkipUnavailable bool   `json:"skip_unavailable"`
}

// ListRemoteClusters returns the configured remote clusters via GET /_remote/info.
// Returns an empty slice (no error) if the API is unavailable (e.g., older ES versions
// or insufficient privileges).
func (c *Client) ListRemoteClusters() ([]RemoteClusterInfo, error) {
	body, err := c.doGet("/_remote/info")
	if err != nil {
		var esErr *ESError
		if errors.As(err, &esErr) {
			if esErr.Status == http.StatusNotFound || esErr.Status == http.StatusForbidden {
				return nil, nil
			}
		}
		return nil, err
	}

	var raw map[string]struct {
		Connected       bool   `json:"connected"`
		Mode            string `json:"mode"`
		SkipUnavailable bool   `json:"skip_unavailable"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing remote info response: %w", err)
	}

	clusters := make([]RemoteClusterInfo, 0, len(raw))
	for name, info := range raw {
		clusters = append(clusters, RemoteClusterInfo{
			Name:            name,
			Connected:       info.Connected,
			Mode:            info.Mode,
			SkipUnavailable: info.SkipUnavailable,
		})
	}

	return clusters, nil
}

// ResolvedIndex represents an index from the _resolve/index API.
type ResolvedIndex struct {
	Name string `json:"name"`
}

// ResolvedAlias represents an alias from the _resolve/index API.
type ResolvedAlias struct {
	Name    string   `json:"name"`
	Indices []string `json:"indices"`
}

// ResolvedDataStream represents a data stream from the _resolve/index API.
type ResolvedDataStream struct {
	Name string `json:"name"`
}

// ResolveResult holds the combined output from _resolve/index.
type ResolveResult struct {
	Indices     []ResolvedIndex      `json:"indices"`
	Aliases     []ResolvedAlias      `json:"aliases"`
	DataStreams []ResolvedDataStream  `json:"data_streams"`
}

// ResolveIndex uses the _resolve/index API to discover indices and data streams.
// This API requires only view_index_metadata, unlike _cat/indices which needs monitor.
func (c *Client) ResolveIndex(pattern string) (*ResolveResult, error) {
	if pattern == "" {
		pattern = "*"
	}
	path := "/_resolve/index/" + pattern

	body, err := c.doGet(path)
	if err != nil {
		return nil, err
	}

	var result ResolveResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing resolve response: %w", err)
	}

	return &result, nil
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
