package otel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	ot "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const OTEL_EXPORTER_OTLP_TRACES_ENDPOINT_KEY string = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"

const serviceTracerName = "github.com/pitoniak32/trace-export"
const workflowRunTracerName = "github.com/pitoniak32/trace-export/workflow_run"

var (
	otlpEndpoint string
)

// init functions are always executed.
func init() {
	fmt.Println("Getting the collector uri from env!")
	otlpEndpoint = os.Getenv(OTEL_EXPORTER_OTLP_TRACES_ENDPOINT_KEY)

	if otlpEndpoint == "" {
		panic(fmt.Sprintf("ensure %s is set!", OTEL_EXPORTER_OTLP_TRACES_ENDPOINT_KEY))
	}
}

// setupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func SetupOTelSDK(ctx context.Context) (serviceTracer ot.Tracer, workflowRunTracer ot.Tracer, shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// Set up trace provider for workflow run traces.
	wfResource, err := resource.New(
		ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceName("trace-workflow-run")),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to setup workflow run tracer provider resource: %s", err))
	}
	tracerProviderWorkflowRun, err := NewTracerProvider(*wfResource)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProviderWorkflowRun.Shutdown)
	workflowRunTracer = tracerProviderWorkflowRun.Tracer(workflowRunTracerName)

	// We need to create a new tracer provider here to use for our service traces
	// because the global one is used for the traces of workflow runs.
	sResource, err := resource.New(
		ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceName("trace-export-service")),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to setup service tracer provider resource: %s", err))
	}
	tracerProviderService, err := NewTracerProvider(*sResource)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProviderService.Shutdown)
	serviceTracer = tracerProviderService.Tracer(serviceTracerName)

	return
}

func NewTracerProvider(resource resource.Resource) (*trace.TracerProvider, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(otlpEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// stdout exporter example
	// traceExporter, err := stdouttrace.New(
	// 	stdouttrace.WithPrettyPrint())
	// if err != nil {
	// 	return nil, err
	// }

	traceProvider := trace.NewTracerProvider(
		trace.WithResource(&resource),
		trace.WithBatcher(exporter,
			// Default is 5s. Set to 1s for demonstrative purposes.
			trace.WithBatchTimeout(time.Second)),
	)
	return traceProvider, nil
}
