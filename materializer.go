package look

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/benaskins/axon-fact"
)

// CHMaterializer implements fact.Materializer for ClickHouse.
// It generates CREATE TABLE DDL from schema metadata and
// parameterised INSERT statements from fact data.
type CHMaterializer struct {
	db      Inserter
	schemas map[string]fact.Schema
}

// NewCHMaterializer creates a Materializer backed by a ClickHouse Inserter.
func NewCHMaterializer(db Inserter) *CHMaterializer {
	return &CHMaterializer{
		db:      db,
		schemas: make(map[string]fact.Schema),
	}
}

// EnsureSchema creates ClickHouse tables from schema metadata.
func (m *CHMaterializer) EnsureSchema(ctx context.Context, schemas ...fact.Schema) error {
	for _, s := range schemas {
		ddl := schemaToCreateTable(s)
		if err := m.db.Exec(ctx, ddl, nil); err != nil {
			return fmt.Errorf("ensure schema %q: %w", s.Name, err)
		}
		m.schemas[s.Name] = s
	}
	return nil
}

// Materialize inserts facts into ClickHouse using parameterised queries
// derived from schema metadata.
func (m *CHMaterializer) Materialize(ctx context.Context, facts ...fact.Fact) error {
	for i, f := range facts {
		s, ok := m.schemas[f.Schema]
		if !ok {
			return fmt.Errorf("fact %d: unknown schema %q (call EnsureSchema first)", i, f.Schema)
		}
		query, params := factToInsert(s, f)
		if err := m.db.Exec(ctx, query, params); err != nil {
			return fmt.Errorf("materialize %q row %d: %w", f.Schema, i, err)
		}
	}
	return nil
}

// chTypeName maps a fact.FieldType to its ClickHouse column type.
func chTypeName(ft fact.FieldType) string {
	switch ft {
	case fact.String:
		return "String"
	case fact.LowCardinalityString:
		return "LowCardinality(String)"
	case fact.Bool:
		return "Bool"
	case fact.UInt16:
		return "UInt16"
	case fact.UInt32:
		return "UInt32"
	case fact.Float32:
		return "Float32"
	case fact.Float64:
		return "Float64"
	case fact.DateTime64:
		return "DateTime64(3)"
	case fact.JSON:
		return "String"
	default:
		return "String"
	}
}

// chParamType maps a fact.FieldType to the ClickHouse param placeholder type.
func chParamType(ft fact.FieldType) string {
	switch ft {
	case fact.LowCardinalityString:
		return "String" // params don't use LowCardinality wrapper
	case fact.DateTime64:
		return "DateTime64(3)"
	case fact.JSON:
		return "String"
	default:
		return chTypeName(ft)
	}
}

// schemaToCreateTable generates a CREATE TABLE IF NOT EXISTS DDL
// from schema metadata.
func schemaToCreateTable(s fact.Schema) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS %s (\n", s.Name)
	for i, f := range s.Fields {
		if i > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(&b, "\t%s %s", f.Name, chTypeName(f.Type))
	}
	b.WriteString("\n) ENGINE = MergeTree()\n")
	if len(s.OrderBy) > 0 {
		fmt.Fprintf(&b, "ORDER BY (%s)", strings.Join(s.OrderBy, ", "))
	} else {
		b.WriteString("ORDER BY tuple()")
	}
	return b.String()
}

// factToInsert builds a parameterised INSERT statement and param map.
func factToInsert(s fact.Schema, f fact.Fact) (string, map[string]string) {
	cols := make([]string, 0, len(s.Fields))
	placeholders := make([]string, 0, len(s.Fields))
	params := make(map[string]string, len(s.Fields))

	for _, field := range s.Fields {
		cols = append(cols, field.Name)
		placeholders = append(placeholders, fmt.Sprintf("{%s:%s}", field.Name, chParamType(field.Type)))
		params[field.Name] = formatValue(field.Type, f.Data[field.Name])
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		s.Name,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)

	return query, params
}

// formatValue converts a Go value to a string suitable for ClickHouse params.
func formatValue(ft fact.FieldType, v any) string {
	if v == nil {
		return zeroValue(ft)
	}

	switch ft {
	case fact.DateTime64:
		switch tv := v.(type) {
		case time.Time:
			return tv.Format("2006-01-02 15:04:05.000")
		case string:
			return tv
		default:
			return "1970-01-01 00:00:00.000"
		}

	case fact.Bool:
		switch bv := v.(type) {
		case bool:
			if bv {
				return "true"
			}
			return "false"
		default:
			return "false"
		}

	case fact.UInt16, fact.UInt32:
		return fmt.Sprintf("%v", v)

	case fact.Float32, fact.Float64:
		return fmt.Sprintf("%v", v)

	case fact.String, fact.LowCardinalityString, fact.JSON:
		return fmt.Sprintf("%v", v)

	default:
		return fmt.Sprintf("%v", v)
	}
}

// zeroValue returns the zero/default string for a ClickHouse param type.
func zeroValue(ft fact.FieldType) string {
	switch ft {
	case fact.Bool:
		return "false"
	case fact.UInt16, fact.UInt32:
		return "0"
	case fact.Float32, fact.Float64:
		return "0"
	case fact.DateTime64:
		return "1970-01-01 00:00:00.000"
	case fact.JSON:
		return "[]"
	default:
		return ""
	}
}
