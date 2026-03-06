# axon-look

An analytics event ingestion and query service backed by ClickHouse. Part of [lamina](https://github.com/benaskins/lamina) — each axon package can be used independently.

Accepts structured events via HTTP, stores them in typed tables, and exposes query endpoints for dashboards.

## Install

```
go get github.com/benaskins/axon-look@latest
```

Requires Go 1.24+.

## Usage

```go
ch := look.NewClickHouse(clickhouseURL)
ch.InitSchema(ctx)

srv := look.NewServer(ch, ch)
http.Handle("/", srv)
```

### Key types

- `Event` — analytics event with typed fields
- `Inserter` — interface for event ingestion
- `Querier` — interface for event queries
- `ClickHouse` — ClickHouse client implementing both interfaces
- `Server` — HTTP server with ingest and query endpoints

All queries use ClickHouse parameterized queries to prevent SQL injection.

## License

MIT — see [LICENSE](LICENSE).
