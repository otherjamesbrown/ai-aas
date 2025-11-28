/**
 * Structured Logger for Frontend
 *
 * Provides structured JSON logging with:
 * - Log levels (debug, info, warn, error)
 * - Automatic context capture (correlation ID, user ID, component)
 * - Stack trace capture for errors
 * - Browser/environment metadata
 * - Remote logging support (extensible)
 *
 * Usage:
 * ```typescript
 * import { logger } from '@/lib/logger';
 *
 * logger.info('User logged in', { userId: '123' });
 * logger.error('API call failed', { endpoint: '/users' }, error);
 * logger.withContext({ component: 'UserList' }).debug('Fetching users');
 * ```
 */

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface LogContext {
  // Request/trace context
  correlationId?: string;
  traceId?: string;
  spanId?: string;
  requestId?: string;

  // User context
  userId?: string;
  organizationId?: string;
  email?: string;

  // Application context
  component?: string;
  action?: string;
  feature?: string;

  // Additional fields
  [key: string]: unknown;
}

export interface LogEntry {
  timestamp: string;
  level: LogLevel;
  message: string;
  context: LogContext;
  error?: {
    name: string;
    message: string;
    stack?: string;
  };
  browser?: {
    userAgent: string;
    url: string;
    referrer: string;
  };
  service: string;
  environment: string;
}

export interface LoggerConfig {
  level: LogLevel;
  serviceName: string;
  environment: string;
  enableConsole: boolean;
  enableRemote: boolean;
  remoteEndpoint?: string;
}

const LOG_LEVELS: Record<LogLevel, number> = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3,
};

const DEFAULT_CONFIG: LoggerConfig = {
  level: import.meta.env.DEV ? 'debug' : 'info',
  serviceName: 'web-portal',
  environment: import.meta.env.MODE || 'development',
  enableConsole: true,
  enableRemote: false,
};

class Logger {
  private config: LoggerConfig;
  private contextStack: LogContext[] = [];
  private remoteBuffer: LogEntry[] = [];
  private flushTimer: NodeJS.Timeout | null = null;

  constructor(config: Partial<LoggerConfig> = {}) {
    this.config = { ...DEFAULT_CONFIG, ...config };
  }

  /**
   * Creates a new logger with additional context
   */
  withContext(context: LogContext): Logger {
    const newLogger = new Logger(this.config);
    newLogger.contextStack = [...this.contextStack, context];
    return newLogger;
  }

  /**
   * Debug level log
   */
  debug(message: string, context?: LogContext): void {
    this.log('debug', message, context);
  }

  /**
   * Info level log
   */
  info(message: string, context?: LogContext): void {
    this.log('info', message, context);
  }

  /**
   * Warning level log
   */
  warn(message: string, context?: LogContext): void {
    this.log('warn', message, context);
  }

  /**
   * Error level log with optional Error object
   */
  error(message: string, context?: LogContext, error?: Error): void {
    this.log('error', message, context, error);
  }

  /**
   * Log HTTP request/response
   */
  httpRequest(
    method: string,
    url: string,
    statusCode: number,
    durationMs: number,
    context?: LogContext
  ): void {
    const level: LogLevel = statusCode >= 500 ? 'error' : statusCode >= 400 ? 'warn' : 'info';

    this.log(level, 'HTTP request', {
      ...context,
      httpMethod: method,
      httpUrl: url,
      httpStatus: statusCode,
      httpDurationMs: durationMs,
    });
  }

  /**
   * Log user action for analytics/debugging
   */
  userAction(action: string, target?: string, context?: LogContext): void {
    this.log('info', 'User action', {
      ...context,
      action,
      target,
      actionTimestamp: new Date().toISOString(),
    });
  }

