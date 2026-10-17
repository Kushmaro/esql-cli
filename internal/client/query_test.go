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

func TestNativeQueryRequest(t *testing.T) {
	query := `ROW value = ?n, text = ?s`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_query" || r.Method != "POST" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "ApiKey key" {
			t.Error("missing authentication")
		}
		var req QueryRequest
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Query != query {
			t.Errorf("query was interpolated: %s", req.Query)
		}
		if req.Params[0].(map[string]interface{})["n"] != json.Number("9223372036854775807") {
			t.Error("parameter precision lost")
		}
		if req.Settings["time_zone"] != "Europe/Amsterdam" || req.Settings["approximation"] != false {
			t.Errorf("settings=%v", req.Settings)
		}
		if !req.Profile || !req.IncludeExecutionMetadata {
			t.Error("missing execution options")
		}
		fmt.Fprint(w, `{"columns":[{"name":"value","type":"long","_meta":{"bucket":"1h"}}],"values":[[9223372036854775807]],"profile":{"driver":1},"cpu_nanos":9007199254740993}`)
	}))
	defer srv.Close()
	resp, err := New(Config{Host: srv.URL, APIKey: "key"}).ExecuteQuery(context.Background(), QueryRequest{
		Query: query, Params: []interface{}{map[string]interface{}{"n": json.Number("9223372036854775807")}, map[string]interface{}{"s": `" | LIMIT 0`}},
		Settings: map[string]interface{}{"time_zone": "Europe/Amsterdam", "approximation": false}, Profile: true, IncludeExecutionMetadata: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Values[0][0] != json.Number("9223372036854775807") {
		t.Fatal("response precision lost")
	}
	if !strings.Contains(string(resp.Raw), `"cpu_nanos":9007199254740993`) || !strings.Contains(string(resp.Columns[0].Meta), "bucket") {
		t.Fatal("metadata lost")
	}
}

func TestAsyncLifecycle(t *testing.T) {
	calls := 0
	id := "opaque+/=?&id"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			if r.Method != "POST" || r.URL.Path != "/_query/async" || r.URL.RawQuery != "" {
				t.Errorf("bad submit route: %s %s", r.Method, r.URL)
			}
			var req AsyncQueryRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
			}
			if req.Query != "ROW n = ?n" || req.KeepOnCompletion == nil || !*req.KeepOnCompletion || req.WaitForCompletionTimeout != "0s" || req.KeepAlive != "1h" || len(req.Params) != 1 || req.Settings["time_zone"] != "UTC" {
				t.Errorf("request=%+v", req)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "is_running": true, "is_partial": true})
		case 2:
			if r.Method != "GET" || r.URL.Path != "/_query/async/"+id {
				t.Errorf("bad get route: %s %s", r.Method, r.URL)
			}
			if r.URL.Query().Get("wait_for_completion_timeout") != "2s" || r.URL.Query().Get("keep_alive") != "1h" {
				t.Errorf("query params=%s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"is_running":false,"columns":[{"name":"n","type":"long"}],"values":[[9007199254740993]]}`)
		case 3:
			if r.Method != "POST" || r.URL.Path != "/_query/async/"+id+"/stop" {
				t.Errorf("bad stop route: %s %s", r.Method, r.URL)
			}
			fmt.Fprint(w, `{"is_running":false,"is_partial":true,"columns":[],"values":[]}`)
		case 4:
			if r.Method != "DELETE" || r.URL.Path != "/_query/async/"+id {
				t.Errorf("bad delete route: %s %s", r.Method, r.URL)
			}
			fmt.Fprint(w, `{"acknowledged":true}`)
		default:
			t.Errorf("extra request: %s %s", r.Method, r.URL)
		}
	}))
	defer srv.Close()
	c := New(Config{Host: srv.URL})
	keep := true
	pending, err := c.SubmitAsync(context.Background(), AsyncQueryRequest{QueryRequest: QueryRequest{Query: "ROW n = ?n", Params: []interface{}{map[string]interface{}{"n": 1}}, Settings: map[string]interface{}{"time_zone": "UTC"}}, WaitForCompletionTimeout: "0s", KeepAlive: "1h", KeepOnCompletion: &keep})
	if err != nil || pending.ID != id || !pending.IsRunning {
		t.Fatalf("pending=%+v error=%v", pending, err)
	}
	resp, err := c.GetAsync(context.Background(), id, AsyncGetOptions{WaitForCompletionTimeout: "2s", KeepAlive: "1h"})
	if err != nil || resp.IsRunning || resp.Values[0][0] != json.Number("9007199254740993") {
		t.Fatalf("result=%+v error=%v", resp, err)
	}
	stopped, err := c.StopAsync(context.Background(), id)
	if err != nil || stopped.IsRunning || !stopped.IsPartial {
		t.Fatalf("stop=%+v error=%v", stopped, err)
	}
	deleted, err := c.DeleteAsync(context.Background(), id)
	if err != nil || !strings.Contains(string(deleted), "true") {
		t.Fatalf("delete=%s error=%v", deleted, err)
	}
	if calls != 4 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestRunningQueryMonitoring(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Error("expected GET")
		}
		if calls == 1 {
			if r.URL.Path != "/_query/queries" {
				t.Error(r.URL.Path)
			}
			fmt.Fprint(w, `{"queries":{"node:1":{"id":9007199254740993,"node":"node","query":"ROW n=1","running_time_nanos":100}}}`)
		} else {
			if r.URL.Path != "/_query/queries/node:1" {
				t.Error(r.URL.Path)
			}
			fmt.Fprint(w, `{"id":9007199254740993,"coordinating_node":"node","data_nodes":["data"]}`)
		}
	}))
	defer srv.Close()
	c := New(Config{Host: srv.URL})
	list, err := c.ListRunningQueries(context.Background())
	if err != nil || !strings.Contains(string(list), "9007199254740993") {
		t.Fatalf("list=%s err=%v", list, err)
	}
	one, err := c.GetRunningQuery(context.Background(), "node:1")
	if err != nil || !strings.Contains(string(one), "coordinating_node") {
		t.Fatalf("one=%s err=%v", one, err)
	}
}

func TestNewAPIErrorsAndCancellation(t *testing.T) {
	for _, status := range []int{400, 403, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":{"type":"test_exception","reason":"test"}}`)
			}))
			defer srv.Close()
			_, err := New(Config{Host: srv.URL}).GetAsync(context.Background(), "missing", AsyncGetOptions{})
			var esErr *ESError
			if !errors.As(err, &esErr) || esErr.Status != status {
				t.Fatalf("error=%v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(Config{Host: "http://localhost:1"}).ExecuteQuery(ctx, QueryRequest{Query: "ROW n=1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	c := New(Config{})
	if _, err := c.GetAsync(context.Background(), "", AsyncGetOptions{}); err == nil {
		t.Fatal("accepted empty ID")
	}
	if _, err := c.GetRunningQuery(context.Background(), ""); err == nil {
		t.Fatal("accepted empty monitoring ID")
	}
}
