package anal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClickHouse talks to a ClickHouse instance over its HTTP interface.
type ClickHouse struct {
	baseURL    string
	httpClient *http.Client
}

// NewClickHouse creates a client pointing at a ClickHouse HTTP interface.
func NewClickHouse(baseURL string) *ClickHouse {
	return &ClickHouse{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Exec executes a DDL or INSERT statement.
func (ch *ClickHouse) Exec(ctx context.Context, query string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ch.baseURL, strings.NewReader(query))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := ch.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("clickhouse request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clickhouse error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// Query executes a SELECT and returns the raw response body.
func (ch *ClickHouse) Query(ctx context.Context, query string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ch.baseURL, strings.NewReader(query))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := ch.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clickhouse request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clickhouse error (%d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// InitSchema creates all analytics tables if they don't exist.
func (ch *ClickHouse) InitSchema(ctx context.Context) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS events_message (
			timestamp DateTime64(3),
			conversation_id String,
			agent_slug LowCardinality(String),
			user_id LowCardinality(String),
			role LowCardinality(String),
			prompt_tokens UInt32,
			completion_tokens UInt32,
			duration_ms UInt32,
			run_id String DEFAULT ''
		) ENGINE = MergeTree()
		ORDER BY (agent_slug, timestamp)`,

		`CREATE TABLE IF NOT EXISTS events_tool_invocation (
			timestamp DateTime64(3),
			conversation_id String,
			agent_slug LowCardinality(String),
			user_id LowCardinality(String),
			tool_name LowCardinality(String),
			success Bool,
			duration_ms UInt32,
			run_id String DEFAULT ''
		) ENGINE = MergeTree()
		ORDER BY (agent_slug, timestamp)`,

		`CREATE TABLE IF NOT EXISTS events_conversation (
			timestamp DateTime64(3),
			conversation_id String,
			agent_slug LowCardinality(String),
			user_id LowCardinality(String),
			event LowCardinality(String),
			run_id String DEFAULT ''
		) ENGINE = MergeTree()
		ORDER BY (agent_slug, timestamp)`,

		`CREATE TABLE IF NOT EXISTS events_memory (
			timestamp DateTime64(3),
			agent_slug LowCardinality(String),
			user_id LowCardinality(String),
			memory_type LowCardinality(String),
			importance Float32,
			run_id String DEFAULT ''
		) ENGINE = MergeTree()
		ORDER BY (agent_slug, timestamp)`,

		`CREATE TABLE IF NOT EXISTS events_relationship (
			timestamp DateTime64(3),
			agent_slug LowCardinality(String),
			user_id LowCardinality(String),
			trust Float32,
			intimacy Float32,
			autonomy Float32,
			reciprocity Float32,
			playfulness Float32,
			conflict Float32,
			run_id String DEFAULT ''
		) ENGINE = MergeTree()
		ORDER BY (agent_slug, timestamp)`,

		`CREATE TABLE IF NOT EXISTS events_consolidation (
			timestamp DateTime64(3),
			agent_slug LowCardinality(String),
			user_id LowCardinality(String),
			patterns_found UInt16,
			memories_merged UInt16,
			run_id String DEFAULT ''
		) ENGINE = MergeTree()
		ORDER BY (agent_slug, timestamp)`,

		`CREATE TABLE IF NOT EXISTS events_eval (
			timestamp DateTime64(3),
			run_id String,
			agent_slug LowCardinality(String),
			user_id LowCardinality(String),
			scenario String,
			response String,
			duration_ms UInt32,
			tools_used String DEFAULT '[]',
			passed UInt16,
			failed UInt16,
			total UInt16,
			criteria String DEFAULT '[]'
		) ENGINE = MergeTree()
		ORDER BY (run_id, timestamp)`,

		`CREATE TABLE IF NOT EXISTS events_run (
			timestamp DateTime64(3),
			run_id String,
			agent_slug LowCardinality(String),
			user_id LowCardinality(String),
			event LowCardinality(String),
			description String DEFAULT ''
		) ENGINE = MergeTree()
		ORDER BY (run_id, timestamp)`,
	}

	for _, ddl := range tables {
		if err := ch.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}

	return nil
}
