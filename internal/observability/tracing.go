package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// TracerName is the package import path used to obtain the global
// tracer for sched. Callers can pass this to otel.Tracer().
const TracerName = "github.com/edaywalid/sched"

// InitTracing installs the global OpenTelemetry tracer provider.
//
// SCHED_OTLP_ENDPOINT controls where spans are exported, e.g.
// "jaeger:4317" inside docker-compose or "localhost:4317" for a local
// collector. When the env var is empty the function installs the no-op
// tracer and returns a shutdown that does nothing, so binaries can call
// InitTracing unconditionally without a hard dependency on a collector.
//
// SCHED_OTEL_SERVICE_NAME overrides the default service name attribute
// set on every span; defaults to component.
func InitTracing(ctx context.Context, component string) (shutdown func(context.Context) error, err error) {
	endpoint := os.Getenv("SCHED_OTLP_ENDPOINT")
	if endpoint == "" {
		slog.Info("OTel disabled: SCHED_OTLP_ENDPOINT not set")
		return func(context.Context) error { return nil }, nil
	}

	serviceName := os.Getenv("SCHED_OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = component
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("v2.0.0-dev"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("resource: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithDialOption(),
		otlptracegrpc.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxQueueSize(2048),
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.Info("OTel tracing enabled",
		slog.String("endpoint", endpoint),
		slog.String("service", serviceName))

	return func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		shutdownErr := tp.Shutdown(shutdownCtx)
		if shutdownErr != nil && !errors.Is(shutdownErr, context.Canceled) {
			return fmt.Errorf("tracer shutdown: %w", shutdownErr)
		}
		return nil
	}, nil
}
