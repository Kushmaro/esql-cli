package formatter

import (
	"fmt"
	"io"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/parquet-go/parquet-go"
)

func formatParquet(w io.Writer, resp *client.QueryResponse) error {
	if len(resp.Columns) == 0 {
		return nil
	}

	group := make(parquet.Group, len(resp.Columns))
	for _, col := range resp.Columns {
		group[col.Name] = parquet.Optional(parquet.Leaf(esqlToParquetType(col.Type)))
	}

	schema := parquet.NewSchema("esql", group)
	writer := parquet.NewWriter(w, schema)

	for _, vals := range resp.Values {
		row := buildRowMap(resp.Columns, vals)
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("writing parquet row: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing parquet writer: %w", err)
	}
	return nil
}

func buildRowMap(columns []client.Column, vals []interface{}) map[string]interface{} {
	row := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		val := vals[i]
		if val == nil {
			row[col.Name] = nil
			continue
		}
		row[col.Name] = convertValue(val, col.Type)
	}
	return row
}

func convertValue(val interface{}, esqlType string) interface{} {
	f, isFloat := val.(float64)
	switch esqlType {
	case "boolean":
		return val
	case "integer":
		if isFloat {
			return int32(f)
		}
	case "long", "unsigned_long", "counter_long":
		if isFloat {
			return int64(f)
		}
	case "double", "float", "counter_double":
		if isFloat {
			return f
		}
	}
	return formatValue(val)
}

func esqlToParquetType(esqlType string) parquet.Type {
	switch esqlType {
	case "boolean":
		return parquet.BooleanType
	case "integer":
		return parquet.Int32Type
	case "long", "unsigned_long", "counter_long":
		return parquet.Int64Type
	case "double", "float", "counter_double":
		return parquet.DoubleType
	default:
		return parquet.ByteArrayType
	}
}
