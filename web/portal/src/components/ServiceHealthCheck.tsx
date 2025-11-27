import { useState, useEffect, useCallback, useRef } from 'react';

type ServiceStatus = 'checking' | 'healthy' | 'degraded' | 'unhealthy' | 'error';

interface ServiceHealth {
  name: string;
  status: ServiceStatus;
  message?: string;
  details?: string;
  responseTime?: number;
}

interface ReadinessResponse {
  status: string;
  // Components can be either a string status or an object with status/message
  components?: Record<string, string | { status: string; message?: string }>;
}

interface HealthzResponse {
  status: string;
}

const STATUS_COLORS: Record<ServiceStatus, string> = {
  checking: 'bg-gray-400 animate-pulse',
  healthy: 'bg-green-500',
  degraded: 'bg-yellow-500',
  unhealthy: 'bg-red-500',
  error: 'bg-red-500',
};

const STATUS_LABELS: Record<ServiceStatus, string> = {
  checking: 'Checking...',
  healthy: 'Healthy',
  degraded: 'Degraded',
  unhealthy: 'Unhealthy',
  error: 'Error',
};

// Headers that the frontend sends and expects to be allowed by CORS
const REQUIRED_CORS_HEADERS = [
  'authorization',
  'content-type',
  'x-correlation-id',
  'x-csrf-token',
];

function TrafficLight({ status }: { status: ServiceStatus }) {
  return (
    <span
      className={`inline-block w-3 h-3 rounded-full ${STATUS_COLORS[status]}`}
      title={STATUS_LABELS[status]}
    />
  );
}

function parseCorsAllowedHeaders(headerValue: string | null): string[] {
  if (!headerValue) return [];
  return headerValue.split(',').map(h => h.trim().toLowerCase());
}

function checkMissingHeaders(allowedHeaders: string[]): string[] {
  return REQUIRED_CORS_HEADERS.filter(
    required => !allowedHeaders.some(allowed => allowed === required)
  );
}

