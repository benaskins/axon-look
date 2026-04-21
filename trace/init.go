package trace

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// Init installs a global TracerProvider for serviceName. Always installs
// the stdouttrace exporter writing to stderr; additionally installs an
// OTLP/HTTP exporter when OTEL_EXPORTER_OTLP_ENDPOINT is set. Callers
// must invoke the returned shutdown function at program exit.
func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	exporters, err := chooseExporters(ctx)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("trace: build resource: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	for _, exp := range exporters {
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func chooseExporters(ctx context.Context) ([]sdktrace.SpanExporter, error) {
	stdoutExp, err := stdouttrace.New(stdouttrace.WithWriter(os.Stderr))
	if err != nil {
		return nil, fmt.Errorf("trace: stdout exporter: %w", err)
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return []sdktrace.SpanExporter{stdoutExp}, nil
	}
	otlpExp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace: otlp exporter: %w", err)
	}
	return []sdktrace.SpanExporter{stdoutExp, otlpExp}, nil
}
