import { describe, it, expect, beforeEach, afterEach, vi, type MockInstance } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useLogger } from './index';

describe('useLogger', () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let consoleInfoSpy: MockInstance<any[], any>;

  beforeEach(() => {
    consoleInfoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should return a logger instance', () => {
    const { result } = renderHook(() => useLogger());

    expect(result.current).toBeDefined();
    expect(typeof result.current.info).toBe('function');
    expect(typeof result.current.debug).toBe('function');
    expect(typeof result.current.warn).toBe('function');
    expect(typeof result.current.error).toBe('function');
  });

  it('should create logger with component name as string', () => {
    const { result } = renderHook(() => useLogger('TestComponent'));

    result.current.info('Test message');

    expect(consoleInfoSpy).toHaveBeenCalledWith(
      '[INFO] Test message',
      expect.objectContaining({
        component: 'TestComponent',
      })
    );
  });

  it('should create logger with context object', () => {
    const { result } = renderHook(() =>
      useLogger({
        component: 'UserList',
        feature: 'user-management',
      })
    );

    result.current.info('Test message');

    expect(consoleInfoSpy).toHaveBeenCalledWith(
      '[INFO] Test message',
      expect.objectContaining({
        component: 'UserList',
        feature: 'user-management',
      })
    );
  });

  it('should create logger without context', () => {
    const { result } = renderHook(() => useLogger());

    result.current.info('Test message');

    expect(consoleInfoSpy).toHaveBeenCalledWith('[INFO] Test message', expect.any(Object));
  });

  it('should memoize logger instance', () => {
    const { result, rerender } = renderHook(
      ({ component }: { component?: string }) => useLogger(component),
      {
        initialProps: { component: 'TestComponent' },
      }
    );

    const firstLogger = result.current;

    // Rerender with same props
    rerender({ component: 'TestComponent' });
    expect(result.current).toBe(firstLogger);

    // Rerender with different props
    rerender({ component: 'DifferentComponent' });
    expect(result.current).not.toBe(firstLogger);
  });

  it('should work with different log levels', () => {
    const consoleDebugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});
    const consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    const { result } = renderHook(() => useLogger('TestComponent'));

    result.current.debug('Debug message');
    result.current.info('Info message');
    result.current.warn('Warning message');
    result.current.error('Error message');

    // Debug might not be called depending on log level config
    expect(consoleInfoSpy).toHaveBeenCalled();
    expect(consoleWarnSpy).toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalled();

    consoleDebugSpy.mockRestore();
    consoleWarnSpy.mockRestore();
    consoleErrorSpy.mockRestore();
  });

  it('should allow chaining with withContext', () => {
    const { result } = renderHook(() => useLogger('TestComponent'));

    const chainedLogger = result.current.withContext({ userId: '123' });
    chainedLogger.info('User action');

    expect(consoleInfoSpy).toHaveBeenCalledWith(
      '[INFO] User action',
      expect.objectContaining({
        component: 'TestComponent',
        userId: '123',
      })
    );
  });

  it('should support logging with additional context', () => {
    const { result } = renderHook(() => useLogger('TestComponent'));

    result.current.info('Button clicked', {
      action: 'click',
      target: 'submit-button',
    });

    expect(consoleInfoSpy).toHaveBeenCalledWith(
      '[INFO] Button clicked',
      expect.objectContaining({
        component: 'TestComponent',
        action: 'click',
        target: 'submit-button',
      })
    );
  });

  it('should handle complex context objects', () => {
    const { result } = renderHook(() =>
      useLogger({
        component: 'UserDashboard',
        feature: 'analytics',
        userId: '123',
        organizationId: 'org-456',
      })
    );

    result.current.info('Dashboard loaded');

    expect(consoleInfoSpy).toHaveBeenCalledWith(
      '[INFO] Dashboard loaded',
      expect.objectContaining({
        component: 'UserDashboard',
        feature: 'analytics',
        userId: '123',
        organizationId: 'org-456',
      })
    );
  });

  it('should work with HTTP request logging', () => {
    const { result } = renderHook(() => useLogger('ApiClient'));

    result.current.httpRequest('GET', '/api/users', 200, 150);

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

  it('should work with user action logging', () => {
    const { result } = renderHook(() => useLogger('UserProfile'));

    result.current.userAction('edit-profile', 'save-button', { field: 'email' });

    expect(consoleInfoSpy).toHaveBeenCalledWith(
      '[INFO] User action',
      expect.objectContaining({
        action: 'edit-profile',
        target: 'save-button',
        field: 'email',
      })
    );
  });

  it('should handle errors with Error objects', () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const { result } = renderHook(() => useLogger('ErrorComponent'));

    const error = new Error('Test error');
    result.current.error('An error occurred', undefined, error);

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      '[ERROR] An error occurred',
      expect.any(Object),
      expect.objectContaining({
        name: 'Error',
        message: 'Test error',
      })
    );

    consoleErrorSpy.mockRestore();
  });
});
