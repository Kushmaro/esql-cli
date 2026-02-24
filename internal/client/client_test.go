package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------- Test 5: Client.Query and Client.Ping via httptest ----------

func TestClientQuery(t *testing.T) {
	t.Run("successful query", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/_query" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodPost {
				t.Errorf("unexpected method: %s", r.Method)
			}

			var req QueryRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decoding request body: %v", err)
			}
			if req.Query != "FROM logs | LIMIT 1" {
				t.Errorf("unexpected query: %s", req.Query)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(QueryResponse{
				Columns: []Column{{Name: "host", Type: "keyword"}},
				Values:  [][]interface{}{{"server-1"}},
				Took:    5,
			})
		}))
		defer srv.Close()

		c := New(Config{Host: srv.URL})
		resp, err := c.Query("FROM logs | LIMIT 1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Columns) != 1 || resp.Columns[0].Name != "host" {
			t.Errorf("unexpected columns: %+v", resp.Columns)
		}
		if len(resp.Values) != 1 {
			t.Fatalf("expected 1 row, got %d", len(resp.Values))
		}
		if resp.Took != 5 {
			t.Errorf("took = %d, want 5", resp.Took)
		}
	})

	t.Run("api key auth header", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth != "ApiKey my-secret-key" {
				t.Errorf("auth header = %q, want %q", auth, "ApiKey my-secret-key")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(QueryResponse{})
		}))
		defer srv.Close()

		c := New(Config{Host: srv.URL, APIKey: "my-secret-key"})
		_, err := c.Query("ROW x = 1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("basic auth header", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				t.Fatal("expected basic auth")
			}
			if user != "elastic" || pass != "changeme" {
				t.Errorf("credentials = %s/%s, want elastic/changeme", user, pass)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(QueryResponse{})
		}))
		defer srv.Close()

		c := New(Config{Host: srv.URL, User: "elastic", Password: "changeme"})
		_, err := c.Query("ROW x = 1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ES error response 400", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{
				"error": {
					"root_cause": [{"type": "parsing_exception", "reason": "Unknown command [FORM]"}],
					"type": "parsing_exception",
					"reason": "Unknown command [FORM]"
				},
				"status": 400
			}`))
		}))
		defer srv.Close()

		c := New(Config{Host: srv.URL})
		_, err := c.Query("FORM logs")
		if err == nil {
			t.Fatal("expected error")
		}
		var esErr *ESError
		if !errors.As(err, &esErr) {
			t.Fatalf("expected *ESError, got %T: %v", err, err)
		}
		if esErr.Status != 400 {
			t.Errorf("status = %d, want 400", esErr.Status)
		}
		if esErr.Type != "parsing_exception" {
			t.Errorf("type = %q, want parsing_exception", esErr.Type)
		}
	})

	t.Run("connection error", func(t *testing.T) {
		c := New(Config{Host: "http://127.0.0.1:1"}) // port 1 should refuse
		_, err := c.Query("ROW x = 1")
		if err == nil {
			t.Fatal("expected connection error")
		}
		var connErr *ConnectionError
		if !errors.As(err, &connErr) {
			t.Fatalf("expected *ConnectionError, got %T: %v", err, err)
		}
	})
}

func TestClientPing(t *testing.T) {
	t.Run("successful ping", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"name": "node-1",
				"cluster_name": "my-cluster",
				"version": {"number": "8.15.0"}
			}`))
		}))
		defer srv.Close()

		c := New(Config{Host: srv.URL})
		info, err := c.Ping()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Name != "node-1" {
			t.Errorf("name = %q, want node-1", info.Name)
		}
		if info.Cluster != "my-cluster" {
			t.Errorf("cluster = %q, want my-cluster", info.Cluster)
		}
		if info.Version.Number != "8.15.0" {
			t.Errorf("version = %q, want 8.15.0", info.Version.Number)
		}
	})

	t.Run("ping 403 fallback to query", func(t *testing.T) {
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Path == "/" && r.Method == http.MethodGet {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":{"type":"security_exception","reason":"forbidden"},"status":403}`))
				return
			}
			// Fallback query: POST /_query with ROW x = 1
			if r.URL.Path == "/_query" && r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(QueryResponse{
					Columns: []Column{{Name: "x", Type: "integer"}},
					Values:  [][]interface{}{{float64(1)}},
				})
				return
			}
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}))
		defer srv.Close()

		c := New(Config{Host: srv.URL})
		info, err := c.Ping()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Name != "n/a" {
			t.Errorf("name = %q, want n/a (fallback)", info.Name)
		}
		if calls != 2 {
			t.Errorf("expected 2 HTTP calls (GET / + POST /_query), got %d", calls)
		}
	})
}

