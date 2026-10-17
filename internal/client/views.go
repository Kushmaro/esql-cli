package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ViewRequest defines a server-side view. A non-nil Description also permits
// explicitly clearing an existing description with an empty string.
type ViewRequest struct {
	Query       string  `json:"query"`
	Description *string `json:"description,omitempty"`
}

// GetViews lists all views if names is empty, or retrieves a comma-separated
// selection. The raw response preserves fields added by newer server versions.
func (c *Client) GetViews(ctx context.Context, names string) (json.RawMessage, error) {
	path := "/_query/view"
	if names != "" {
		path += "/" + url.PathEscape(names)
	}
	var result json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) PutView(ctx context.Context, name string, request ViewRequest) (json.RawMessage, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("view name is required")
	}
	if strings.TrimSpace(request.Query) == "" {
		return nil, fmt.Errorf("view query is required")
	}
	var result json.RawMessage
	if err := c.doJSON(ctx, http.MethodPut, "/_query/view/"+url.PathEscape(name), request, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) DeleteViews(ctx context.Context, names string) (json.RawMessage, error) {
	if strings.TrimSpace(names) == "" {
		return nil, fmt.Errorf("view name is required")
	}
	var result json.RawMessage
	if err := c.doJSON(ctx, http.MethodDelete, "/_query/view/"+url.PathEscape(names), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
