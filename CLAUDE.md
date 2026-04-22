@AGENTS.md

## Conventions
- Two storage backends live side by side as sibling packages:
  - Root `look` package: ClickHouse HTTP client in `clickhouse.go` (original, retained for back-compat).
  - `look/duckdb` package: DuckDB client over `database/sql` (added for low-volume, file/R2 deployments).
- Each package owns its own `Exec`/`Query`/`InitSchema`, ingest handlers, query handlers, and `NewServer`.
- `Inserter` interface for ingestion, `Querier` interface for reads — duplicated per subpackage until a shared abstraction earns its keep.
- Server wires routes via `Handler()` method following axon patterns
- Static assets embedded via `//go:embed`

## Constraints
- Do NOT create a generic `Store` interface speculatively. If duplication between the ClickHouse and DuckDB subpackages proves painful in practice, extract then — not before.
- Depends on axon only — no other axon-* imports
- Schema initialization happens via `InitSchema` in each backend's client
- DuckDB subpackage uses CGo (`github.com/marcboeker/go-duckdb/v2`). Root package stays pure Go.

## Testing
- `go test ./...` — unit tests do not require a running ClickHouse instance; DuckDB tests use in-process `:memory:` so they always run.
- `go vet ./...` — lint
- Integration testing against ClickHouse requires an instance (via OrbStack/Docker)
