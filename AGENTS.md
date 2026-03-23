# axon-look

Analytics event ingestion and querying backed by ClickHouse.

## Build & Test

```bash
go test ./...
go vet ./...
```

## Key Files

- `clickhouse.go` — ClickHouse HTTP client (Exec, Query, InitSchema)
- `events.go` — Event type and Inserter/Querier interfaces
- `ingest.go` — event ingestion handlers
- `query.go` — query endpoints for stats and time-series breakdowns
- `server.go` — Server type with Handler() route wiring
- `embed.go` — embedded static assets
