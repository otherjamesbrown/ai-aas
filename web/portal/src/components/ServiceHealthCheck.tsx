import { useState, useEffect, useRef } from 'react';
import { publicClient } from '@/lib/http/client';

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
  components?: Record<string, string | { status: string; message?: string }>;
}

interface HealthzResponse {
  status: string;
}

interface PlatformHealthResponse {
  status: string;
  services?: Record<string, { status: string; message?: string; response_ms?: number }>;
  functional_tests?: Record<string, { status: string; message?: string }>;
}

const STATUS_COLORS: Record<ServiceStatus, string> = {
  checking: 'bg-gray-400 dark:bg-gray-500 animate-pulse',
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

/**
 * Performs health checks against all service endpoints.
 * Returns array of service health statuses.
 */
async function performHealthCheck(): Promise<ServiceHealth[]> {
  const startTime = Date.now();
  const newServices: ServiceHealth[] = [];

  // Check API Gateway health (healthz endpoint)
  try {
    const healthzStart = Date.now();
    const healthzResponse = await publicClient.get<HealthzResponse>('/v1/status/healthz');
    const healthzTime = Date.now() - healthzStart;

    newServices.push({
      name: 'API Gateway',
      status: healthzResponse.data.status === 'healthy' ? 'healthy' : 'degraded',
      message: `Status: ${healthzResponse.data.status}`,
      responseTime: healthzTime,
    });
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : 'Connection failed';
    const isCorsError = errorMessage.includes('CORS') ||
                        errorMessage.includes('NetworkError') ||
                        errorMessage.includes('Network Error') ||
                        errorMessage.includes('timeout');
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
    const readyzResponse = await publicClient.get<ReadinessResponse>('/v1/status/readyz');
    const readyzTime = Date.now() - readyzStart;

    // Add individual component statuses
    if (readyzResponse.data.components) {
      for (const [componentName, componentData] of Object.entries(readyzResponse.data.components)) {
        const statusValue = typeof componentData === 'string'
          ? componentData
          : componentData.status;
        const messageValue = typeof componentData === 'string'
          ? componentData
          : (componentData.message || componentData.status);

        // Skip not_configured services
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
  } catch {
    // Readyz failed, but we already have healthz status
  }

  // Try to get platform health (includes functional tests)
  try {
    const platformResponse = await publicClient.get<PlatformHealthResponse>('/v1/platform/health');

    // Add service statuses from platform health
    if (platformResponse.data.services) {
      for (const [serviceName, serviceData] of Object.entries(platformResponse.data.services)) {
        // Skip if we already have this service
        const displayName = serviceName
          .split('_')
          .map(word => word.charAt(0).toUpperCase() + word.slice(1))
          .join(' ');

        if (newServices.some(s => s.name === displayName)) continue;

        let status: ServiceStatus = 'healthy';
        if (serviceData.status === 'degraded') {
          status = 'degraded';
        } else if (serviceData.status !== 'healthy') {
          status = 'unhealthy';
        }

        newServices.push({
          name: displayName,
          status,
          message: serviceData.message,
          responseTime: serviceData.response_ms,
        });
      }
    }

    // Add functional test results
    if (platformResponse.data.functional_tests) {
      for (const [testName, testData] of Object.entries(platformResponse.data.functional_tests)) {
        const displayName = testName
          .split('_')
          .map(word => word.charAt(0).toUpperCase() + word.slice(1))
          .join(' ') + ' Test';

        let status: ServiceStatus = 'healthy';
        if (testData.status === 'failed') {
          status = 'error';
        } else if (testData.status === 'skipped') {
          status = 'degraded';
        } else if (testData.status !== 'passed') {
          status = 'unhealthy';
        }

        newServices.push({
          name: displayName,
          status,
          message: testData.message,
        });
      }
    }
  } catch {
    // Platform health endpoint not available - this is fine for older deployments
  }

  // CORS connectivity test
  newServices.push({
    name: 'CORS Config',
    status: newServices[0]?.status === 'healthy' ? 'healthy' : 'unhealthy',
    message: newServices[0]?.status === 'healthy' ? 'Connected' : 'Check CORS',
  });

  return newServices;
}

/**
 * ServiceHealthCheck component
 *
 * Displays health status of backend services with traffic light indicators.
 * Uses publicClient for unauthenticated health check requests.
 *
 * Fixed re-rendering issue by using initialized.current ref to prevent
 * StrictMode double-execution.
 */
export function ServiceHealthCheck() {
  const [services, setServices] = useState<ServiceHealth[]>([
    { name: 'API Gateway', status: 'checking' },
  ]);
  const [expanded, setExpanded] = useState(false);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);

  // Prevent StrictMode double-execution and track mount state
  const initialized = useRef(false);

  useEffect(() => {
    // Prevent StrictMode double-execution
    if (initialized.current) return;
    initialized.current = true;

    let mounted = true;
    let intervalId: NodeJS.Timeout | null = null;

    const checkHealth = async () => {
      if (!mounted) return;
      const newServices = await performHealthCheck();
      if (mounted) {
        setServices(newServices);
        setLastChecked(new Date());
      }
    };

    // Initial check
    checkHealth();

    // Refresh every 30 seconds
    intervalId = setInterval(checkHealth, 30000);

    return () => {
      mounted = false;
      if (intervalId) clearInterval(intervalId);
    };
  }, []);

  const overallStatus = services.reduce<ServiceStatus>((worst, service) => {
    const priority: ServiceStatus[] = ['error', 'unhealthy', 'degraded', 'checking', 'healthy'];
    return priority.indexOf(service.status) < priority.indexOf(worst)
      ? service.status
      : worst;
  }, 'healthy');

  const healthyCount = services.filter(s => s.status === 'healthy').length;
  const totalCount = services.length;
  const hasErrors = services.some(s => s.status === 'error' || s.status === 'unhealthy');

  const handleRefresh = async () => {
    setServices([{ name: 'API Gateway', status: 'checking' }]);
    const newServices = await performHealthCheck();
    setServices(newServices);
    setLastChecked(new Date());
  };

  return (
    <div
      data-testid="service-health-check"
      className={`border rounded-lg shadow-sm ${
        hasErrors
          ? 'bg-red-50 border-red-200 dark:bg-red-900/20 dark:border-red-800'
          : 'bg-white border-gray-200 dark:bg-gray-800 dark:border-gray-700'
      }`}
    >
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-opacity-80 transition-colors rounded-lg"
      >
        <div className="flex items-center space-x-3">
          <TrafficLight status={overallStatus} />
          <span className="text-sm font-medium text-gray-700 dark:text-gray-200">
            Service Health
          </span>
          <span className={`text-xs ${hasErrors ? 'text-red-600 dark:text-red-400 font-medium' : 'text-gray-500 dark:text-gray-400'}`}>
            ({healthyCount}/{totalCount} healthy)
          </span>
        </div>
        <div className="flex items-center space-x-2">
          {lastChecked && (
            <span className="text-xs text-gray-400 dark:text-gray-500">
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
        <div className="px-4 pb-4 border-t border-gray-100 dark:border-gray-700">
          {hasErrors && (
            <div className="mt-3 p-3 bg-red-100 border border-red-200 rounded-md dark:bg-red-900/30 dark:border-red-800">
              <p className="text-xs text-red-800 dark:text-red-300 font-medium">
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
                    ? 'bg-red-50 dark:bg-red-900/20'
                    : service.status === 'degraded'
                    ? 'bg-yellow-50 dark:bg-yellow-900/20'
                    : 'bg-gray-50 dark:bg-gray-700/50'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-3">
                    <TrafficLight status={service.status} />
                    <span className="text-sm text-gray-700 dark:text-gray-200">{service.name}</span>
                  </div>
                  <div className="flex items-center space-x-3 text-xs">
                    {service.message && (
                      <span className={`${
                        service.status === 'error' || service.status === 'unhealthy'
                          ? 'text-red-600 dark:text-red-400'
                          : 'text-gray-500 dark:text-gray-400'
                      }`}>
                        {service.message}
                      </span>
                    )}
                    {service.responseTime && (
                      <span className="text-gray-400 dark:text-gray-500">{service.responseTime}ms</span>
                    )}
                  </div>
                </div>
                {service.details && (
                  <p className="mt-1 ml-6 text-xs text-gray-500 dark:text-gray-400">{service.details}</p>
                )}
              </div>
            ))}
          </div>

          <button
            onClick={handleRefresh}
            className="mt-3 w-full py-2 text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 font-medium transition-colors"
          >
            Refresh Status
          </button>
        </div>
      )}
    </div>
  );
}
