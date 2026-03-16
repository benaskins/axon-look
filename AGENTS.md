# axon-look

Analytics event ingestion and querying backed by ClickHouse.

## Build & Test

```bash
go test ./...
go vet ./...
```

## Key Files

- `clickhouse.go` — ClickHouse client and query logic
- `events.go` — analytics event definitions
- `embed.go` — embedded assets
