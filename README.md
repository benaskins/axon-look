# axon-look

> Domain package · Part of the [lamina](https://github.com/benaskins/lamina-mono) workspace

Analytics event ingestion and querying backed by ClickHouse. Accepts structured events via HTTP (messages, tool invocations, conversations, memory extractions, eval results, runs), stores them in typed tables, and exposes query endpoints for per-agent stats, time-series breakdowns, and eval summaries. All queries use ClickHouse parameterized queries to prevent SQL injection.

## Getting started

```
go get github.com/benaskins/axon-look@latest
```

axon-look is a domain package — it provides HTTP handlers and a ClickHouse client, but no `main` function. You assemble it in your own composition root (see `example/main.go`).

```go
ch := look.NewClickHouse("http://localhost:8123")
ch.InitSchema(ctx)

srv := look.NewServer(nil, ch)
http.Handle("/", srv.Handler())
```

## Key types

- **`Event`** — structured analytics event with typed fields (message, tool invocation, eval result, etc.)
- **`ClickHouse`** — HTTP client for ClickHouse, implements both `Inserter` and `Querier`
- **`Inserter`** — interface for executing insert statements
- **`Querier`** — interface for executing select queries
- **`Server`** — HTTP server wiring ingest (`POST /api/events`) and query endpoints

## License

MIT — see [LICENSE](LICENSE).
