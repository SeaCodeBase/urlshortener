// backend/pkg/telemetry/otel.go
package telemetry

import (
	"context"
	"io"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init initializes OpenTelemetry with a stdout exporter for local development.
// Returns a shutdown function that should be deferred.
func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	// Create exporter that discards output - we only need trace/span IDs in logs
	exporter, err := stdouttrace.New(
		stdouttrace.WithWriter(io.Discard),
	)
	if err != nil {
		return nil, err
	}

	// Create resource with service name
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create trace provider
	// Use WithSyncer for immediate export (good for development)
	// Use WithBatcher for production (batches spans before export)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(res),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
