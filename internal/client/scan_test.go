package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScanPagination(t *testing.T) {
	const total = 10003
	pages, closed := 0, false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "ApiKey test-key" {
			t.Error("missing authentication")
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/logs-*/_pit":
			if r.URL.Query().Get("allow_partial_search_results") != "false" {
				t.Error("partial PIT allowed")
			}
			fmt.Fprint(w, `{"id":"pit-0"}`)
		case r.Method == "POST" && r.URL.Path == "/_search":
			if r.URL.Query().Get("allow_partial_search_results") != "false" {
				t.Error("partial search allowed")
			}
			var body struct {
				Size int `json:"size"`
				PIT  struct {
					ID string `json:"id"`
				} `json:"pit"`
				After []json.Number `json:"search_after"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.PIT.ID != fmt.Sprintf("pit-%d", pages) {
				t.Errorf("stale PIT: %s", body.PIT.ID)
			}
			start := 0
			if len(body.After) > 0 {
				n, _ := body.After[0].Int64()
				start = int(n) + 1
			}
			pages++
			hits := []map[string]interface{}{}
			for n := start; n < start+body.Size && n < total; n++ {
				hits = append(hits, map[string]interface{}{"_id": fmt.Sprint(n), "_index": "logs-1", "_source": map[string]interface{}{"value": json.Number("9007199254740993")}, "sort": []int{n}})
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"pit_id": fmt.Sprintf("pit-%d", pages), "hits": map[string]interface{}{"hits": hits}})
		case r.Method == "DELETE" && r.URL.Path == "/_pit":
			var body struct {
				ID string `json:"id"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.ID != fmt.Sprintf("pit-%d", pages) {
				t.Errorf("closing stale PIT: %s", body.ID)
			}
			closed = true
			fmt.Fprint(w, `{"succeeded":true}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer srv.Close()
	count := 0
	err := New(Config{Host: srv.URL, APIKey: "test-key"}).Scan(context.Background(), "logs-*", 2500, func(doc Document) error {
		if doc.ID != fmt.Sprint(count) {
			t.Errorf("document order/duplicates: %s at %d", doc.ID, count)
		}
		if doc.Source["value"] != json.Number("9007199254740993") {
			t.Error("number precision lost")
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != total || pages != 6 || !closed {
		t.Fatalf("count=%d pages=%d closed=%v", count, pages, closed)
	}
}

func TestScanFailureCleanup(t *testing.T) {
	for _, mode := range []string{"timeout", "shards", "missing-source", "missing-sort", "http-error", "visitor", "cancel", "close-error", "open-shards"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			closed := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "DELETE" {
					closed = true
					if mode == "close-error" {
						http.Error(w, `{"error":"failed to close"}`, 500)
					} else {
						fmt.Fprint(w, `{}`)
					}
					return
				}
				if r.URL.Path == "/logs/_pit" {
					if mode == "open-shards" {
						fmt.Fprint(w, `{"id":"pit","_shards":{"failed":1}}`)
					} else {
						fmt.Fprint(w, `{"id":"pit"}`)
					}
					return
				}
				switch mode {
				case "timeout":
					fmt.Fprint(w, `{"pit_id":"new","timed_out":true}`)
				case "shards":
					fmt.Fprint(w, `{"_shards":{"failed":1}}`)
				case "missing-source":
					fmt.Fprint(w, `{"hits":{"hits":[{"_index":"logs","sort":[1]}]}}`)
				case "missing-sort":
					fmt.Fprint(w, `{"hits":{"hits":[{"_source":{}}]}}`)
				case "http-error":
					http.Error(w, `{"error":"search failed"}`, 500)
				case "close-error":
					fmt.Fprint(w, `{"hits":{"hits":[]}}`)
				default:
					fmt.Fprint(w, `{"hits":{"hits":[{"_source":{},"sort":[1]}]}}`)
				}
			}))
			defer srv.Close()
			err := New(Config{Host: srv.URL}).Scan(ctx, "logs", 10, func(Document) error {
				if mode == "cancel" {
					cancel()
					return ctx.Err()
				}
				return errors.New("visitor failed")
			})
			if err == nil || !closed {
				t.Fatalf("err=%v closed=%v", err, closed)
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
		})
	}
}

func TestQueryNumberPrecision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"columns":[{"name":"n","type":"long"}],"values":[[9223372036854775807]],"is_partial":true}`)
	}))
	defer srv.Close()
	resp, err := New(Config{Host: srv.URL}).Query("ROW n = 9223372036854775807")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Values[0][0] != json.Number("9223372036854775807") || !resp.IsPartial {
		t.Fatalf("response=%+v", resp)
	}
}
