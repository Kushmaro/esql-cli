package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	return c.doRequestJSON(ctx, method, path, "application/json", bytes.NewReader(data), result)
}

// doRequestJSON shares authentication, error decoding and cancellation across
// JSON REST calls and Prometheus form-encoded requests.
func (c *Client) doRequestJSON(ctx context.Context, method, path, contentType string, body io.Reader, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.config.Host, "/")+path, body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.config.APIKey)
	} else if c.config.User != "" {
		req.SetBasicAuth(c.config.User, c.config.Password)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ConnectionError{Err: fmt.Errorf("connection failed: %w", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading error response: %w", err)
		}
		return parseESError(resp.StatusCode, data)
	}
	if result == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	return nil
}
