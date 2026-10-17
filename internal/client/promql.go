package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// PromQLRequest supports the documented Prometheus-compatible HTTP API.
// Parameters remain strings, including timestamps, selectors and durations.
type PromQLRequest struct {
	Endpoint string // query, query_range, labels, label_values, series, metadata, buildinfo
	Index    string
	Method   string // GET (default) or POST for supported form-encoded endpoints
	Label    string // used only by label_values
	Params   url.Values
}

// PromQL returns the full Prometheus envelope, preserving sample strings (NaN,
// infinities and decimal values), labels, warnings and future response fields.
func (c *Client) PromQL(ctx context.Context, request PromQLRequest) (json.RawMessage, error) {
	route, postAllowed, err := promQLRoute(request.Endpoint, request.Label)
	if err != nil {
		return nil, err
	}
	method := strings.ToUpper(request.Method)
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPost {
		return nil, fmt.Errorf("PromQL method must be GET or POST")
	}
	if method == http.MethodPost && !postAllowed {
		return nil, fmt.Errorf("PromQL %s supports GET only", request.Endpoint)
	}
	path := "/_prometheus/"
	if request.Index != "" {
		path += url.PathEscape(request.Index) + "/"
	}
	path += "api/v1/" + route
	var result json.RawMessage
	if method == http.MethodPost {
		err = c.doRequestJSON(ctx, method, path, "application/x-www-form-urlencoded", strings.NewReader(request.Params.Encode()), &result)
	} else {
		if len(request.Params) > 0 {
			path += "?" + request.Params.Encode()
		}
		err = c.doRequestJSON(ctx, method, path, "", nil, &result)
	}
	if err != nil {
		if esErr, ok := err.(*ESError); ok {
			esErr.Language = "PromQL"
		}
		return nil, err
	}
	var envelope struct {
		Status string `json:"status"`
		Type   string `json:"errorType"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil {
		return nil, fmt.Errorf("parsing PromQL response: %w", err)
	}
	if envelope.Status == "error" {
		return nil, fmt.Errorf("PromQL %s: %s", envelope.Type, envelope.Error)
	}
	if envelope.Status != "success" {
		return nil, fmt.Errorf("PromQL response is missing success status")
	}
	return result, nil
}

func promQLRoute(endpoint, label string) (route string, postAllowed bool, err error) {
	switch endpoint {
	case "query", "query_range", "labels", "series":
		return endpoint, true, nil
	case "metadata":
		return endpoint, false, nil
	case "buildinfo":
		return "status/buildinfo", false, nil
	case "label_values":
		if label == "" {
			return "", false, fmt.Errorf("label name is required")
		}
		return "label/" + url.PathEscape(label) + "/values", false, nil
	default:
		return "", false, fmt.Errorf("unknown PromQL endpoint %q", endpoint)
	}
}
