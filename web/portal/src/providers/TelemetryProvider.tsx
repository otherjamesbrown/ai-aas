import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { Resource } from '@opentelemetry/resources';
import { SemanticResourceAttributes } from '@opentelemetry/semantic-conventions';
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { ZoneContextManager } from '@opentelemetry/context-zone';
import { trace, Span, SpanStatusCode } from '@opentelemetry/api';

interface TelemetryContextValue {
  startSpan: <T>(name: string, fn: (span: Span) => Promise<T>) => Promise<T>;
  addSpanAttribute: (key: string, value: string | number | boolean, span?: Span) => void;
  recordException: (error: Error, span?: Span) => void;
  isInitialized: boolean;
}

const TelemetryContext = createContext<TelemetryContextValue | undefined>(undefined);

interface TelemetryProviderProps {
  children: ReactNode;
  serviceName?: string;
  serviceVersion?: string;
  endpoint?: string;
  enabled?: boolean;
}

/**
 * Telemetry provider - manages OpenTelemetry browser instrumentation
 * Uses OTLP HTTP exporter for traces
 */
export function TelemetryProvider({
  children,
  serviceName = 'web-portal',
  serviceVersion = '0.1.0',
  endpoint = import.meta.env.VITE_OTEL_EXPORTER_OTLP_ENDPOINT,
  enabled = true,
}: TelemetryProviderProps) {
  const [isInitialized, setIsInitialized] = useState(false);
  const [tracerProvider, setTracerProvider] = useState<WebTracerProvider | null>(null);

  useEffect(() => {
    if (!enabled || !endpoint) {
      console.warn('OpenTelemetry disabled: endpoint not configured');
      setIsInitialized(true);
      return;
    }

    try {
      // Create resource
      const resource = new Resource({
        [SemanticResourceAttributes.SERVICE_NAME]: serviceName,
        [SemanticResourceAttributes.SERVICE_VERSION]: serviceVersion,
        [SemanticResourceAttributes.DEPLOYMENT_ENVIRONMENT]:
          import.meta.env.MODE || 'development',
      });

      // Create OTLP HTTP exporter
      const exporter = new OTLPTraceExporter({
        url: `${endpoint}/v1/traces`,
        headers: {
          'Content-Type': 'application/json',
        },
      });

      // Create tracer provider
      const provider = new WebTracerProvider({
        resource,
      });

      // Add batch span processor
      provider.addSpanProcessor(
        new BatchSpanProcessor(exporter, {
          maxQueueSize: 2048,
          maxExportBatchSize: 512,
          scheduledDelayMillis: 5000,
          exportTimeoutMillis: 30000,
        })
      );

      // Register the provider
      provider.register({
        contextManager: new ZoneContextManager(),
      });

      setTracerProvider(provider);
      setIsInitialized(true);

      console.log(`OpenTelemetry initialized: ${serviceName}@${serviceVersion} -> ${endpoint}`);
    } catch (error) {
      console.error('Failed to initialize OpenTelemetry:', error);
      setIsInitialized(true); // Still mark as initialized to prevent blocking
    }

    // Cleanup on unmount
    return () => {
      if (tracerProvider) {
        tracerProvider.shutdown().catch((error) => {
          console.error('Telemetry shutdown error:', error);
        });
      }
    };
  }, [enabled, endpoint, serviceName, serviceVersion]);

  const startSpan = async <T,>(
    name: string,
    fn: (span: Span) => Promise<T>
  ): Promise<T> => {
    if (!isInitialized || !tracerProvider) {
      // Fallback: execute function without tracing
      const mockSpan = {} as Span;
      return fn(mockSpan);
    }

    const tracer = trace.getTracer(serviceName, serviceVersion);
    return tracer.startActiveSpan(name, async (span) => {
      try {
        const result = await fn(span);
        span.setStatus({ code: SpanStatusCode.OK });
        return result;
      } catch (error) {
        span.setStatus({
          code: SpanStatusCode.ERROR,
          message: error instanceof Error ? error.message : String(error),
        });
        if (error instanceof Error) {
          span.recordException(error);
        }
        throw error;
      } finally {
        span.end();
      }
    });
  };

  const addSpanAttribute = (
    key: string,
    value: string | number | boolean,
    span?: Span
  ) => {
    if (!span) {
      const activeSpan = trace.getActiveSpan();
      if (activeSpan) {
        activeSpan.setAttribute(key, value);
      }
      return;
    }

    span.setAttribute(key, value);
  };

  const recordException = (error: Error, span?: Span) => {
    if (!span) {
      const activeSpan = trace.getActiveSpan();
      if (activeSpan) {
        activeSpan.recordException(error);
        activeSpan.setStatus({
          code: SpanStatusCode.ERROR,
          message: error.message,
        });
      }
      return;
    }

    span.recordException(error);
    span.setStatus({
      code: SpanStatusCode.ERROR,
      message: error.message,
    });
  };

  return (
    <TelemetryContext.Provider
      value={{
        startSpan,
        addSpanAttribute,
        recordException,
        isInitialized,
      }}
    >
      {children}
    </TelemetryContext.Provider>
  );
}

export function useTelemetry() {
  const context = useContext(TelemetryContext);
  if (!context) {
    throw new Error('useTelemetry must be used within TelemetryProvider');
  }
  return context;
}
