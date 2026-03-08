//go:build ignore

package main

import (
	"context"
	"log"
	"net/http"

	look "github.com/benaskins/axon-look"
)

func main() {
	ctx := context.Background()

	// Connect to ClickHouse and create tables.
	ch := look.NewClickHouse("http://localhost:8123")
	if err := ch.InitSchema(ctx); err != nil {
		log.Fatal(err)
	}

	// Create the server with ClickHouse for ingest and queries.
	// Pass nil for staticFiles when not embedding a dashboard.
	srv := look.NewServer(nil, ch)

	log.Println("listening on :8090")
	log.Fatal(http.ListenAndServe(":8090", srv.Handler()))
}
