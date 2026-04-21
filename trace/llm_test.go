package trace

import (
	"context"
	"errors"
	"testing"

	talk "github.com/benaskins/axon-talk"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// stubClient is a talk.LLMClient that invokes fn with pre-canned chunks.
type stubClient struct {
	chunks []talk.Response
	err    error
}

func (s *stubClient) Chat(ctx context.Context, req *talk.Request, fn func(talk.Response) error) error {
	for _, r := range s.chunks {
		if err := fn(r); err != nil {
			return err
		}
	}
	return s.err
}

func withRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return recorder
}

func findAttr(attrs []attribute.KeyValue, key string) (attribute.Value, bool) {
	for _, kv := range attrs {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func TestWrapLLMClient_EmitsSpanWithTokenAttrs(t *testing.T) {
	recorder := withRecorder(t)

	inner := &stubClient{chunks: []talk.Response{
		{Content: "hi"},
		{Done: true, Usage: &talk.Usage{InputTokens: 42, OutputTokens: 7}},
	}}
	wrapped := WrapLLMClient(inner)

	err := wrapped.Chat(context.Background(), &talk.Request{Model: "qwen3-122b"}, func(r talk.Response) error { return nil })
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	span := spans[0]
	if span.Name() != "llm.call" {
		t.Errorf("name = %q, want llm.call", span.Name())
	}

	attrs := span.Attributes()
	if v, ok := findAttr(attrs, "llm.model"); !ok || v.AsString() != "qwen3-122b" {
		t.Errorf("llm.model attr missing/wrong: %v", attrs)
	}
	if v, ok := findAttr(attrs, "llm.input_tokens"); !ok || v.AsInt64() != 42 {
		t.Errorf("llm.input_tokens attr: %v", attrs)
	}
	if v, ok := findAttr(attrs, "llm.output_tokens"); !ok || v.AsInt64() != 7 {
		t.Errorf("llm.output_tokens attr: %v", attrs)
	}
	if _, ok := findAttr(attrs, "llm.latency_ms"); !ok {
		t.Errorf("llm.latency_ms attr missing: %v", attrs)
	}
}

func TestWrapLLMClient_RecordsErrorOnSpan(t *testing.T) {
	recorder := withRecorder(t)

	inner := &stubClient{err: errors.New("provider down")}
	wrapped := WrapLLMClient(inner)

	err := wrapped.Chat(context.Background(), &talk.Request{Model: "m"}, func(r talk.Response) error { return nil })
	if err == nil {
		t.Fatal("expected inner error")
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	if spans[0].Status().Code.String() != "Error" {
		t.Errorf("status = %v, want Error", spans[0].Status())
	}
}

func TestWrapLLMClient_WorksWithoutUsage(t *testing.T) {
	recorder := withRecorder(t)

	inner := &stubClient{chunks: []talk.Response{
		{Content: "hi"},
		{Done: true},
	}}
	wrapped := WrapLLMClient(inner)

	if err := wrapped.Chat(context.Background(), &talk.Request{Model: "m"}, func(r talk.Response) error { return nil }); err != nil {
		t.Fatalf("Chat: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans = %d", len(spans))
	}
	attrs := spans[0].Attributes()
	if _, ok := findAttr(attrs, "llm.input_tokens"); ok {
		t.Errorf("expected no input_tokens attr when Usage absent")
	}
}
