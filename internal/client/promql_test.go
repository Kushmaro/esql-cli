package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestPromQLTransport(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		t.Run(method, func(t *testing.T) {
			params := url.Values{"query": {`sum(up{job="a+b",zone=~"x|y"})`}, "time": {"1792281600.123456789"}, "match[]": {"up{a=\"x,y\"}", "other"}}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method || r.URL.Path != "/_prometheus/metrics-a,metrics-b*/api/v1/query" {
					t.Errorf("route: %s %s", r.Method, r.URL)
				}
				user, password, ok := r.BasicAuth()
				if !ok || user != "elastic" || password != "test" {
					t.Error("missing basic authentication")
				}
				got := r.URL.Query()
				if method == "POST" {
					if r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
						t.Error("form parameters must only be in body")
					}
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					got = r.PostForm
				} else if r.Header.Get("Content-Type") != "" {
					t.Error("GET must not carry form content type")
				}
				if !reflect.DeepEqual(got, params) {
					t.Errorf("parameters=%v want %v", got, params)
				}
				fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"job":"a+b"},"value":[1792281600.123456789,"NaN"]}]},"warnings":["truncated"],"extra":9007199254740993}`)
			}))
			defer srv.Close()
			result, err := New(Config{Host: srv.URL, User: "elastic", Password: "test"}).PromQL(context.Background(), PromQLRequest{Endpoint: "query", Index: "metrics-a,metrics-b*", Method: method, Params: params})
			if err != nil {
				t.Fatal(err)
			}
			for _, value := range []string{"NaN", "1792281600.123456789", "9007199254740993", "warnings"} {
				if !strings.Contains(string(result), value) {
					t.Errorf("lost %s in %s", value, result)
				}
			}
		})
	}
}

func TestPromQLRoutes(t *testing.T) {
	for _, tt := range []struct{ endpoint, label, path string }{{"query_range", "", "query_range"}, {"labels", "", "labels"}, {"series", "", "series"}, {"metadata", "", "metadata"}, {"buildinfo", "", "status/buildinfo"}, {"label_values", "host/name?+", "label/host/name?+/values"}} {
		t.Run(tt.endpoint, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/_prometheus/api/v1/"+tt.path {
					t.Error(r.URL.Path)
				}
				if r.Header.Get("Authorization") != "ApiKey key" {
					t.Error("missing API key")
				}
				fmt.Fprint(w, `{"status":"success","data":[]}`)
			}))
			defer srv.Close()
			if _, err := New(Config{Host: srv.URL, APIKey: "key"}).PromQL(context.Background(), PromQLRequest{Endpoint: tt.endpoint, Label: tt.label}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPromQLErrors(t *testing.T) {
	for _, status := range []int{400, 403, 406, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"status":"error","errorType":"bad_data","error":"invalid expression"}`)
			}))
			defer srv.Close()
			_, err := New(Config{Host: srv.URL}).PromQL(context.Background(), PromQLRequest{Endpoint: "query"})
			var esErr *ESError
			if !errors.As(err, &esErr) || esErr.Status != status || esErr.Type != "bad_data" || esErr.Reason != "invalid expression" {
				t.Fatalf("error=%v", err)
			}
		})
	}
	for _, body := range []string{`{"status":"error","errorType":"execution","error":"failed"}`, `{"data":[]}`, `not json`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer srv.Close()
			if _, err := New(Config{Host: srv.URL}).PromQL(context.Background(), PromQLRequest{Endpoint: "query"}); err == nil {
				t.Fatal("accepted error/invalid envelope")
			}
		})
	}
	c := New(Config{})
	for _, request := range []PromQLRequest{{Endpoint: "other"}, {Endpoint: "label_values"}, {Endpoint: "query", Method: "DELETE"}, {Endpoint: "metadata", Method: "POST"}} {
		if _, err := c.PromQL(context.Background(), request); err == nil {
			t.Fatalf("accepted invalid request %+v", request)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(Config{Host: "http://localhost:1"}).PromQL(ctx, PromQLRequest{Endpoint: "query"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
