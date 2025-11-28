import { useToast } from '@/providers/ToastProvider';
import { logger } from '@/lib/logger';

/**
 * Hook-based error handler for functional components
 * Use this for async errors that React ErrorBoundary can't catch
 *
 * Usage:
 * ```tsx
 * const handleError = useErrorHandler();
 *
 * try {
 *   await fetchData();
 * } catch (error) {
 *   handleError(error as Error, 'fetchData');
 * }
 * ```
 */
export function useErrorHandler() {
  const { showError } = useToast();

  return (error: Error, context?: string) => {
    // Log with structured logger
    logger.error('Async error caught', { context, action: 'useErrorHandler' }, error);

    // Show user-friendly toast
    showError(
      'An error occurred',
      context ? `${context}: ${error.message}` : error.message,
      0 // Persistent until dismissed
    );
  };
}