// ---------- Test 7: parseESError and IsPrivilegeError ----------

func TestParseESError(t *testing.T) {
	t.Run("root_cause extraction", func(t *testing.T) {
		body := `{
			"error": {
				"root_cause": [{"type": "index_not_found_exception", "reason": "no such index [missing]"}],
				"type": "index_not_found_exception",
				"reason": "no such index [missing]"
			},
			"status": 404
		}`
		esErr := parseESError(404, []byte(body))
		if esErr.Status != 404 {
			t.Errorf("status = %d, want 404", esErr.Status)
		}
		if esErr.Type != "index_not_found_exception" {
			t.Errorf("type = %q, want index_not_found_exception", esErr.Type)
		}
		if !strings.Contains(esErr.Reason, "no such index") {
			t.Errorf("reason = %q, should contain 'no such index'", esErr.Reason)
		}
	})

	t.Run("fallback to top-level error", func(t *testing.T) {
		body := `{
			"error": {
				"type": "mapper_parsing_exception",
				"reason": "failed to parse"
			},
			"status": 400
		}`
		esErr := parseESError(400, []byte(body))
		if esErr.Type != "mapper_parsing_exception" {
			t.Errorf("type = %q, want mapper_parsing_exception", esErr.Type)
		}
		if esErr.Reason != "failed to parse" {
			t.Errorf("reason = %q, want 'failed to parse'", esErr.Reason)
		}
	})

	t.Run("non-JSON body", func(t *testing.T) {
		body := "Bad Gateway"
		esErr := parseESError(502, []byte(body))
		if esErr.Status != 502 {
			t.Errorf("status = %d, want 502", esErr.Status)
		}
		if esErr.Raw != "Bad Gateway" {
			t.Errorf("raw = %q, want 'Bad Gateway'", esErr.Raw)
		}
		if esErr.Type != "" {
			t.Errorf("type should be empty for non-JSON, got %q", esErr.Type)
		}
	})

	t.Run("Error() string format with type and reason", func(t *testing.T) {
		esErr := &ESError{Status: 400, Type: "parsing_exception", Reason: "bad query"}
		got := esErr.Error()
		want := "[400] parsing_exception: bad query"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("Error() string format without type", func(t *testing.T) {
		esErr := &ESError{Status: 502, Raw: "Bad Gateway"}
		got := esErr.Error()
		if !strings.HasPrefix(got, "[502]") {
			t.Errorf("Error() = %q, should start with [502]", got)
		}
	})
}

func TestIsPrivilegeError(t *testing.T) {
	t.Run("403 returns true", func(t *testing.T) {
		err := &ESError{Status: 403, Type: "security_exception", Reason: "forbidden"}
		if !IsPrivilegeError(err) {
			t.Error("expected IsPrivilegeError to return true for 403")
		}
	})

	t.Run("401 returns false", func(t *testing.T) {
		err := &ESError{Status: 401, Type: "security_exception", Reason: "unauthorized"}
		if IsPrivilegeError(err) {
			t.Error("expected IsPrivilegeError to return false for 401")
		}
	})

	t.Run("nil error returns false", func(t *testing.T) {
		if IsPrivilegeError(nil) {
			t.Error("expected IsPrivilegeError to return false for nil")
		}
	})

	t.Run("non-ESError returns false", func(t *testing.T) {
		err := errors.New("some other error")
		if IsPrivilegeError(err) {
			t.Error("expected IsPrivilegeError to return false for non-ESError")
		}
	})
}
