package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// verboseTransport wraps an http.RoundTripper and logs request/response
// details to stderr when --verbose is enabled.
type verboseTransport struct {
	inner http.RoundTripper
}

func (t *verboseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	fmt.Fprintf(os.Stderr, "\n\033[2m> %s %s\033[0m\n", req.Method, req.URL)

	for name, vals := range req.Header {
		val := strings.Join(vals, ", ")
		if strings.EqualFold(name, "Authorization") {
			val = maskAuth(val)
		}
		fmt.Fprintf(os.Stderr, "\033[2m> %s: %s\033[0m\n", name, val)
	}

	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		body := string(bodyBytes)
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		fmt.Fprintf(os.Stderr, "\033[2m> Body: %s\033[0m\n", body)
	}

	resp, err := t.inner.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[2m< Error: %s (%s)\033[0m\n\n", err, elapsed.Round(time.Millisecond))
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "\033[2m< %s (%s)\033[0m\n", resp.Status, elapsed.Round(time.Millisecond))

	if resp.ContentLength >= 0 {
		fmt.Fprintf(os.Stderr, "\033[2m< Content-Length: %d\033[0m\n", resp.ContentLength)
	}

	fmt.Fprintln(os.Stderr)
	return resp, nil
}

func maskAuth(val string) string {
	if strings.HasPrefix(val, "ApiKey ") && len(val) > 13 {
		return "ApiKey " + val[7:13] + "..."
	}
	if strings.HasPrefix(val, "Basic ") && len(val) > 12 {
		return "Basic " + val[6:12] + "..."
	}
	if len(val) > 10 {
		return val[:10] + "..."
	}
	return "***"
}
