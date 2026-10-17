// Package exporter writes complete index contents to Parquet using a stable
// point-in-time scan. A temporary spool allows schema discovery without holding
// the full dataset in memory, including fields that first occur on later pages.
package exporter

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/formatter"
)

type Scanner interface {
	Scan(context.Context, string, int, func(client.Document) error) error
}

type columnType struct {
	typ  string
	list bool
}

// Export writes source fields as top-level columns plus _index and _id. Objects,
// nested arrays, and fields with conflicting types are encoded as JSON strings.
// Source arrays become native Parquet lists. Temporary disk usage is proportional
// to the source dataset; memory usage is bounded by a search page and row group.
func Export(ctx context.Context, c Scanner, index string, pageSize int, w io.Writer) (count int64, err error) {
	spool, err := os.CreateTemp("", "esql-export-*.jsonl")
	if err != nil {
		return 0, fmt.Errorf("creating export spool: %w", err)
	}
	defer os.Remove(spool.Name())
	defer spool.Close()
	buffer := bufio.NewWriter(spool)
	encoder := json.NewEncoder(buffer)
	types := map[string]columnType{"_index": {typ: "keyword"}, "_id": {typ: "keyword"}}
	err = c.Scan(ctx, index, pageSize, func(doc client.Document) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		row := make(map[string]interface{}, len(doc.Source)+2)
		row["_index"], row["_id"] = doc.Index, doc.ID
		for name, value := range doc.Source {
			if name == "_index" || name == "_id" {
				return fmt.Errorf("source field %q conflicts with export metadata", name)
			}
			row[name] = value
			col := types[name]
			values, list := value.([]interface{})
			if list {
				col.list = true
			} else {
				values = []interface{}{value}
			}
			for _, item := range values {
				typ := sourceType(item)
				if typ == "" {
					continue
				}
				if col.typ == "" {
					col.typ = typ
				} else if col.typ != typ {
					col.typ = "json"
				}
			}
			types[name] = col
		}
		if err := encoder.Encode(row); err != nil {
			return fmt.Errorf("spooling export: %w", err)
		}
		count++
		return nil
	})
	if err != nil {
		return count, err
	}
	if err := buffer.Flush(); err != nil {
		return count, fmt.Errorf("flushing export spool: %w", err)
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		return count, err
	}
	names := make([]string, 0, len(types)-2)
	for name := range types {
		if name != "_index" && name != "_id" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	names = append([]string{"_index", "_id"}, names...)
	columns := make([]client.Column, len(names))
	lists := make([]bool, len(names))
	for i, name := range names {
		col := types[name]
		if col.typ == "" {
			col.typ = "keyword"
		}
		columns[i] = client.Column{Name: name, Type: col.typ}
		lists[i] = col.list
	}
	writer, err := formatter.NewParquetWriter(w, columns, lists)
	if err != nil {
		return count, err
	}
	defer func() { err = errors.Join(err, writer.Close()) }()
	decoder := json.NewDecoder(bufio.NewReader(spool))
	decoder.UseNumber()
	for {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		var row map[string]interface{}
		if err := decoder.Decode(&row); err != nil {
			if err == io.EOF {
				break
			}
			return count, fmt.Errorf("reading export spool: %w", err)
		}
		values := make([]interface{}, len(names))
		for i, name := range names {
			value := row[name]
			if columns[i].Type == "json" {
				value, err = jsonValue(value, lists[i])
				if err != nil {
					return count, err
				}
			}
			values[i] = value
		}
		if err := writer.WriteValues(values); err != nil {
			return count, err
		}
	}
	return count, nil
}

func sourceType(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case bool:
		return "boolean"
	case string:
		return "keyword"
	case json.Number:
		if _, err := strconv.ParseInt(v.String(), 10, 64); err == nil {
			return "long"
		}
		if _, err := strconv.ParseUint(v.String(), 10, 64); err == nil {
			return "unsigned_long"
		}
		return "double"
	default:
		return "json"
	}
}

func jsonValue(value interface{}, list bool) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	if list {
		items, ok := value.([]interface{})
		if !ok {
			items = []interface{}{value}
		}
		encoded := make([]interface{}, len(items))
		for i, item := range items {
			v, err := jsonValue(item, false)
			if err != nil {
				return nil, err
			}
			encoded[i] = v
		}
		return encoded, nil
	}
	data, err := json.Marshal(value)
	return string(data), err
}
