package otel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	ot "go.opentelemetry.io/otel/trace"
)

const CLOUD_RUN_EXECUTION_KEY string = "CLOUD_RUN_EXECUTION"
const CLOUD_RUN_TASK_INDEX_KEY string = "CLOUD_RUN_TASK_INDEX"

const SERVICE_TRACER_NAME = "github.com/pitoniak32/trace-export"
const workflowRunTracerName = "github.com/pitoniak32/trace-export/workflow_run"

// setupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func SetupOTelSDK(ctx context.Context, otlpEndpoint string) (serviceTracer ot.Tracer, workflowRunTracer ot.Tracer, shutdown func(context.Context) error, err error) {
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
		resource.WithAttributes(
			semconv.ServiceName("trace-workflow-run"),
			semconv.GCPCloudRunJobExecution(os.Getenv(CLOUD_RUN_EXECUTION_KEY)),
		),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to setup workflow run tracer provider resource: %s", err))
	}
	tracerProviderWorkflowRun, err := NewTracerProvider(otlpEndpoint, *wfResource)
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
	tracerProviderService, err := NewTracerProvider(otlpEndpoint, *sResource)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProviderService.Shutdown)
	otel.SetTracerProvider(tracerProviderService)
	serviceTracer = tracerProviderService.Tracer(SERVICE_TRACER_NAME)

	return
}

func NewTracerProvider(otlpEndpoint string, resource resource.Resource) (*sdktrace.TracerProvider, error) {

	var exporter sdktrace.SpanExporter
	if otlpEndpoint == "" {
		var err error
		exporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}
	} else {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		conn, err := grpc.NewClient(otlpEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
		}
		exporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			return nil, fmt.Errorf("failed to create grpc OTLP trace exporter: %w", err)
		}
	}

	// Google exporter
	// exporter, err := texporter.New()
	// if err != nil {
	// 	panic(fmt.Sprintf("Could not create trace exporter %s", err))
	// }

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(&resource),
		sdktrace.WithSyncer(
			exporter,
		),
	)
	return traceProvider, nil
}