  /**
   * Core logging method
   */
  private log(level: LogLevel, message: string, context?: LogContext, error?: Error): void {
    // Check log level
    if (LOG_LEVELS[level] < LOG_LEVELS[this.config.level]) {
      return;
    }

    const entry = this.buildLogEntry(level, message, context, error);

    // Console output
    if (this.config.enableConsole) {
      this.writeToConsole(entry);
    }

    // Remote logging (if enabled)
    if (this.config.enableRemote && level !== 'debug') {
      this.bufferForRemote(entry);
    }
  }

  /**
   * Build structured log entry
   */
  private buildLogEntry(
    level: LogLevel,
    message: string,
    context?: LogContext,
    error?: Error
  ): LogEntry {
    // Merge context from stack and provided context
    const mergedContext: LogContext = {};
    for (const ctx of this.contextStack) {
      Object.assign(mergedContext, ctx);
    }
    if (context) {
      Object.assign(mergedContext, context);
    }

    // Try to get correlation ID from current request if not provided
    if (!mergedContext.correlationId) {
      mergedContext.correlationId = this.getCurrentCorrelationId();
    }

    const entry: LogEntry = {
      timestamp: new Date().toISOString(),
      level,
      message,
      context: mergedContext,
      service: this.config.serviceName,
      environment: this.config.environment,
    };

    // Add error details
    if (error) {
      entry.error = {
        name: error.name,
        message: error.message,
        stack: error.stack,
      };
    }

    // Add browser metadata for errors
    if (level === 'error' && typeof window !== 'undefined') {
      entry.browser = {
        userAgent: navigator.userAgent,
        url: window.location.href,
        referrer: document.referrer,
      };
    }

    return entry;
  }

  /**
   * Get current correlation ID from session/request context
   */
  private getCurrentCorrelationId(): string | undefined {
    // Try to get from sessionStorage (set by API calls)
    if (typeof sessionStorage !== 'undefined') {
      const id = sessionStorage.getItem('current_correlation_id');
      if (id) return id;
    }
    return undefined;
  }

  /**
   * Write log entry to console with proper formatting
   */
  private writeToConsole(entry: LogEntry): void {
    const { level, message, context, error } = entry;

    // Build console args
    const args: unknown[] = [];

    // Add structured context if present
    if (Object.keys(context).length > 0) {
      args.push(context);
    }

    // Add error if present
    if (error) {
      args.push(error);
    }

    // Use appropriate console method
    switch (level) {
      case 'debug':
        console.debug(`[DEBUG] ${message}`, ...args);
        break;
      case 'info':
        console.info(`[INFO] ${message}`, ...args);
        break;
      case 'warn':
        console.warn(`[WARN] ${message}`, ...args);
        break;
      case 'error':
        console.error(`[ERROR] ${message}`, ...args);
        break;
    }
  }

  /**
   * Buffer log entries for remote sending
   */
  private bufferForRemote(entry: LogEntry): void {
    this.remoteBuffer.push(entry);

    // Flush after buffer size or timeout
    if (this.remoteBuffer.length >= 10) {
      this.flushToRemote();
    } else if (!this.flushTimer) {
      this.flushTimer = setTimeout(() => this.flushToRemote(), 5000);
    }
  }

  /**
   * Send buffered logs to remote endpoint
   */
  private async flushToRemote(): Promise<void> {
    if (this.flushTimer) {
      clearTimeout(this.flushTimer);
      this.flushTimer = null;
    }

    if (this.remoteBuffer.length === 0 || !this.config.remoteEndpoint) {
      return;
    }

    const entries = [...this.remoteBuffer];
    this.remoteBuffer = [];

    try {
      await fetch(this.config.remoteEndpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ logs: entries }),
        keepalive: true, // Ensure logs are sent even on page unload
      });
    } catch {
      // Re-buffer on failure (with limit)
      if (this.remoteBuffer.length < 100) {
        this.remoteBuffer.push(...entries);
      }
    }
  }

  /**
   * Flush any pending logs (call on page unload)
   */
  flush(): void {
    this.flushToRemote();
  }
}

// Export singleton instance
export const logger = new Logger();

// Export for creating custom instances
export { Logger };
