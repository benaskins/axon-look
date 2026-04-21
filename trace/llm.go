package trace

import (
	"context"
	"sync"
	"time"

	talk "github.com/benaskins/axon-talk"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/benaskins/axon-look/trace"

// WrapLLMClient returns a talk.LLMClient that opens an "llm.call" span
// around each Chat invocation. Model, latency, and (when the provider
// reports them) input/output token counts are recorded as attributes.
// Inner errors mark the span as ERROR before being returned unchanged.
func WrapLLMClient(inner talk.LLMClient) talk.LLMClient {
	return &tracedClient{inner: inner, tracer: otel.Tracer(tracerName)}
}

type tracedClient struct {
	inner  talk.LLMClient
	tracer trace.Tracer
}

func (t *tracedClient) Chat(ctx context.Context, req *talk.Request, fn func(talk.Response) error) error {
	ctx, span := t.tracer.Start(ctx, "llm.call", trace.WithAttributes(
		attribute.String("llm.model", req.Model),
	))
	start := time.Now()

	var (
		mu    sync.Mutex
		usage *talk.Usage
	)

	err := t.inner.Chat(ctx, req, func(r talk.Response) error {
		if r.Usage != nil {
			mu.Lock()
			usage = r.Usage
			mu.Unlock()
		}
		return fn(r)
	})

	span.SetAttributes(attribute.Int64("llm.latency_ms", time.Since(start).Milliseconds()))

	mu.Lock()
	finalUsage := usage
	mu.Unlock()
	if finalUsage != nil {
		span.SetAttributes(
			attribute.Int64("llm.input_tokens", int64(finalUsage.InputTokens)),
			attribute.Int64("llm.output_tokens", int64(finalUsage.OutputTokens)),
		)
	}

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	span.End()
	return err
}
