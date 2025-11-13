import { useToast } from '@/providers/ToastProvider';

/**
 * Hook-based error handler for functional components
 * Use this for async errors that React ErrorBoundary can't catch
 */
export function useErrorHandler() {
  const { showError } = useToast();

  return (error: Error, context?: string) => {
    console.error('Error caught:', context, error);
    showError(
      'An error occurred',
      context ? `${context}: ${error.message}` : error.message,
      0 // Persistent until dismissed
    );
  };
}

