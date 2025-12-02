import { publicClient } from '@/lib/http/client';

type ServiceStatus = 'checking' | 'healthy' | 'degraded' | 'unhealthy' | 'error';

export interface ServiceHealth {
  name: string;
  status: ServiceStatus;
  message?: string;
  details?: string;
  responseTime?: number;
}

type HealthListener = () => void;

interface HealthzResponse {
  status: string;
}

interface ReadinessResponse {
  status: string;
  components?: Record<string, string | { status: string; message?: string }>;
}

interface PlatformHealthResponse {
  status: string;
  services?: Record<string, { status: string; message?: string; response_ms?: number }>;
  functional_tests?: Record<string, { status: string; message?: string }>;
}

/**
 * HealthMonitor - Singleton service for health check management
 *
 * Manages health check execution outside of React component lifecycle,
 * preventing re-rendering issues and StrictMode double-execution problems.
 *
 * Usage:
 * ```typescript
 * import { healthMonitor } from '@/services/healthMonitor';
 *
 * // Start monitoring
 * healthMonitor.start();
 *
 * // Subscribe to updates
 * const unsubscribe = healthMonitor.subscribe(() => {
 *   const status = healthMonitor.getStatus();
 *   console.log('Health updated:', status);
 * });
 *
 * // Stop monitoring
 * healthMonitor.stop();
 * ```
 */
class HealthMonitor {
  private status: Map<string, ServiceHealth> = new Map();
  private listeners: Set<HealthListener> = new Set();
  private intervalId: ReturnType<typeof setInterval> | null = null;
  private isRunning = false;

  /**
   * Start health monitoring
   */
  start(): void {
    if (this.isRunning) return;
    this.isRunning = true;

    // Perform initial check
    this.performCheck();

    // Schedule periodic checks every 30 seconds
    this.intervalId = setInterval(() => {
      this.performCheck();
    }, 30000);
  }

  /**
   * Stop health monitoring
   */
  stop(): void {
    if (!this.isRunning) return;
    this.isRunning = false;

    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }

  /**
   * Subscribe to health status updates
   * @returns Unsubscribe function
   */
  subscribe(listener: HealthListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  /**
   * Get current health status
   */
  getStatus(): Record<string, ServiceHealth> {
    return Object.fromEntries(this.status);
  }

  /**
   * Check if overall status is healthy
   */
  isHealthy(): boolean {
    if (this.status.size === 0) return false;
    for (const health of this.status.values()) {
      if (health.status === 'error' || health.status === 'unhealthy') {
        return false;
      }
    }
    return true;
  }

  /**
   * Notify all listeners of status update
   */
  private notifyListeners(): void {
    this.listeners.forEach(listener => listener());
  }

  /**
   * Perform health checks against all endpoints
   */
  private async performCheck(): Promise<void> {
    const newStatus = new Map<string, ServiceHealth>();
    const startTime = Date.now();

    // Check API Gateway health (healthz endpoint)
    try {
      const healthzStart = Date.now();
      const healthzResponse = await publicClient.get<HealthzResponse>('/v1/status/healthz');
      const healthzTime = Date.now() - healthzStart;

      newStatus.set('api-gateway', {
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
      newStatus.set('api-gateway', {
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

          newStatus.set(componentName, {
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
          if (newStatus.has(serviceName)) continue;

          const displayName = serviceName
            .split('_')
            .map(word => word.charAt(0).toUpperCase() + word.slice(1))
            .join(' ');

          let status: ServiceStatus = 'healthy';
          if (serviceData.status === 'degraded') {
            status = 'degraded';
          } else if (serviceData.status !== 'healthy') {
            status = 'unhealthy';
          }

          newStatus.set(serviceName, {
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

          newStatus.set(`test-${testName}`, {
            name: displayName,
            status,
            message: testData.message,
          });
        }
      }
    } catch {
      // Platform health endpoint not available - this is fine for older deployments
    }

    // CORS connectivity test based on API Gateway status
    const apiGatewayStatus = newStatus.get('api-gateway')?.status;
    newStatus.set('cors-config', {
      name: 'CORS Config',
      status: apiGatewayStatus === 'healthy' ? 'healthy' : 'unhealthy',
      message: apiGatewayStatus === 'healthy' ? 'Connected' : 'Check CORS',
    });

    // Update status and notify listeners
    this.status = newStatus;
    this.notifyListeners();
  }

  /**
   * Force a health check refresh
   */
  async refresh(): Promise<void> {
    await this.performCheck();
  }
}

/**
 * Singleton health monitor instance
 */
export const healthMonitor = new HealthMonitor();
