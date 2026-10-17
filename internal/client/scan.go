package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Document contains the original source and identifying metadata from a search.
type Document struct {
	Index  string                 `json:"_index"`
	ID     string                 `json:"_id"`
	Source map[string]interface{} `json:"_source"`
	Sort   []json.RawMessage      `json:"sort,omitempty"`
}

// Scan visits every document visible at the time the PIT is opened. It keeps
// only one page in memory and always closes the latest PIT, including on errors.
func (c *Client) Scan(ctx context.Context, index string, pageSize int, visit func(Document) error) (err error) {
	if index == "" {
		return fmt.Errorf("index is required")
	}
	if pageSize < 1 || pageSize > 10000 {
		return fmt.Errorf("page size must be between 1 and 10000")
	}
	var pit struct {
		ID     string `json:"id"`
		Shards struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/"+url.PathEscape(index)+"/_pit?keep_alive=5m&allow_partial_search_results=false", nil, &pit); err != nil {
		return err
	}
	if pit.ID == "" {
		return fmt.Errorf("Elasticsearch returned no point-in-time ID")
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if closeErr := c.doJSON(cleanupCtx, http.MethodDelete, "/_pit", map[string]interface{}{"id": pit.ID}, nil); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing point in time: %w", closeErr))
		}
	}()
	if pit.Shards.Failed > 0 {
		return fmt.Errorf("cannot export: point in time has failed shards")
	}
	var after []json.RawMessage
	for {
		body := map[string]interface{}{
			"size": pageSize, "pit": map[string]string{"id": pit.ID, "keep_alive": "5m"},
			"sort": []string{"_shard_doc"}, "track_total_hits": false, "_source": true,
		}
		if len(after) > 0 {
			body["search_after"] = after
		}
		var page struct {
			PITID    string `json:"pit_id"`
			TimedOut bool   `json:"timed_out"`
			Shards   struct {
				Failed int `json:"failed"`
			} `json:"_shards"`
			Clusters struct {
				Failed  int `json:"failed"`
				Partial int `json:"partial"`
				Skipped int `json:"skipped"`
			} `json:"_clusters"`
			Hits struct {
				Hits []Document `json:"hits"`
			} `json:"hits"`
		}
		if err := c.doJSON(ctx, http.MethodPost, "/_search?allow_partial_search_results=false", body, &page); err != nil {
			return err
		}
		if page.PITID != "" {
			pit.ID = page.PITID
		}
		if page.TimedOut || page.Shards.Failed > 0 || page.Clusters.Failed > 0 || page.Clusters.Partial > 0 || page.Clusters.Skipped > 0 {
			return fmt.Errorf("cannot export incomplete search results (timeout or shard/cluster failure)")
		}
		if len(page.Hits.Hits) == 0 {
			return nil
		}
		next := page.Hits.Hits[len(page.Hits.Hits)-1].Sort
		oldCursor, _ := json.Marshal(after)
		newCursor, _ := json.Marshal(next)
		if len(next) == 0 || bytes.Equal(oldCursor, newCursor) {
			return fmt.Errorf("Elasticsearch returned a missing or repeated pagination cursor")
		}
		for _, doc := range page.Hits.Hits {
			if err := ctx.Err(); err != nil {
				return err
			}
			if doc.Source == nil {
				return fmt.Errorf("cannot export index %q: document source is unavailable; _source must be enabled", doc.Index)
			}
			if err := visit(doc); err != nil {
				return err
			}
		}
		after = next
	}
}
