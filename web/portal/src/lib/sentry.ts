/**
 * Sentry Error Tracking Integration
 *
 * Provides centralized error tracking and monitoring with:
 * - Automatic error capture and reporting
 * - Session replay for error reproduction
 * - Performance monitoring and tracing
 * - Sensitive data scrubbing
 * - Environment-based configuration
 *
 * Usage:
 * ```typescript
 * import { initSentry, captureError, setUserContext } from '@/lib/sentry';
 *
 * // Initialize once in main.tsx
 * initSentry();
 *
 * // Manual error capture
 * try {
 *   riskyOperation();
 * } catch (error) {
 *   captureError(error, { context: 'additional info' });
 * }
 *
 * // Set user context for better debugging
 * setUserContext({ id: '123', email: 'user@example.com' });
 * ```
 */

import * as Sentry from '@sentry/react';

/**
 * Initialize Sentry SDK with configuration from environment variables
 * Should be called once at application startup, before React renders
 */
export function initSentry(): void {
  const dsn = import.meta.env.VITE_SENTRY_DSN;

  // Skip initialization if DSN is not configured
  if (!dsn) {
    if (import.meta.env.DEV) {
      console.info('[Sentry] DSN not configured, error tracking disabled');
    }
    return;
  }

  const environment = import.meta.env.MODE || 'development';
  const isProduction = import.meta.env.PROD;

  Sentry.init({
    dsn,
    environment,

    integrations: [
      Sentry.browserTracingIntegration({
        // Trace HTTP requests to our API endpoints
        tracePropagationTargets: [
          'localhost',
          /^https:\/\/api\./,
          /^https:\/\/.*\.ai-aas\.local/,
          /^https:\/\/.*\.otherjamesbrown\.com/,
        ],
      }),
      Sentry.replayIntegration({
        // Mask sensitive input fields (passwords, etc.)
        maskAllText: false,
        maskAllInputs: true,
        blockAllMedia: false,
        // Network request/response capture
        networkDetailAllowUrls: [
          /^https:\/\/api\./,
          /^https:\/\/.*\.ai-aas\.local/,
        ],
      }),
    ],

    // Performance monitoring sample rate
    // In production: 10% of transactions, in dev: 100%
    tracesSampleRate: isProduction ? 0.1 : 1.0,

    // Session replay sampling
    // Normal sessions: 0% (only capture when errors occur)
    // Error sessions: 100% (always capture when an error happens)
    replaysSessionSampleRate: 0,
    replaysOnErrorSampleRate: 1.0,

    // Filter out known noise and non-actionable errors
    ignoreErrors: [
      // Browser extension errors
      'ResizeObserver loop',
      'ResizeObserver loop completed with undelivered notifications',
      // Network errors that aren't actionable
      'Network request failed',
      'Failed to fetch',
      'NetworkError',
      // Abort errors (user-initiated)
      'AbortError',
      'The operation was aborted',
      // Chrome extensions
      'chrome-extension://',
      'moz-extension://',
      // Safari extensions
      'safari-extension://',
      // Random scripts
      'top.GLOBALS',
      // Facebook related
      'fb_xd_fragment',
    ],

    // URL patterns to ignore (don't send to Sentry)
    denyUrls: [
      // Browser extensions
      /extensions\//i,
      /^chrome:\/\//i,
      /^chrome-extension:\/\//i,
      /^moz-extension:\/\//i,
      // Local files
      /^file:\/\//i,
    ],

    // Scrub sensitive data before sending to Sentry
    beforeSend(event, hint) {
      // Scrub authorization headers
      if (event.request?.headers) {
        const headers = event.request.headers;
        delete headers['Authorization'];
        delete headers['authorization'];
        delete headers['X-API-Key'];
        delete headers['x-api-key'];
        delete headers['Cookie'];
        delete headers['cookie'];
      }

      // Scrub sensitive data from request bodies
      if (event.request?.data) {
        const data = event.request.data as Record<string, unknown>;
        if (typeof data === 'object' && data !== null) {
          // Remove common sensitive fields
          const sensitiveFields = ['password', 'token', 'apiKey', 'secret', 'credentials'];
          for (const field of sensitiveFields) {
            if (field in data) {
              data[field] = '[Filtered]';
            }
          }
        }
      }

      // Scrub sensitive data from breadcrumbs
      if (event.breadcrumbs) {
        event.breadcrumbs = event.breadcrumbs.map((breadcrumb) => {
          if (breadcrumb.data) {
            const data = breadcrumb.data as Record<string, unknown>;
            if ('password' in data) data.password = '[Filtered]';
            if ('token' in data) data.token = '[Filtered]';
            if ('apiKey' in data) data.apiKey = '[Filtered]';
          }
          return breadcrumb;
        });
      }

      // In development, also log to console
      if (import.meta.env.DEV && hint.originalException) {
        console.error('[Sentry] Capturing error:', hint.originalException);
      }

      return event;
    },

    // Add release information if available
    release: import.meta.env.VITE_APP_VERSION,
  });

  console.info(`[Sentry] Initialized for environment: ${environment}`);
}

/**
 * Manually capture an error with additional context
 */
export function captureError(
  error: Error | unknown,
  context?: Record<string, unknown>
): string {
  const eventId = Sentry.captureException(error, {
    contexts: {
      additional: context,
    },
  });
  return eventId;
}

/**
 * Manually capture a message (for non-error events)
 */
export function captureMessage(
  message: string,
  level: Sentry.SeverityLevel = 'info',
  context?: Record<string, unknown>
): string {
  const eventId = Sentry.captureMessage(message, {
    level,
    contexts: {
      additional: context,
    },
  });
  return eventId;
}

/**
 * Set user context for better error attribution
 * Call this after user login
 */
export function setUserContext(user: {
  id: string;
  email?: string;
  username?: string;
  organizationId?: string;
}): void {
  Sentry.setUser({
    id: user.id,
    email: user.email,
    username: user.username,
    // Custom fields
    organizationId: user.organizationId,
  });
}

/**
 * Clear user context (call on logout)
 */
export function clearUserContext(): void {
  Sentry.setUser(null);
}

/**
 * Add breadcrumb for debugging context
 */
export function addBreadcrumb(
  message: string,
  category: string,
  level: Sentry.SeverityLevel = 'info',
  data?: Record<string, unknown>
): void {
  Sentry.addBreadcrumb({
    message,
    category,
    level,
    data,
    timestamp: Date.now() / 1000,
  });
}

/**
 * Set custom context for errors
 */
export function setContext(name: string, context: Record<string, unknown>): void {
  Sentry.setContext(name, context);
}

/**
 * Add tags for error grouping and filtering
 */
export function setTags(tags: Record<string, string>): void {
  Sentry.setTags(tags);
}

/**
 * Create a Sentry error boundary wrapper for React components
 * This is a re-export for convenience
 */
export const ErrorBoundary = Sentry.ErrorBoundary;

/**
 * HOC to wrap components with error boundary
 */
export const withErrorBoundary = Sentry.withErrorBoundary;

/**
 * Hook to get the current Sentry scope
 */
export const useScope = Sentry.getCurrentScope;

// Re-export Sentry for advanced usage
export { Sentry };
