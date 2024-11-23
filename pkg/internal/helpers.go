package internal

import (
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	ot "go.opentelemetry.io/otel/trace"
)

func NewTestTracer() ot.Tracer {
	traceProvider := trace.NewTracerProvider(
		trace.WithSyncer(tracetest.NewInMemoryExporter()),
	)
	if traceProvider == nil {
		panic("Failed to create TestTracerProvider!")
	}

	return traceProvider.Tracer("TESTING_TRACER")
}
