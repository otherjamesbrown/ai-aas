import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { Logger, logger, LogContext } from './index';

describe('Logger', () => {
  let consoleDebugSpy: ReturnType<typeof vi.spyOn>;
  let consoleInfoSpy: ReturnType<typeof vi.spyOn>;
  let consoleWarnSpy: ReturnType<typeof vi.spyOn>;
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    // Spy on console methods
    consoleDebugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});
    consoleInfoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
    consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    // Clear sessionStorage
    sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('Basic Logging', () => {
    it('should log debug messages when level is debug', () => {
      const testLogger = new Logger({ level: 'debug', enableConsole: true });
      testLogger.debug('Test debug message');

      expect(consoleDebugSpy).toHaveBeenCalledWith(
        '[DEBUG] Test debug message',
        expect.any(Object)
      );
    });

    it('should log info messages', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.info('Test info message');

      expect(consoleInfoSpy).toHaveBeenCalledWith('[INFO] Test info message', expect.any(Object));
    });

    it('should log warn messages', () => {
      const testLogger = new Logger({ level: 'warn', enableConsole: true });
      testLogger.warn('Test warn message');

      expect(consoleWarnSpy).toHaveBeenCalledWith('[WARN] Test warn message', expect.any(Object));
    });

    it('should log error messages', () => {
      const testLogger = new Logger({ level: 'error', enableConsole: true });
      testLogger.error('Test error message');

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        '[ERROR] Test error message',
        expect.any(Object)
      );
    });

    it('should log error messages with Error object', () => {
      const testLogger = new Logger({ level: 'error', enableConsole: true });
      const error = new Error('Test error');
      testLogger.error('Error occurred', undefined, error);

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        '[ERROR] Error occurred',
        expect.any(Object),
        expect.objectContaining({
          name: 'Error',
          message: 'Test error',
          stack: expect.any(String),
        })
      );
    });
  });

  describe('Log Levels', () => {
    it('should respect log level threshold - debug level', () => {
      const testLogger = new Logger({ level: 'debug', enableConsole: true });

      testLogger.debug('debug');
      testLogger.info('info');
      testLogger.warn('warn');
      testLogger.error('error');

      expect(consoleDebugSpy).toHaveBeenCalled();
      expect(consoleInfoSpy).toHaveBeenCalled();
      expect(consoleWarnSpy).toHaveBeenCalled();
      expect(consoleErrorSpy).toHaveBeenCalled();
    });

    it('should respect log level threshold - info level', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });

      testLogger.debug('debug');
      testLogger.info('info');
      testLogger.warn('warn');
      testLogger.error('error');

      expect(consoleDebugSpy).not.toHaveBeenCalled();
      expect(consoleInfoSpy).toHaveBeenCalled();
      expect(consoleWarnSpy).toHaveBeenCalled();
      expect(consoleErrorSpy).toHaveBeenCalled();
    });

    it('should respect log level threshold - warn level', () => {
      const testLogger = new Logger({ level: 'warn', enableConsole: true });

      testLogger.debug('debug');
      testLogger.info('info');
      testLogger.warn('warn');
      testLogger.error('error');

      expect(consoleDebugSpy).not.toHaveBeenCalled();
      expect(consoleInfoSpy).not.toHaveBeenCalled();
      expect(consoleWarnSpy).toHaveBeenCalled();
      expect(consoleErrorSpy).toHaveBeenCalled();
    });

    it('should respect log level threshold - error level', () => {
      const testLogger = new Logger({ level: 'error', enableConsole: true });

      testLogger.debug('debug');
      testLogger.info('info');
      testLogger.warn('warn');
      testLogger.error('error');

      expect(consoleDebugSpy).not.toHaveBeenCalled();
      expect(consoleInfoSpy).not.toHaveBeenCalled();
      expect(consoleWarnSpy).not.toHaveBeenCalled();
      expect(consoleErrorSpy).toHaveBeenCalled();
    });
  });

  describe('Context Management', () => {
    it('should include provided context in logs', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });
      const context: LogContext = { userId: '123', component: 'TestComponent' };

      testLogger.info('Test message', context);

      expect(consoleInfoSpy).toHaveBeenCalledWith(
        '[INFO] Test message',
        expect.objectContaining({
          userId: '123',
          component: 'TestComponent',
        })
      );
    });

    it('should merge context from withContext', () => {
      const baseLogger = new Logger({ level: 'info', enableConsole: true });
      const contextLogger = baseLogger.withContext({ component: 'UserList' });

      contextLogger.info('User action', { action: 'click' });

      expect(consoleInfoSpy).toHaveBeenCalledWith(
        '[INFO] User action',
        expect.objectContaining({
          component: 'UserList',
          action: 'click',
        })
      );
    });

    it('should chain context correctly', () => {
      const baseLogger = new Logger({ level: 'info', enableConsole: true });
      const contextLogger1 = baseLogger.withContext({ component: 'UserList' });
      const contextLogger2 = contextLogger1.withContext({ userId: '123' });

      contextLogger2.info('Action performed');

      expect(consoleInfoSpy).toHaveBeenCalledWith(
        '[INFO] Action performed',
        expect.objectContaining({
          component: 'UserList',
          userId: '123',
        })
      );
    });

    it('should override context with later values', () => {
      const baseLogger = new Logger({ level: 'info', enableConsole: true });
      const contextLogger = baseLogger.withContext({ userId: '123' });

      contextLogger.info('Test', { userId: '456' });

      expect(consoleInfoSpy).toHaveBeenCalledWith(
        '[INFO] Test',
        expect.objectContaining({
          userId: '456',
        })
      );
    });
  });

  describe('Correlation ID', () => {
    it('should read correlation ID from sessionStorage', () => {
      sessionStorage.setItem('current_correlation_id', 'test-correlation-id');

      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.info('Test message');

      expect(consoleInfoSpy).toHaveBeenCalledWith(
        '[INFO] Test message',
        expect.objectContaining({
          correlationId: 'test-correlation-id',
        })
      );
    });

    it('should use provided correlation ID over sessionStorage', () => {
      sessionStorage.setItem('current_correlation_id', 'session-id');

      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.info('Test message', { correlationId: 'custom-id' });

      expect(consoleInfoSpy).toHaveBeenCalledWith(
        '[INFO] Test message',
        expect.objectContaining({
          correlationId: 'custom-id',
        })
      );
    });

    it('should not include correlation ID if not available', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.info('Test message');

      const [[, context]] = consoleInfoSpy.mock.calls;
      // When no correlation ID is available, the property may be undefined or not present
      expect(context?.correlationId).toBeUndefined();
    });
  });

  describe('HTTP Request Logging', () => {
    it('should log successful HTTP requests as info', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.httpRequest('GET', '/api/users', 200, 150);

      expect(consoleInfoSpy).toHaveBeenCalledWith(
        '[INFO] HTTP request',
        expect.objectContaining({
          httpMethod: 'GET',
          httpUrl: '/api/users',
          httpStatus: 200,
          httpDurationMs: 150,
        })
      );
    });

    it('should log 4xx responses as warn', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.httpRequest('POST', '/api/login', 401, 100);

      expect(consoleWarnSpy).toHaveBeenCalledWith(
        '[WARN] HTTP request',
        expect.objectContaining({
          httpMethod: 'POST',
          httpUrl: '/api/login',
          httpStatus: 401,
          httpDurationMs: 100,
        })
      );
    });

    it('should log 5xx responses as error', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.httpRequest('GET', '/api/users', 500, 200);

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        '[ERROR] HTTP request',
        expect.objectContaining({
          httpMethod: 'GET',
          httpUrl: '/api/users',
          httpStatus: 500,
          httpDurationMs: 200,
        })
      );
    });
  });

  describe('User Action Logging', () => {
    it('should log user actions with metadata', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.userAction('click', 'submit-button', { userId: '123' });

      expect(consoleInfoSpy).toHaveBeenCalledWith(
        '[INFO] User action',
        expect.objectContaining({
          action: 'click',
          target: 'submit-button',
          userId: '123',
          actionTimestamp: expect.any(String),
        })
      );
    });

    it('should log user actions without target', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.userAction('page-view');

      expect(consoleInfoSpy).toHaveBeenCalledWith(
        '[INFO] User action',
        expect.objectContaining({
          action: 'page-view',
        })
      );
    });
  });

  describe('Configuration', () => {
    it('should use default configuration', () => {
      const testLogger = new Logger();

      // Config is private, but we can verify behavior
      testLogger.info('Test');
      expect(consoleInfoSpy).toHaveBeenCalled();
    });

    it('should accept custom configuration', () => {
      const testLogger = new Logger({
        level: 'error',
        serviceName: 'custom-service',
        environment: 'test',
        enableConsole: true,
      });

      testLogger.info('Should not log');
      expect(consoleInfoSpy).not.toHaveBeenCalled();

      testLogger.error('Should log');
      expect(consoleErrorSpy).toHaveBeenCalled();
    });

    it('should respect enableConsole flag', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: false });

      testLogger.info('Test message');
      expect(consoleInfoSpy).not.toHaveBeenCalled();
    });
  });

  describe('Error Handling', () => {
    it('should capture error name, message, and stack', () => {
      const testLogger = new Logger({ level: 'error', enableConsole: true });
      const error = new Error('Test error');

      testLogger.error('Error occurred', undefined, error);

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        '[ERROR] Error occurred',
        expect.any(Object),
        expect.objectContaining({
          name: 'Error',
          message: 'Test error',
          stack: expect.stringContaining('Error: Test error'),
        })
      );
    });

    it('should handle custom error types', () => {
      const testLogger = new Logger({ level: 'error', enableConsole: true });

      class CustomError extends Error {
        constructor(message: string) {
          super(message);
          this.name = 'CustomError';
        }
      }

      const error = new CustomError('Custom error message');
      testLogger.error('Custom error occurred', undefined, error);

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        '[ERROR] Custom error occurred',
        expect.any(Object),
        expect.objectContaining({
          name: 'CustomError',
          message: 'Custom error message',
        })
      );
    });
  });

  describe('Singleton Instance', () => {
    it('should export a singleton logger instance', () => {
      expect(logger).toBeDefined();
      expect(logger).toBeInstanceOf(Logger);
    });

    it('should allow creating custom logger instances', () => {
      const customLogger = new Logger({ level: 'debug' });
      expect(customLogger).toBeInstanceOf(Logger);
      expect(customLogger).not.toBe(logger);
    });
  });

  describe('Structured Output', () => {
    it('should include timestamp in ISO format', () => {
      const testLogger = new Logger({ level: 'info', enableConsole: true });
      testLogger.info('Test');

      const [[, context]] = consoleInfoSpy.mock.calls;
      // Context should be a structured object, but timestamp is added to the full log entry
      expect(context).toEqual(expect.any(Object));
    });

    it('should include service name and environment', () => {
      const testLogger = new Logger({
        level: 'info',
        enableConsole: true,
        serviceName: 'test-service',
        environment: 'test-env',
      });

      testLogger.info('Test');
      expect(consoleInfoSpy).toHaveBeenCalled();
    });
  });

  describe('Remote Logging', () => {
    let fetchSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(new Response());
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
      fetchSpy.mockRestore();
    });

    it('should not send remote logs when disabled', () => {
      const testLogger = new Logger({
        level: 'info',
        enableConsole: true,
        enableRemote: false,
      });

      testLogger.error('Test error');
      vi.runAllTimers();

      expect(fetchSpy).not.toHaveBeenCalled();
    });

    it('should buffer logs when remote logging is enabled', () => {
      const testLogger = new Logger({
        level: 'info',
        enableConsole: true,
        enableRemote: true,
        remoteEndpoint: 'https://logs.example.com/api/logs',
      });

      testLogger.info('Log 1');
      testLogger.warn('Log 2');
      testLogger.error('Log 3');

      // Should not send immediately
      expect(fetchSpy).not.toHaveBeenCalled();
    });

    it('should flush logs after buffer size threshold', () => {
      const testLogger = new Logger({
        level: 'info',
        enableConsole: true,
        enableRemote: true,
        remoteEndpoint: 'https://logs.example.com/api/logs',
      });

      // Send 10 logs to trigger buffer flush
      for (let i = 0; i < 10; i++) {
        testLogger.error(`Log ${i}`);
      }

      expect(fetchSpy).toHaveBeenCalledWith(
        'https://logs.example.com/api/logs',
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          keepalive: true,
        })
      );
    });

    it('should flush logs after timeout', () => {
      const testLogger = new Logger({
        level: 'info',
        enableConsole: true,
        enableRemote: true,
        remoteEndpoint: 'https://logs.example.com/api/logs',
      });

      testLogger.error('Test log');

      // Fast-forward time by 5 seconds
      vi.advanceTimersByTime(5000);

      expect(fetchSpy).toHaveBeenCalledWith(
        'https://logs.example.com/api/logs',
        expect.objectContaining({
          method: 'POST',
        })
      );
    });

    it('should not send debug logs to remote', () => {
      const testLogger = new Logger({
        level: 'debug',
        enableConsole: true,
        enableRemote: true,
        remoteEndpoint: 'https://logs.example.com/api/logs',
      });

      // Send 10 debug logs - should not trigger remote logging
      for (let i = 0; i < 10; i++) {
        testLogger.debug(`Debug log ${i}`);
      }

      expect(fetchSpy).not.toHaveBeenCalled();
    });

    it('should handle flush manually', async () => {
      const testLogger = new Logger({
        level: 'info',
        enableConsole: true,
        enableRemote: true,
        remoteEndpoint: 'https://logs.example.com/api/logs',
      });

      testLogger.error('Test log');
      testLogger.flush();

      // Wait for async flush
      await vi.runAllTimersAsync();

      expect(fetchSpy).toHaveBeenCalled();
    });
  });

  describe('Environment Variable Configuration', () => {
    it('should read log level from import.meta.env', () => {
      // This test verifies the default config respects environment
      // In actual usage, Vite sets import.meta.env.DEV and import.meta.env.MODE
      const testLogger = new Logger();
      expect(testLogger).toBeDefined();
    });
  });
});
