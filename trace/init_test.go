package trace

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestInit_InstallsProviderAndReturnsShutdown(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := Init(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown is nil")
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	}()

	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		t.Fatalf("global provider is %T, want *sdktrace.TracerProvider", otel.GetTracerProvider())
	}
	if tp == nil {
		t.Fatal("provider is nil")
	}

	_, span := otel.Tracer("t").Start(context.Background(), "span")
	span.End()
}

func TestInit_OTLPWhenEndpointSet(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")

	shutdown, err := Init(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer shutdown(context.Background())

	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Fatalf("provider type wrong: %T", otel.GetTracerProvider())
	}
}

func TestChooseExporters_StdoutOnlyWhenOTLPUnset(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	exporters, err := chooseExporters(context.Background())
	if err != nil {
		t.Fatalf("chooseExporters: %v", err)
	}
	if len(exporters) != 1 {
		t.Errorf("len(exporters) = %d, want 1 (stdout only)", len(exporters))
	}
}

func TestChooseExporters_StdoutAndOTLPWhenEndpointSet(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")

	exporters, err := chooseExporters(context.Background())
	if err != nil {
		t.Fatalf("chooseExporters: %v", err)
	}
	if len(exporters) != 2 {
		t.Errorf("len(exporters) = %d, want 2 (stdout + otlp)", len(exporters))
	}
}
