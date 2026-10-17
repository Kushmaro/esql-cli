package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestViewLifecycle(t *testing.T) {
	calls := 0
	query := `ROW message = "a;b", n = 9007199254740993`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "ApiKey test-key" {
			t.Error("missing authentication")
		}
		if calls == 1 {
			if r.Method != "GET" || r.URL.Path != "/_query/view" {
				t.Errorf("list: %s %s", r.Method, r.URL)
			}
			fmt.Fprint(w, `{"views":[]}`)
			return
		}
		switch r.Method {
		case "PUT":
			if r.URL.Path != "/_query/view/view?&name" {
				t.Error(r.URL.Path)
			}
			var request ViewRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Query != query || request.Description == nil || *request.Description != "" {
				t.Errorf("request=%+v", request)
			}
			fmt.Fprint(w, `{"acknowledged":true}`)
		case "GET":
			if r.URL.Path != "/_query/view/a,b" {
				t.Error(r.URL.Path)
			}
			fmt.Fprint(w, `{"views":[{"name":"a","query":"ROW n=1","extra":9007199254740993}]}`)
		case "DELETE":
			if r.URL.Path != "/_query/view/a,b" {
				t.Error(r.URL.Path)
			}
			fmt.Fprint(w, `{"acknowledged":true}`)
		default:
			t.Error(r.Method)
		}
	}))
	defer srv.Close()
	c := New(Config{Host: srv.URL, APIKey: "test-key"})
	if _, err := c.GetViews(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	description := ""
	if _, err := c.PutView(context.Background(), "view?&name", ViewRequest{Query: query, Description: &description}); err != nil {
		t.Fatal(err)
	}
	result, err := c.GetViews(context.Background(), "a,b")
	if err != nil || !strings.Contains(string(result), "9007199254740993") {
		t.Fatalf("result=%s error=%v", result, err)
	}
	if _, err := c.DeleteViews(context.Background(), "a,b"); err != nil {
		t.Fatal(err)
	}
	if calls != 4 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestViewValidationAndErrors(t *testing.T) {
	c := New(Config{})
	if _, err := c.PutView(context.Background(), "", ViewRequest{Query: "ROW n=1"}); err == nil {
		t.Fatal("accepted empty name")
	}
	if _, err := c.PutView(context.Background(), "name", ViewRequest{Query: " "}); err == nil {
		t.Fatal("accepted empty query")
	}
	if _, err := c.DeleteViews(context.Background(), ""); err == nil {
		t.Fatal("accepted empty delete")
	}
	for _, status := range []int{400, 403, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":{"type":"view_exception","reason":"view failed"}}`)
			}))
			defer srv.Close()
			_, err := New(Config{Host: srv.URL}).GetViews(context.Background(), "missing")
			var esErr *ESError
			if !errors.As(err, &esErr) || esErr.Status != status {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
