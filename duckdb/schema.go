package duckdb

import (
	"context"
	"fmt"
)

// InitSchema creates all analytics tables if they don't exist. The column
// shape mirrors the ClickHouse version one-for-one; only the types and
// engine clauses are translated to DuckDB's dialect.
func (d *DuckDB) InitSchema(ctx context.Context) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS events_message (
			timestamp TIMESTAMP,
			conversation_id VARCHAR,
			agent_slug VARCHAR,
			user_id VARCHAR,
			role VARCHAR,
			prompt_tokens UINTEGER,
			completion_tokens UINTEGER,
			duration_ms UINTEGER,
			run_id VARCHAR DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS events_tool_invocation (
			timestamp TIMESTAMP,
			conversation_id VARCHAR,
			agent_slug VARCHAR,
			user_id VARCHAR,
			tool_name VARCHAR,
			success BOOLEAN,
			duration_ms UINTEGER,
			run_id VARCHAR DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS events_conversation (
			timestamp TIMESTAMP,
			conversation_id VARCHAR,
			agent_slug VARCHAR,
			user_id VARCHAR,
			event VARCHAR,
			run_id VARCHAR DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS events_memory (
			timestamp TIMESTAMP,
			agent_slug VARCHAR,
			user_id VARCHAR,
			memory_type VARCHAR,
			importance REAL,
			run_id VARCHAR DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS events_relationship (
			timestamp TIMESTAMP,
			agent_slug VARCHAR,
			user_id VARCHAR,
			trust REAL,
			intimacy REAL,
			autonomy REAL,
			reciprocity REAL,
			playfulness REAL,
			conflict REAL,
			run_id VARCHAR DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS events_consolidation (
			timestamp TIMESTAMP,
			agent_slug VARCHAR,
			user_id VARCHAR,
			patterns_found USMALLINT,
			memories_merged USMALLINT,
			run_id VARCHAR DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS events_eval (
			timestamp TIMESTAMP,
			run_id VARCHAR,
			agent_slug VARCHAR,
			user_id VARCHAR,
			scenario VARCHAR,
			response VARCHAR,
			duration_ms UINTEGER,
			tools_used VARCHAR DEFAULT '[]',
			passed USMALLINT,
			failed USMALLINT,
			total USMALLINT,
			criteria VARCHAR DEFAULT '[]'
		)`,

		`CREATE TABLE IF NOT EXISTS events_run (
			timestamp TIMESTAMP,
			run_id VARCHAR,
			agent_slug VARCHAR,
			user_id VARCHAR,
			event VARCHAR,
			description VARCHAR DEFAULT ''
		)`,
	}

	for _, ddl := range tables {
		if _, err := d.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}