export function ServiceHealthCheck() {
  const [services, setServices] = useState<ServiceHealth[]>([
    { name: 'API Gateway', status: 'checking' },
  ]);
  const [expanded, setExpanded] = useState(false);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);

  const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';
  // Extract base URL without /api suffix for status endpoints
  const baseUrl = apiBaseUrl.replace(/\/api\/?$/, '');

  const checkHealth = useCallback(async () => {
    const startTime = Date.now();
    const newServices: ServiceHealth[] = [];

    // Check API Gateway health (healthz endpoint)
    try {
      const healthzStart = Date.now();
      const healthzResponse = await fetch(`${baseUrl}/v1/status/healthz`, {
        method: 'GET',
        mode: 'cors',
        headers: {
          'Accept': 'application/json',
        },
      });
      const healthzTime = Date.now() - healthzStart;

      if (healthzResponse.ok) {
        const data: HealthzResponse = await healthzResponse.json();
        newServices.push({
          name: 'API Gateway',
          status: data.status === 'healthy' ? 'healthy' : 'degraded',
          message: `Status: ${data.status}`,
          responseTime: healthzTime,
        });
      } else {
        newServices.push({
          name: 'API Gateway',
          status: 'unhealthy',
          message: `HTTP ${healthzResponse.status}`,
          responseTime: healthzTime,
        });
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Connection failed';
      const isCorsError = errorMessage.includes('CORS') ||
                          errorMessage.includes('NetworkError') ||
                          errorMessage.includes('Failed to fetch');
      newServices.push({
        name: 'API Gateway',
        status: 'error',
        message: isCorsError ? 'CORS/Network error' : errorMessage,
        details: isCorsError
          ? 'Cannot reach API. Check network or CORS configuration.'
          : undefined,
        responseTime: Date.now() - startTime,
      });
    }

    // Check API Gateway readiness (readyz endpoint) for component details
    try {
      const readyzStart = Date.now();
      const readyzResponse = await fetch(`${baseUrl}/v1/status/readyz`, {
        method: 'GET',
        mode: 'cors',
        headers: {
          'Accept': 'application/json',
        },
      });
      const readyzTime = Date.now() - readyzStart;

      if (readyzResponse.ok || readyzResponse.status === 503) {
        const data: ReadinessResponse = await readyzResponse.json();

        // Add individual component statuses
        if (data.components) {
          for (const [componentName, componentData] of Object.entries(data.components)) {
            // Handle both string and object formats
            const statusValue = typeof componentData === 'string'
              ? componentData
              : componentData.status;
            const messageValue = typeof componentData === 'string'
              ? componentData
              : (componentData.message || componentData.status);

            // Skip not_configured services - they're intentionally disabled in dev
            if (statusValue === 'not_configured') {
              continue;
            }

            const displayName = componentName
              .split('_')
              .map(word => word.charAt(0).toUpperCase() + word.slice(1))
              .join(' ');

            let status: ServiceStatus = 'healthy';
            if (statusValue === 'degraded') {
              status = 'degraded';
            } else if (statusValue !== 'ready' && statusValue !== 'healthy') {
              status = 'unhealthy';
            }

            newServices.push({
              name: displayName,
              status,
              message: messageValue,
              responseTime: readyzTime,
            });
          }
        }
      }
    } catch {
      // Readyz failed, but we already have healthz status
      // Don't add duplicate error entries
    }

    // CORS Preflight Test - Check that all required headers are allowed
    try {
      const corsStart = Date.now();
      const corsResponse = await fetch(`${baseUrl}/v1/status/healthz`, {
        method: 'OPTIONS',
        mode: 'cors',
        headers: {
          'Origin': window.location.origin,
          'Access-Control-Request-Method': 'GET',
          'Access-Control-Request-Headers': REQUIRED_CORS_HEADERS.join(','),
        },
      });
      const corsTime = Date.now() - corsStart;

      // Check the Access-Control-Allow-Headers response
      const allowedHeadersStr = corsResponse.headers.get('Access-Control-Allow-Headers');
      const allowedOrigin = corsResponse.headers.get('Access-Control-Allow-Origin');
      const allowedHeaders = parseCorsAllowedHeaders(allowedHeadersStr);
      const missingHeaders = checkMissingHeaders(allowedHeaders);

      if (corsResponse.status === 204 || corsResponse.ok) {
        if (missingHeaders.length === 0) {
          newServices.push({
            name: 'CORS Config',
            status: 'healthy',
            message: 'All headers allowed',
            details: allowedOrigin ? `Origin: ${allowedOrigin}` : undefined,
            responseTime: corsTime,
          });
        } else {
          newServices.push({
            name: 'CORS Config',
            status: 'degraded',
            message: `Missing: ${missingHeaders.join(', ')}`,
            details: 'Some headers not in Access-Control-Allow-Headers',
            responseTime: corsTime,
          });
        }
      } else {
        newServices.push({
          name: 'CORS Config',
          status: 'unhealthy',
          message: `HTTP ${corsResponse.status}`,
          details: 'CORS preflight request failed',
          responseTime: corsTime,
        });
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      newServices.push({
        name: 'CORS Config',
        status: 'error',
        message: 'Preflight failed',
        details: errorMessage.includes('Failed to fetch')
          ? 'Browser blocked request. Origin not allowed or network issue.'
          : errorMessage,
        responseTime: Date.now() - startTime,
      });
    }

    // Test actual request with X-Correlation-ID header (the one that was breaking)
    try {
      const headerTestStart = Date.now();
      const headerTestResponse = await fetch(`${baseUrl}/v1/status/healthz`, {
        method: 'GET',
        mode: 'cors',
        headers: {
          'Accept': 'application/json',
          'X-Correlation-ID': crypto.randomUUID(),
        },
      });
      const headerTestTime = Date.now() - headerTestStart;

      if (headerTestResponse.ok) {
        newServices.push({
          name: 'Header Test',
          status: 'healthy',
          message: 'X-Correlation-ID allowed',
          responseTime: headerTestTime,
        });
      } else {
        newServices.push({
          name: 'Header Test',
          status: 'unhealthy',
          message: `HTTP ${headerTestResponse.status}`,
          responseTime: headerTestTime,
        });
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      const isCorsError = errorMessage.includes('CORS') ||
                          errorMessage.includes('NetworkError') ||
                          errorMessage.includes('Failed to fetch');
      newServices.push({
        name: 'Header Test',
        status: 'error',
        message: isCorsError ? 'X-Correlation-ID blocked' : 'Request failed',
        details: isCorsError
          ? 'CORS blocking X-Correlation-ID header. Check server config.'
          : errorMessage,
        responseTime: Date.now() - startTime,
      });
    }

    setServices(newServices);
    setLastChecked(new Date());
  }, [baseUrl]);

  // Use ref to avoid re-render loop while keeping checkHealth up-to-date
  const checkHealthRef = useRef(checkHealth);
  checkHealthRef.current = checkHealth;

  useEffect(() => {
    // Initial check
    checkHealthRef.current();

    // Refresh every 30 seconds
    const interval = setInterval(() => checkHealthRef.current(), 30000);
    return () => clearInterval(interval);
  }, []); // Empty deps - only run once on mount

  const overallStatus = services.reduce<ServiceStatus>((worst, service) => {
    const priority: ServiceStatus[] = ['error', 'unhealthy', 'degraded', 'checking', 'healthy'];
    return priority.indexOf(service.status) < priority.indexOf(worst)
      ? service.status
      : worst;
  }, 'healthy');

  const healthyCount = services.filter(s => s.status === 'healthy').length;
  const totalCount = services.length;
  const hasErrors = services.some(s => s.status === 'error' || s.status === 'unhealthy');

  return (
    <div className={`border rounded-lg shadow-sm ${hasErrors ? 'bg-red-50 border-red-200' : 'bg-white border-gray-200'}`}>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-opacity-80 transition-colors rounded-lg"
      >
        <div className="flex items-center space-x-3">
          <TrafficLight status={overallStatus} />
          <span className="text-sm font-medium text-gray-700">
            Service Health
          </span>
          <span className={`text-xs ${hasErrors ? 'text-red-600 font-medium' : 'text-gray-500'}`}>
            ({healthyCount}/{totalCount} healthy)
          </span>
        </div>
        <div className="flex items-center space-x-2">
          {lastChecked && (
            <span className="text-xs text-gray-400">
              {lastChecked.toLocaleTimeString()}
            </span>
          )}
          <svg
            className={`w-4 h-4 text-gray-400 transition-transform ${expanded ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </button>

      {expanded && (
        <div className="px-4 pb-4 border-t border-gray-100">
          {hasErrors && (
            <div className="mt-3 p-3 bg-red-100 border border-red-200 rounded-md">
              <p className="text-xs text-red-800 font-medium">
                Some services are unavailable. Login may not work correctly.
              </p>
            </div>
          )}

          <div className="mt-3 space-y-2">
            {services.map((service, index) => (
              <div
                key={index}
                className={`py-2 px-3 rounded-md ${
                  service.status === 'error' || service.status === 'unhealthy'
                    ? 'bg-red-50'
                    : service.status === 'degraded'
                    ? 'bg-yellow-50'
                    : 'bg-gray-50'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-3">
                    <TrafficLight status={service.status} />
                    <span className="text-sm text-gray-700">{service.name}</span>
                  </div>
                  <div className="flex items-center space-x-3 text-xs">
                    {service.message && (
                      <span className={`${
                        service.status === 'error' || service.status === 'unhealthy'
                          ? 'text-red-600'
                          : 'text-gray-500'
                      }`}>
                        {service.message}
                      </span>
                    )}
                    {service.responseTime && (
                      <span className="text-gray-400">{service.responseTime}ms</span>
                    )}
                  </div>
                </div>
                {service.details && (
                  <p className="mt-1 ml-6 text-xs text-gray-500">{service.details}</p>
                )}
              </div>
            ))}
          </div>

          <button
            onClick={checkHealth}
            className="mt-3 w-full py-2 text-xs text-primary hover:text-primary-dark font-medium transition-colors"
          >
            Refresh Status
          </button>
        </div>
      )}
    </div>
  );
}
