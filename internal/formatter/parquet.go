package formatter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/parquet-go/parquet-go"
)

// orderedParquetGroup preserves ES|QL's column order (Group sorts by name).
type orderedParquetGroup struct {
	parquet.Node
	fields []parquet.Field
}

func (g orderedParquetGroup) Fields() []parquet.Field { return g.fields }

func formatParquet(w io.Writer, resp *client.QueryResponse) (err error) {
	if resp.IsPartial {
		return fmt.Errorf("cannot export partial ES|QL results to Parquet")
	}
	if len(resp.Columns) == 0 {
		return fmt.Errorf("cannot export Parquet without columns")
	}
	lists := make([]bool, len(resp.Columns))
	for i, col := range resp.Columns {
		lists[i] = col.Type == "dense_vector"
	}
	for _, vals := range resp.Values {
		for j, val := range vals {
			if _, ok := val.([]interface{}); ok {
				lists[j] = true
			}
		}
	}
	writer, err := NewParquetWriter(w, resp.Columns, lists)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, writer.Close()) }()
	for _, vals := range resp.Values {
		if err := writer.WriteValues(vals); err != nil {
			return err
		}
	}
	return nil
}

// ParquetWriter streams rows using a fixed schema. lists marks columns that may
// contain arrays; scalar values in those columns become single-element lists.
type ParquetWriter struct {
	writer   *parquet.Writer
	columns  []client.Column
	lists    []bool
	rowIndex int
}

func NewParquetWriter(w io.Writer, columns []client.Column, lists []bool) (*ParquetWriter, error) {
	if len(columns) == 0 || len(columns) != len(lists) {
		return nil, fmt.Errorf("invalid Parquet schema")
	}
	group := make(parquet.Group, len(columns))
	order := make(map[string]int, len(columns))
	for i, col := range columns {
		if _, exists := order[col.Name]; exists {
			return nil, fmt.Errorf("duplicate Parquet column %q", col.Name)
		}
		order[col.Name] = i
		node := esqlToParquetNode(col.Type)
		if lists[i] {
			node = parquet.List(parquet.Optional(node))
		}
		group[col.Name] = parquet.Optional(node)
	}
	fields := group.Fields()
	sort.Slice(fields, func(i, j int) bool { return order[fields[i].Name()] < order[fields[j].Name()] })
	schema := parquet.NewSchema("esql", orderedParquetGroup{Node: group, fields: fields})
	return &ParquetWriter{
		writer:  parquet.NewWriter(w, schema, parquet.MaxRowsPerRowGroup(64*1024)),
		columns: append([]client.Column(nil), columns...),
		lists:   append([]bool(nil), lists...),
	}, nil
}

func (p *ParquetWriter) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("closing Parquet writer: %w", err)
	}
	return nil
}

func (p *ParquetWriter) WriteValues(vals []interface{}) error {
	p.rowIndex++
	rowIndex := p.rowIndex - 1
	if len(vals) != len(p.columns) {
		return fmt.Errorf("Parquet row %d has %d values for %d columns", p.rowIndex, len(vals), len(p.columns))
	}

	row := make(parquet.Row, 0, len(vals))
	for i, val := range vals {
		col := p.columns[i]
		if val == nil {
			row = append(row, parquet.NullValue().Level(0, 0, i))
			continue
		}
		if !p.lists[i] {
			v, err := parquetScalar(val, col.Type)
			if err != nil {
				return fmt.Errorf("Parquet row %d column %q: %w", rowIndex+1, col.Name, err)
			}
			row = append(row, v.Level(0, 1, i))
			continue
		}
		values, ok := val.([]interface{})
		if !ok {
			values = []interface{}{val}
		}
		if len(values) == 0 {
			row = append(row, parquet.NullValue().Level(0, 1, i))
			continue
		}
		for j, item := range values {
			rep := 0
			if j > 0 {
				rep = 1
			}
			if item == nil {
				row = append(row, parquet.NullValue().Level(rep, 2, i))
				continue
			}
			v, err := parquetScalar(item, col.Type)
			if err != nil {
				return fmt.Errorf("Parquet row %d column %q element %d: %w", rowIndex+1, col.Name, j+1, err)
			}
			row = append(row, v.Level(rep, 3, i))
		}
	}
	if _, err := p.writer.WriteRows([]parquet.Row{row}); err != nil {
		return fmt.Errorf("writing Parquet row %d: %w", rowIndex+1, err)
	}
	return nil
}

func esqlToParquetNode(t string) parquet.Node {
	switch t {
	case "boolean":
		return parquet.Leaf(parquet.BooleanType)
	case "byte", "short", "int", "integer", "counter_integer":
		return parquet.Int(32)
	case "long", "counter_long":
		return parquet.Int(64)
	case "unsigned_long":
		return parquet.Uint(64)
	case "double", "float", "half_float", "scaled_float", "counter_double", "dense_vector":
		return parquet.Leaf(parquet.DoubleType)
	case "date", "datetime":
		return parquet.Timestamp(parquet.Millisecond)
	case "date_nanos":
		return parquet.Timestamp(parquet.Nanosecond)
	default:
		return parquet.String()
	}
}

func parquetScalar(val interface{}, t string) (parquet.Value, error) {
	fail := func() (parquet.Value, error) {
		return parquet.Value{}, fmt.Errorf("invalid %s value %v (%T)", t, val, val)
	}
	switch t {
	case "boolean":
		if b, ok := val.(bool); ok {
			return parquet.BooleanValue(b), nil
		}
		return fail()
	case "byte", "short", "int", "integer", "counter_integer", "long", "counter_long", "unsigned_long":
		s, err := parquetNumber(val)
		if err != nil {
			return fail()
		}
		if t == "unsigned_long" {
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return fail()
			}
			return parquet.ValueOf(n), nil
		}
		bits := 64
		if t != "long" && t != "counter_long" {
			bits = 32
		}
		n, err := strconv.ParseInt(s, 10, bits)
		if err != nil {
			return fail()
		}
		if bits == 32 {
			return parquet.Int32Value(int32(n)), nil
		}
		return parquet.Int64Value(n), nil
	case "double", "float", "half_float", "scaled_float", "counter_double", "dense_vector":
		s, err := parquetNumber(val)
		if err != nil {
			return fail()
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
			return fail()
		}
		return parquet.DoubleValue(n), nil
	case "date", "datetime", "date_nanos":
		s, ok := val.(string)
		if !ok {
			return fail()
		}
		stamp, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return fail()
		}
		if t == "date_nanos" {
			n := stamp.UnixNano()
			if !time.Unix(0, n).Equal(stamp) {
				return fail()
			}
			return parquet.Int64Value(n), nil
		}
		return parquet.Int64Value(stamp.UnixMilli()), nil
	default:
		if s, ok := val.(string); ok {
			return parquet.ByteArrayValue([]byte(s)), nil
		}
		// Complex/unknown ES|QL types stay readable and retain their JSON structure.
		data, err := json.Marshal(val)
		if err != nil {
			return parquet.Value{}, fmt.Errorf("encoding %s value: %w", t, err)
		}
		return parquet.ByteArrayValue(data), nil
	}
}

func parquetNumber(v interface{}) (string, error) {
	switch n := v.(type) {
	case json.Number:
		return n.String(), nil
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(n), 'f', -1, 32), nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(n), nil
	default:
		return "", fmt.Errorf("expected number, got %T", v)
	}
}
