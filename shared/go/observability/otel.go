package observability

import (
	"context"
	"errors"
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
	"go.opentelemetry.io/otel/trace"
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
	tp       *sdktrace.TracerProvider
	fallback bool
}

// Shutdown flushes telemetry exporters.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.tp == nil {
		return nil
	}
	return p.tp.Shutdown(ctx)
}

// Fallback reports whether the provider is operating in a degraded mode.
func (p *Provider) Fallback() bool {
	if p == nil {
		return false
	}
	return p.fallback
}

// Init configures OpenTelemetry exporters and global providers.
func Init(ctx context.Context, cfg Config) (*Provider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("telemetry endpoint required")
	}

	provider, err := initWithConfig(ctx, cfg)
	if err == nil {
		return provider, nil
	}

	recordExporterFailure(cfg.ServiceName, cfg.Protocol)
	otel.Handle(fmt.Errorf("telemetry init failed for %s exporter: %w", cfg.Protocol, err))

	// Attempt HTTP fallback when gRPC fails.
	if cfg.Protocol == "grpc" {
		httpCfg := cfg
		httpCfg.Protocol = "http"
		if httpProvider, httpErr := initWithConfig(ctx, httpCfg); httpErr == nil {
			return httpProvider, nil
		} else {
			recordExporterFailure(cfg.ServiceName, "http")
			err = errors.Join(err, httpErr)
			otel.Handle(fmt.Errorf("telemetry http fallback failed: %w", httpErr))
		}
	}

	return degradedProvider(cfg.ServiceName), nil
}

// MustInit panics if Init returns an error.
func MustInit(ctx context.Context, cfg Config) *Provider {
	provider, err := Init(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return provider
}

func initWithConfig(ctx context.Context, cfg Config) (*Provider, error) {
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

func degradedProvider(serviceName string) *Provider {
	recordExporterFailure(serviceName, "degraded")
	otel.SetTracerProvider(trace.NewNoopTracerProvider())
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return &Provider{fallback: true}
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
			otlptracegrpc.WithDialOption(grpc.WithBlock()),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
		}
		return otlptracegrpc.NewClient(opts...), nil
	default:
		return nil, fmt.Errorf("unsupported otlp protocol %q", cfg.Protocol)
	}
}
