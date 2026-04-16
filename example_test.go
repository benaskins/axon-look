package look_test

import (
	"encoding/json"
	"fmt"
	"time"

	look "github.com/benaskins/axon-look"
)

// An Event represents a typed analytics event from a producer service.
// The Type field determines which ClickHouse table the event is routed to.
func ExampleEvent() {
	e := look.Event{
		Type:             "message",
		Timestamp:        time.Date(2026, 4, 16, 10, 30, 0, 0, time.UTC),
		AgentSlug:        "vita",
		UserID:           "user-1",
		ConversationID:   "conv-abc",
		Role:             "assistant",
		PromptTokens:     512,
		CompletionTokens: 128,
		DurationMs:       1200,
	}
	fmt.Println(e.Type)
	fmt.Println(e.AgentSlug)
	fmt.Println(e.PromptTokens + e.CompletionTokens)
	// Output:
	// message
	// vita
	// 640
}

// Events are JSON-serialisable for HTTP ingestion via POST /api/events.
func ExampleEvent_json() {
	success := true
	e := look.Event{
		Type:      "tool_invocation",
		Timestamp: time.Date(2026, 4, 16, 10, 30, 0, 0, time.UTC),
		AgentSlug: "vita",
		ToolName:  "search_memory",
		Success:   &success,
	}

	data, _ := json.Marshal(e)
	var decoded look.Event
	_ = json.Unmarshal(data, &decoded)

	fmt.Println(decoded.Type)
	fmt.Println(decoded.ToolName)
	fmt.Println(*decoded.Success)
	// Output:
	// tool_invocation
	// search_memory
	// true
}

// NewClickHouse creates a client that talks to ClickHouse over its HTTP
// interface. No connection is established until a query is executed.
func ExampleNewClickHouse() {
	ch := look.NewClickHouse("http://localhost:8123")
	fmt.Printf("%T\n", ch)
	// Output:
	// *look.ClickHouse
}

// NewServer creates an analytics server. Pass nil for static files when
// running without the embedded dashboard.
func ExampleNewServer() {
	s := look.NewServer(nil)
	fmt.Printf("%T\n", s)
	// Output:
	// *look.Server
}

// NewServer accepts an optional ClickHouse client. Without one, the
// server serves only the health endpoint and static assets.
func ExampleNewServer_withClickHouse() {
	ch := look.NewClickHouse("http://localhost:8123")
	s := look.NewServer(nil, ch)
	handler := s.Handler()
	fmt.Printf("%T\n", handler)
	// Output:
	// *http.ServeMux
}

// NewCHMaterializer creates a fact.Materializer backed by ClickHouse.
// It translates axon-fact schemas into CREATE TABLE DDL and facts into
// parameterised INSERT statements.
func ExampleNewCHMaterializer() {
	ch := look.NewClickHouse("http://localhost:8123")
	m := look.NewCHMaterializer(ch)
	fmt.Printf("%T\n", m)
	// Output:
	// *look.CHMaterializer
}

// Events cover many analytics domains. Relationship snapshots track
// multi-dimensional scores over time.
func ExampleEvent_relationship() {
	e := look.Event{
		Type:      "relationship_snapshot",
		Timestamp: time.Date(2026, 4, 16, 10, 30, 0, 0, time.UTC),
		AgentSlug: "vita",
		UserID:    "user-1",
		Trust:     0.85,
		Intimacy:  0.72,
		Autonomy:  0.90,
	}
	fmt.Println(e.Type)
	fmt.Printf("trust=%.2f intimacy=%.2f autonomy=%.2f\n", e.Trust, e.Intimacy, e.Autonomy)
	// Output:
	// relationship_snapshot
	// trust=0.85 intimacy=0.72 autonomy=0.90
}

// Eval result events carry structured pass/fail criteria for test scenarios.
func ExampleEvent_evalResult() {
	e := look.Event{
		Type:      "eval_result",
		Timestamp: time.Date(2026, 4, 16, 10, 30, 0, 0, time.UTC),
		AgentSlug: "vita",
		RunID:     "run-001",
		Scenario:  "greeting",
		Passed:    3,
		Failed:    1,
		Total:     4,
	}
	fmt.Println(e.Scenario)
	fmt.Printf("%d/%d passed\n", e.Passed, e.Total)
	// Output:
	// greeting
	// 3/4 passed
}
