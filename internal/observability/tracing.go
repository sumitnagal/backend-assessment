package observability

import (
    "context"
    "fmt"

    "backend-assessment/internal/config"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// InitTracing configures OpenTelemetry tracing with Jaeger exporter
func InitTracing(ctx context.Context, cfg *config.Config) (func(context.Context) error, error) {
    if !cfg.TracingEnabled {
        return func(context.Context) error { return nil }, nil
    }

    exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.JaegerEndpoint)))
    if err != nil {
        return nil, fmt.Errorf("create jaeger exporter: %w", err)
    }

    res := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceName(cfg.ServiceName),
        semconv.DeploymentEnvironment(cfg.Environment),
    )

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(res),
    )

    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})

    return tp.Shutdown, nil
}


