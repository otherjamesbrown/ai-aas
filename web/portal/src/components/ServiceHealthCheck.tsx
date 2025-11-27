import { useState, useEffect, useCallback } from 'react';

type ServiceStatus = 'checking' | 'healthy' | 'degraded' | 'unhealthy' | 'error';

interface ServiceHealth {
  name: string;
  status: ServiceStatus;
  message?: string;
  responseTime?: number;
}

interface ReadinessResponse {
  status: string;
  components?: Record<string, {
    status: string;
    message?: string;
  }>;
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

function TrafficLight({ status }: { status: ServiceStatus }) {
  return (
    <span
      className={`inline-block w-3 h-3 rounded-full ${STATUS_COLORS[status]}`}
      title={STATUS_LABELS[status]}
    />
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
            const displayName = componentName
              .split('_')
              .map(word => word.charAt(0).toUpperCase() + word.slice(1))
              .join(' ');

            let status: ServiceStatus = 'healthy';
            if (componentData.status === 'degraded') {
              status = 'degraded';
            } else if (componentData.status !== 'ready' && componentData.status !== 'healthy') {
              status = 'unhealthy';
            }

            newServices.push({
              name: displayName,
              status,
              message: componentData.message || componentData.status,
              responseTime: readyzTime,
            });
          }
        }
      }
    } catch (error) {
      // Readyz failed, but we already have healthz status
      // Don't add duplicate error entries
    }

    // Check Auth endpoint (CORS test)
    try {
      const authStart = Date.now();
      // Use a simple OPTIONS request to test CORS
      const authResponse = await fetch(`${baseUrl}/v1/auth/userinfo`, {
        method: 'OPTIONS',
        mode: 'cors',
        headers: {
          'Accept': 'application/json',
          'Access-Control-Request-Method': 'GET',
          'Access-Control-Request-Headers': 'authorization,x-correlation-id',
        },
      });
      const authTime = Date.now() - authStart;

      // OPTIONS returning 2xx or 204 means CORS is configured
      if (authResponse.ok || authResponse.status === 204) {
        newServices.push({
          name: 'Auth CORS',
          status: 'healthy',
          message: 'CORS configured',
          responseTime: authTime,
        });
      } else {
        newServices.push({
          name: 'Auth CORS',
          status: 'degraded',
          message: `HTTP ${authResponse.status}`,
          responseTime: authTime,
        });
      }
    } catch {
      newServices.push({
        name: 'Auth CORS',
        status: 'error',
        message: 'CORS not configured',
        responseTime: Date.now() - startTime,
      });
    }

    setServices(newServices);
    setLastChecked(new Date());
  }, [baseUrl]);

  useEffect(() => {
    checkHealth();

    // Refresh every 30 seconds
    const interval = setInterval(checkHealth, 30000);
    return () => clearInterval(interval);
  }, [checkHealth]);

  const overallStatus = services.reduce<ServiceStatus>((worst, service) => {
    const priority: ServiceStatus[] = ['error', 'unhealthy', 'degraded', 'checking', 'healthy'];
    return priority.indexOf(service.status) < priority.indexOf(worst)
      ? service.status
      : worst;
  }, 'healthy');

  const healthyCount = services.filter(s => s.status === 'healthy').length;
  const totalCount = services.length;

  return (
    <div className="bg-white border border-gray-200 rounded-lg shadow-sm">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-gray-50 transition-colors rounded-lg"
      >
        <div className="flex items-center space-x-3">
          <TrafficLight status={overallStatus} />
          <span className="text-sm font-medium text-gray-700">
            Service Health
          </span>
          <span className="text-xs text-gray-500">
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
          <div className="mt-3 space-y-2">
            {services.map((service, index) => (
              <div
                key={index}
                className="flex items-center justify-between py-2 px-3 bg-gray-50 rounded-md"
              >
                <div className="flex items-center space-x-3">
                  <TrafficLight status={service.status} />
                  <span className="text-sm text-gray-700">{service.name}</span>
                </div>
                <div className="flex items-center space-x-3 text-xs">
                  {service.message && (
                    <span className="text-gray-500">{service.message}</span>
                  )}
                  {service.responseTime && (
                    <span className="text-gray-400">{service.responseTime}ms</span>
                  )}
                </div>
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
