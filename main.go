package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	eg "github.com/google/go-github/v66/github"
	ig "github.com/pitoniak32/trace-export/pkg/github"
	"github.com/pitoniak32/trace-export/pkg/otel"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

var (
	serviceTracer     trace.Tracer
	workflowRunTracer trace.Tracer
	otelShutdown      func(context.Context) error
)

func init() {
	// Set up OpenTelemetry.
	ctx := context.Background()
	var err error
	serviceTracer, workflowRunTracer, otelShutdown, err = otel.SetupOTelSDK(ctx)
	if err != nil {
		var _ = otelShutdown(ctx)
		panic(fmt.Sprintf("Failed to setup OtelSDK because of: %s", err))
	}
}

func main() {

	if err := run(); err != nil {
		log.Fatalln(err)
	}

	// plan, err := os.ReadFile("./workflow_run_webhook_events.json")
	// if err != nil {
	// 	panic("failed to read test data")
	// }

	// var events [3]eg.WorkflowRunEvent
	// err = json.Unmarshal(plan, &events)
	// if err != nil {
	// 	panic("failed to unmarshal")
	// }

	// for i := 0; i < len(events); i++ {
	// 	event := events[i]

	// 	err := ig.HandlePayload(event)
	// 	if err != nil {
	// 		fmt.Println(err)
	// 	}
	// }

	// err = otelShutdown(context.Background())
	// if err != nil {
	// 	panic("failure occurred during Otel SDK shutdown")
	// }
}

func run() (err error) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Start HTTP server.
	srv := &http.Server{
		Addr:         ":8080",
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      newHTTPHandler(),
	}
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	fmt.Println("Starting server!")

	// Wait for interruption.
	select {
	case err = <-srvErr:
		// Error when starting HTTP server.
		return
	case <-ctx.Done():
		// Wait for first CTRL+C.
		// Stop receiving signal notifications as soon as possible.
		stop()
	}

	// When Shutdown is called, ListenAndServe immediately returns ErrServerClosed.
	err = srv.Shutdown(context.Background())
	return
}

func newHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// handleFunc is a replacement for mux.HandleFunc
	// which enriches the handler's HTTP instrumentation with the pattern as the http.route.
	handleFunc := func(pattern string, handlerFunc func(http.ResponseWriter, *http.Request)) {
		// Configure the "http.route" for the HTTP instrumentation.
		handler := http.HandlerFunc(handlerFunc)
		mux.Handle(pattern, handler)
	}

	// Register handlers.
	handleFunc("/webhook", ghWebhook)

	// Add HTTP instrumentation for the whole server.
	handler := otelhttp.NewHandler(mux, "/")
	return handler
}

func ghWebhook(w http.ResponseWriter, r *http.Request) {
	_, span := serviceTracer.Start(r.Context(), "github-webhook")
	defer span.End()

	workflowRunEvent, err := webhookFromBody(r.Body)
	if err != nil || workflowRunEvent == nil {
		panic(fmt.Errorf("failed to get WorkflowRunEvent from request body: %s", err))
	}

	err = ig.HandlePayload(*workflowRunEvent, workflowRunTracer)
	if err != nil {
		fmt.Println(err)
	}
}

func webhookFromBody(body io.ReadCloser) (*eg.WorkflowRunEvent, error) {
	dec := json.NewDecoder(body)
	if dec == nil {
		return nil, errors.New("failed to create json decoder for request body")
	}
	var event eg.WorkflowRunEvent
	err := dec.Decode(&event)
	if err != nil {
		return nil, err
	}

	return &event, nil
}
