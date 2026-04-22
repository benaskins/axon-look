// Package duckdb provides a DuckDB-backed implementation of the axon-look
// analytics surface. It mirrors the root package's shape (Exec / Query /
// InitSchema, HTTP ingest + query handlers, NewServer) against a file or
// in-memory DuckDB database instead of ClickHouse.
package duckdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	_ "github.com/marcboeker/go-duckdb/v2"
)

// DuckDB wraps a database/sql connection and exposes the same Exec / Query
// shape as the root look.ClickHouse type, so handler bodies can stay close
// to the ClickHouse originals.
type DuckDB struct {
	db *sql.DB
}

// Open creates a DuckDB client. Pass ":memory:" for an ephemeral in-process
// database, or a filesystem path like "/var/lib/axon-look/look.duckdb".
func Open(dsn string) (*DuckDB, error) {
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}
	return &DuckDB{db: db}, nil
}

// Close releases the underlying database handle.
func (d *DuckDB) Close() error { return d.db.Close() }

// DB returns the underlying *sql.DB. Intended for tests and advanced callers.
func (d *DuckDB) DB() *sql.DB { return d.db }

// Exec runs a DDL or INSERT statement using ClickHouse-style `{name:Type}`
// placeholders so callers can reuse query strings across backends. The type
// annotation is used to coerce the string param into the right Go value.
func (d *DuckDB) Exec(ctx context.Context, query string, params map[string]string) error {
	sqlText, args, err := rewrite(query, params)
	if err != nil {
		return fmt.Errorf("rewrite query: %w", err)
	}
	if _, err := d.db.ExecContext(ctx, sqlText, args...); err != nil {
		return fmt.Errorf("duckdb exec: %w", err)
	}
	return nil
}

// Query runs a SELECT and marshals the result set as a newline-delimited JSON
// stream (one object per row). This matches the response shape the existing
// dashboard expects from the ClickHouse `FORMAT JSONEachRow` responses.
func (d *DuckDB) Query(ctx context.Context, query string, params map[string]string) ([]byte, error) {
	sqlText, args, err := rewrite(query, params)
	if err != nil {
		return nil, fmt.Errorf("rewrite query: %w", err)
	}
	rows, err := d.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("duckdb query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	var out []byte
	for rows.Next() {
		values := make([]any, len(cols))
		scanTargets := make([]any, len(cols))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		row := make(map[string]any, len(cols))
		for i, name := range cols {
			row[name] = normalize(values[i])
		}
		line, err := json.Marshal(row)
		if err != nil {
			return nil, fmt.Errorf("marshal row: %w", err)
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

// normalize converts DuckDB-driver-native Go values into forms that JSON
// marshaling handles cleanly. In particular, []byte values (driver's default
// for VARCHAR on some paths) become strings, and time.Time becomes RFC3339.
func normalize(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(x)
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	default:
		return x
	}
}

// placeholderRe matches ClickHouse-style parameter references like
// `{slug:String}` or `{duration_ms:UInt32}`. Group 1 is the parameter name,
// group 2 is the ClickHouse type annotation.
var placeholderRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*):([A-Za-z0-9()]+)\}`)

// rewrite converts a ClickHouse-style query + string-valued params into a
// DuckDB positional query (`?`) and a typed args slice. The type annotation
// drives how the string is coerced (int, float, bool, or left as string).
func rewrite(query string, params map[string]string) (string, []any, error) {
	var args []any
	var rewriteErr error

	out := placeholderRe.ReplaceAllStringFunc(query, func(match string) string {
		m := placeholderRe.FindStringSubmatch(match)
		name, typ := m[1], m[2]
		raw, ok := params[name]
		if !ok {
			rewriteErr = fmt.Errorf("missing param %q", name)
			return match
		}
		v, err := coerce(raw, typ)
		if err != nil {
			rewriteErr = fmt.Errorf("coerce %q: %w", name, err)
			return match
		}
		args = append(args, v)
		return "?"
	})
	if rewriteErr != nil {
		return "", nil, rewriteErr
	}
	return out, args, nil
}

// coerce turns a string parameter into a Go value based on the ClickHouse
// type annotation. Anything unrecognized is passed through as a string,
// which matches what ClickHouse would have done via URL-encoded params.
func coerce(raw, typ string) (any, error) {
	switch typ {
	case "UInt8", "UInt16", "UInt32", "UInt64", "Int8", "Int16", "Int32", "Int64":
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		return n, nil
	case "Float32", "Float64":
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, err
		}
		return f, nil
	case "Bool":
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, err
		}
		return b, nil
	case "DateTime64(3)", "DateTime":
		// CH wire format: "2026-03-04 14:00:00.000". DuckDB accepts this via
		// TIMESTAMP cast. Parse to time.Time so the driver binds it cleanly.
		layouts := []string{"2006-01-02 15:04:05.000", "2006-01-02 15:04:05"}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, raw); err == nil {
				return t, nil
			}
		}
		return nil, fmt.Errorf("unparseable timestamp %q", raw)
	default:
		// String / LowCardinality(String) / unknown — pass through.
		return raw, nil
	}
}
