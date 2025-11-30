import { useState } from 'react';
import { useHealthStatus, ServiceHealth } from '@/hooks/useHealthStatus';

type ServiceStatus = 'checking' | 'healthy' | 'degraded' | 'unhealthy' | 'error';

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
 * ServiceHealthCheck component
 *
 * Displays health status of backend services with traffic light indicators.
 * Uses healthMonitor singleton via useHealthStatus hook for centralized health management.
 *
 * This component is a thin UI layer - all health check logic is delegated to
 * the healthMonitor singleton which runs outside of React lifecycle.
 */
export function ServiceHealthCheck() {
  const { services, lastChecked, refresh, isRefreshing } = useHealthStatus();
  const [expanded, setExpanded] = useState(false);

  // Convert health monitor status to component status
  const mapStatus = (status: string): ServiceStatus => {
    switch (status) {
      case 'healthy':
        return 'healthy';
      case 'degraded':
        return 'degraded';
      case 'unhealthy':
      case 'error':
        return 'error';
      case 'checking':
        return 'checking';
      default:
        return 'checking';
    }
  };

  // Convert services object to array format
  const serviceList: Array<ServiceHealth & { displayStatus: ServiceStatus }> = Object.entries(services).map(
    ([name, service]) => ({
      ...service,
      name,
      displayStatus: mapStatus(service.status),
    })
  );

  // If no services yet, show initial checking state
  if (serviceList.length === 0) {
    serviceList.push({
      name: 'API Gateway',
      status: 'checking',
      displayStatus: 'checking',
    });
  }

  const overallStatus = serviceList.reduce<ServiceStatus>((worst, service) => {
    const priority: ServiceStatus[] = ['error', 'unhealthy', 'degraded', 'checking', 'healthy'];
    return priority.indexOf(service.displayStatus) < priority.indexOf(worst)
      ? service.displayStatus
      : worst;
  }, 'healthy');

  const healthyCount = serviceList.filter((s) => s.displayStatus === 'healthy').length;
  const totalCount = serviceList.length;
  const hasErrors = serviceList.some(
    (s) => s.displayStatus === 'error' || s.displayStatus === 'unhealthy'
  );

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
          <span
            className={`text-xs ${hasErrors ? 'text-red-600 dark:text-red-400 font-medium' : 'text-gray-500 dark:text-gray-400'}`}
          >
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
            {serviceList.map((service, index) => (
              <div
                key={index}
                className={`py-2 px-3 rounded-md ${
                  service.displayStatus === 'error' || service.displayStatus === 'unhealthy'
                    ? 'bg-red-50 dark:bg-red-900/20'
                    : service.displayStatus === 'degraded'
                      ? 'bg-yellow-50 dark:bg-yellow-900/20'
                      : 'bg-gray-50 dark:bg-gray-700/50'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-3">
                    <TrafficLight status={service.displayStatus} />
                    <span className="text-sm text-gray-700 dark:text-gray-200">{service.name}</span>
                  </div>
                  <div className="flex items-center space-x-3 text-xs">
                    {service.message && (
                      <span
                        className={`${
                          service.displayStatus === 'error' || service.displayStatus === 'unhealthy'
                            ? 'text-red-600 dark:text-red-400'
                            : 'text-gray-500 dark:text-gray-400'
                        }`}
                      >
                        {service.message}
                      </span>
                    )}
                    {service.responseTime && (
                      <span className="text-gray-400 dark:text-gray-500">
                        {service.responseTime}ms
                      </span>
                    )}
                  </div>
                </div>
                {service.details && (
                  <p className="mt-1 ml-6 text-xs text-gray-500 dark:text-gray-400">
                    {service.details}
                  </p>
                )}
              </div>
            ))}
          </div>

          <button
            onClick={refresh}
            disabled={isRefreshing}
            className="mt-3 w-full py-2 text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 font-medium transition-colors disabled:opacity-50"
          >
            {isRefreshing ? 'Refreshing...' : 'Refresh Status'}
          </button>
        </div>
      )}
    </div>
  );
}
