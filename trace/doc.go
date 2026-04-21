// Package trace provides producer-side OpenTelemetry primitives for
// axon-based services: a TracerProvider bootstrap and an LLMClient
// wrapper that emits structured "llm.call" spans.
//
// Exporter selection is environment-driven. By default spans are written
// to stderr via the stdouttrace exporter so Fly logs capture them with
// zero additional infrastructure. When OTEL_EXPORTER_OTLP_ENDPOINT is
// set, OTLP/HTTP is used instead; a ClickHouse sink can terminate that
// OTLP stream without any producer changes.
package trace
