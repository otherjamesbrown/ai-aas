package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
)

// Config controls the OpenTelemetry initialization.
type Config struct {
	ServiceName string
	Environment string
	Endpoint    string
	Protocol    string // grpc or http
	Headers     map[string]string
	Insecure    bool
}

// Provider wraps the tracer provider and exposes Shutdown.
type Provider struct {
	tp *sdktrace.TracerProvider
}

// Shutdown flushes telemetry exporters.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.tp == nil {
		return nil
	}
	return p.tp.Shutdown(ctx)
}

// Init configures OpenTelemetry exporters and global providers.
func Init(ctx context.Context, cfg Config) (*Provider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("telemetry endpoint required")
	}

	client, err := buildClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{tp: tp}, nil
}

// MustInit panics if Init returns an error.
func MustInit(ctx context.Context, cfg Config) *Provider {
	provider, err := Init(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return provider
}

func buildClient(ctx context.Context, cfg Config) (otlptrace.Client, error) {
	switch cfg.Protocol {
	case "http":
		options := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithHeaders(cfg.Headers),
			otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
				Enabled:         true,
				InitialInterval: 100 * time.Millisecond,
				MaxInterval:     5 * time.Second,
				MaxElapsedTime:  0,
			}),
		}
		if cfg.Insecure {
			options = append(options, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.NewClient(options...), nil
	case "grpc", "":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
			otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{
				Enabled:         true,
				InitialInterval: 100 * time.Millisecond,
				MaxInterval:     5 * time.Second,
				MaxElapsedTime:  0,
			}),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		} else {
			opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithBlock()))
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
		}
		return otlptracegrpc.NewClient(opts...), nil
	default:
		return nil, fmt.Errorf("unsupported otlp protocol %q", cfg.Protocol)
	}
}
