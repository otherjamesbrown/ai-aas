import { useState, useEffect } from 'react';
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
 *   const { status, isHealthy, refresh } = useHealthStatus();
 *
 *   return (
 *     <div>
 *       {Object.values(status).map(service => (
 *         <div key={service.name}>
 *           {service.name}: {service.status}
 *         </div>
 *       ))}
 *     </div>
 *   );
 * }
 * ```
 */
export function useHealthStatus() {
  const [status, setStatus] = useState<Record<string, ServiceHealth>>(
    healthMonitor.getStatus()
  );

  useEffect(() => {
    // Start health monitor if not already running
    healthMonitor.start();

    // Subscribe to status updates
    const unsubscribe = healthMonitor.subscribe(() => {
      setStatus(healthMonitor.getStatus());
    });

    // Get initial status
    setStatus(healthMonitor.getStatus());

    return () => {
      unsubscribe();
    };
  }, []);

  return {
    /** Current health status of all services */
    status,
    /** Whether all services are healthy */
    isHealthy: healthMonitor.isHealthy(),
    /** Force a health check refresh */
    refresh: () => healthMonitor.refresh(),
  };
}

export type { ServiceHealth };
