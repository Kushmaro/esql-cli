package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// QueryRequest is shared by synchronous and asynchronous ES|QL execution.
// Parameters are native JSON values, never interpolated into the query text.
type QueryRequest struct {
	Query                    string                 `json:"query"`
	Params                   []interface{}          `json:"params,omitempty"`
	Settings                 map[string]interface{} `json:"settings,omitempty"`
	TimeZone                 string                 `json:"time_zone,omitempty"`
	Locale                   string                 `json:"locale,omitempty"`
	Profile                  bool                   `json:"profile,omitempty"`
	IncludeExecutionMetadata bool                   `json:"include_execution_metadata,omitempty"`
}

type QueryResponse struct {
	ID        string          `json:"id,omitempty"`
	IsRunning bool            `json:"is_running"`
	IsPartial bool            `json:"is_partial"`
	Columns   []Column        `json:"columns"`
	Values    [][]interface{} `json:"values"`
	Took      int             `json:"took"`
	// Raw preserves profiling, column metadata and version-specific fields for
	// full-response output without forcing them into a lossy client model.
	Raw json.RawMessage `json:"-"`
}

type Column struct {
	Name string          `json:"name"`
	Type string          `json:"type"`
	Meta json.RawMessage `json:"_meta,omitempty"`
}

func (r *QueryResponse) UnmarshalJSON(data []byte) error {
	type response QueryResponse
	var decoded response
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*r = QueryResponse(decoded)
	r.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (c *Client) Query(esql string) (*QueryResponse, error) {
	return c.ExecuteQuery(context.Background(), QueryRequest{Query: esql})
}

func (c *Client) ExecuteQuery(ctx context.Context, request QueryRequest) (*QueryResponse, error) {
	var result QueryResponse
	if err := c.doJSON(ctx, http.MethodPost, "/_query", request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type AsyncQueryRequest struct {
	QueryRequest
	WaitForCompletionTimeout string `json:"wait_for_completion_timeout,omitempty"`
	KeepAlive                string `json:"keep_alive,omitempty"`
	KeepOnCompletion         *bool  `json:"keep_on_completion,omitempty"`
}

type AsyncGetOptions struct {
	WaitForCompletionTimeout string
	KeepAlive                string
}

func (c *Client) SubmitAsync(ctx context.Context, request AsyncQueryRequest) (*QueryResponse, error) {
	var result QueryResponse
	if err := c.doJSON(ctx, http.MethodPost, "/_query/async", request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func asyncPath(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("async query ID is required")
	}
	return "/_query/async/" + url.PathEscape(id), nil
}

func (c *Client) GetAsync(ctx context.Context, id string, options AsyncGetOptions) (*QueryResponse, error) {
	path, err := asyncPath(id)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if options.WaitForCompletionTimeout != "" {
		params.Set("wait_for_completion_timeout", options.WaitForCompletionTimeout)
	}
	if options.KeepAlive != "" {
		params.Set("keep_alive", options.KeepAlive)
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result QueryResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) StopAsync(ctx context.Context, id string) (*QueryResponse, error) {
	path, err := asyncPath(id)
	if err != nil {
		return nil, err
	}
	var result QueryResponse
	if err := c.doJSON(ctx, http.MethodPost, path+"/stop", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAsync(ctx context.Context, id string) (json.RawMessage, error) {
	path, err := asyncPath(id)
	if err != nil {
		return nil, err
	}
	var result json.RawMessage
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Running-query inspection reports active execution, rather than retained results.
func (c *Client) ListRunningQueries(ctx context.Context) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/_query/queries", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetRunningQuery(ctx context.Context, id string) (json.RawMessage, error) {
	if id == "" {
		return nil, fmt.Errorf("running query ID is required")
	}
	var result json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/_query/queries/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
