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

// Init installs a global TracerProvider for serviceName. Exporter choice
// is environment-driven: OTLP/HTTP when OTEL_EXPORTER_OTLP_ENDPOINT is
// set, otherwise stdouttrace writing to stderr. Callers must invoke the
// returned shutdown function at program exit.
func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	exporter, err := chooseExporter(ctx)
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

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func chooseExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("trace: otlp exporter: %w", err)
		}
		return exp, nil
	}
	exp, err := stdouttrace.New(stdouttrace.WithWriter(os.Stderr))
	if err != nil {
		return nil, fmt.Errorf("trace: stdout exporter: %w", err)
	}
	return exp, nil
}
