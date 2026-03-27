@AGENTS.md

## Conventions
- ClickHouse HTTP client in `clickhouse.go` — uses HTTP interface, not native protocol
- `Inserter` interface for ingestion, `Querier` interface for reads
- Server wires routes via `Handler()` method following axon patterns
- Static assets embedded via `//go:embed`

## Constraints
- ClickHouse is the storage backend — do not abstract it behind a generic storage interface
- Depends on axon only — no other axon-* imports
- Do not add alternative storage backends (Postgres, SQLite, etc.)
- Schema initialization happens via `InitSchema` in the ClickHouse client

## Testing
- `go test ./...` — unit tests do not require a running ClickHouse instance
- `go vet ./...` — lint
- Integration testing requires a ClickHouse instance (via OrbStack/Docker)
