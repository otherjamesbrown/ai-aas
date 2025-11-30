import { useState, useEffect, useCallback } from 'react';
import { healthMonitor, ServiceHealth } from '@/services/healthMonitor';

/**
 * React hook for subscribing to health monitor status updates
 *
 * Automatically starts the health monitor on first subscription
 * and provides current health status.
 *
 * Usage:
 * ```typescript
 * function MyComponent() {
 *   const { services, lastChecked, refresh, isRefreshing, isHealthy } = useHealthStatus();
 *
 *   return (
 *     <div>
 *       {Object.entries(services).map(([key, service]) => (
 *         <div key={key}>
 *           {service.name}: {service.status}
 *         </div>
 *       ))}
 *     </div>
 *   );
 * }
 * ```
 */
export function useHealthStatus() {
  const [services, setServices] = useState<Record<string, ServiceHealth>>(
    healthMonitor.getStatus()
  );
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);

  useEffect(() => {
    // Start health monitor if not already running
    healthMonitor.start();

    // Subscribe to status updates
    const unsubscribe = healthMonitor.subscribe(() => {
      setServices(healthMonitor.getStatus());
      setLastChecked(new Date());
      setIsRefreshing(false);
    });

    // Get initial status
    const initialStatus = healthMonitor.getStatus();
    setServices(initialStatus);
    if (Object.keys(initialStatus).length > 0) {
      setLastChecked(new Date());
    }

    return () => {
      unsubscribe();
    };
  }, []);

  const refresh = useCallback(async () => {
    setIsRefreshing(true);
    await healthMonitor.refresh();
  }, []);

  return {
    /** Current health status of all services */
    services,
    /** @deprecated Use services instead */
    status: services,
    /** When the last health check was performed */
    lastChecked,
    /** Whether a refresh is currently in progress */
    isRefreshing,
    /** Whether all services are healthy */
    isHealthy: healthMonitor.isHealthy(),
    /** Force a health check refresh */
    refresh,
  };
}

export type { ServiceHealth };
