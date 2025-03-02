package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	eg "github.com/google/go-github/v66/github"
	"github.com/pitoniak32/trace-export/pkg/cache"
	ig "github.com/pitoniak32/trace-export/pkg/github"
	"github.com/pitoniak32/trace-export/pkg/otel"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

const OTEL_EXPORTER_OTLP_ENDPOINT_KEY string = "OTEL_EXPORTER_OTLP_ENDPOINT"

var (
	otlpEndpoint      string
	serviceTracer     trace.Tracer
	workflowRunTracer trace.Tracer
	otelShutdown      func(context.Context) error
)

func setup() context.Context {

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Getting the collector uri from env!")
	otlpEndpoint = os.Getenv(OTEL_EXPORTER_OTLP_ENDPOINT_KEY)
	slog.Info("found value for uri", "key", OTEL_EXPORTER_OTLP_ENDPOINT_KEY, "otlp.endpoint", otlpEndpoint)

	// Set up OpenTelemetry.
	ctx := context.Background()
	var err error
	serviceTracer, workflowRunTracer, otelShutdown, err = otel.SetupOTelSDK(ctx, otlpEndpoint)
	if err != nil {
		var _ = otelShutdown(ctx)
		slog.Error("Failed to setup OtelSDK", "err", err)
		os.Exit(1)
	}

	return ctx
}

func main() {
	ctx := setup()
	defer otelShutdown(ctx)

	client := eg.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))

	limits, _, err := client.RateLimit.Get(ctx)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	slog.Info("github ratelimit",
		"core.limit", limits.Core.Limit,
		"core.remaining", limits.Core.Remaining,
		"core.reset", limits.Core.Reset,
	)

	propCache := cache.NewPropCache(12 * time.Hour)

	entry := cache.CacheEntry{
		UpdatedAtMillis: time.Now().UnixMilli(),
		Props:           map[string]string{"foo": "bar"},
	}

	propCache.Insert("trace-export", entry)

	ctxCancelScheduledRefresh, cancelScheduledRefresh := context.WithCancel(ctx)
	defer cancelScheduledRefresh()

	propCache.ScheduleRefresh(ctxCancelScheduledRefresh, 1*time.Hour)

	if err := run(&propCache); err != nil {
		slog.Error(err.Error())
	}
}

func run(propCache *cache.PropCache) (err error) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	addr := ":8080"

	// Start HTTP server.
	srv := &http.Server{
		Addr:         addr,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      newHTTPHandler(propCache),
	}
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	slog.Info("starting server!", "addr", addr)

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

func newHTTPHandler(propCache *cache.PropCache) http.Handler {
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
	handleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		propCache.Insert("from-hello", cache.CacheEntry{
			UpdatedAtMillis: time.Now().UnixMilli(),
			Props:           map[string]string{"foo": "bar"},
		})
		w.WriteHeader(http.StatusAccepted)
		_, err := w.Write([]byte("hello"))
		if err != nil {
			slog.Error("failed hello")
		}
	})

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
