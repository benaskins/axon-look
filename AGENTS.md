---
module: github.com/benaskins/axon-look
kind: service
---

# axon-look

Analytics event ingestion and querying. Two storage backends ship as
sibling packages:

- Root `look`: ClickHouse (HTTP interface). Original implementation.
- `look/duckdb`: DuckDB (in-process CGo driver). Added for low-volume
  deployments and file/R2-backed analytics.

## Build & Test

```bash
go test ./...
go vet ./...
```

## Key Files (root ClickHouse package)

- `clickhouse.go`: ClickHouse HTTP client (Exec, Query, InitSchema)
- `events.go`: Event type and Inserter/Querier interfaces
- `ingest.go`: event ingestion handlers
- `query.go`: query endpoints for stats and time-series breakdowns
- `server.go`: Server type with Handler() route wiring
- `embed.go`: embedded static assets

## Key Files (`duckdb/` subpackage)

- `client.go`: DuckDB client. Rewrites ClickHouse `{name:Type}` params to
  positional `?` placeholders and coerces strings to typed values.
- `schema.go`: `InitSchema` with DuckDB-dialect DDL mirroring the
  ClickHouse tables.
- `ingest.go`: ingest handler plus `insertQuery` (copy-adapt of the root).
- `query.go`: all GET handlers with DuckDB-dialect SQL
  (`COUNT(*) FILTER`, `ARG_MIN`, `CAST(... AS DATE)`, etc.).
- `server.go`: `NewServer(*embed.FS, *DuckDB)` wiring routes that match
  the root server so the dashboard works against either backend.
