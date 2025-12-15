import { Component, ErrorInfo, ReactNode } from 'react';
import { logger } from '@/lib/logger';
import { captureError } from '@/lib/sentry';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
  /** Component name for logging context */
  component?: string;
}

interface State {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
  /** Sentry event ID for error tracking */
  sentryEventId: string | null;
}

/**
 * Error boundary component to catch React errors
 * Provides fallback UI, structured error logging, and error reporting
 */
export class ErrorBoundary extends Component<Props, State> {
  private componentLogger = logger.withContext({ component: 'ErrorBoundary' });

  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null, errorInfo: null, sentryEventId: null };
  }

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    // Capture error in Sentry with component stack context
    const sentryEventId = captureError(error, {
      errorBoundary: this.props.component || 'unknown',
      componentStack: errorInfo.componentStack,
      react: {
        componentStack: errorInfo.componentStack,
      },
    });

    // Update state with error info and Sentry event ID
    this.setState({ errorInfo, sentryEventId });

    // Log error with structured context
    this.componentLogger.error(
      'React error boundary caught error',
      {
        errorBoundary: this.props.component || 'unknown',
        componentStack: errorInfo.componentStack,
        sentryEventId,
      },
      error
    );

    // Call optional error handler
    this.props.onError?.(error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="min-h-screen flex items-center justify-center bg-gray-50 px-4">
          <div className="max-w-md w-full bg-white shadow-lg rounded-lg p-6">
            <div className="flex items-center mb-4">
              <svg
                className="h-8 w-8 text-red-500 mr-3"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <h1 className="text-xl font-bold text-gray-900">Something went wrong</h1>
            </div>
            <p className="text-gray-600 mb-4">
              We're sorry, but something unexpected happened. Please try refreshing the page.
            </p>
            {this.state.sentryEventId && (
              <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-md">
                <p className="text-sm text-gray-700 mb-1">
                  <span className="font-medium">Error ID:</span>{' '}
                  <code className="text-xs bg-white px-2 py-1 rounded border border-gray-300">
                    {this.state.sentryEventId}
                  </code>
                </p>
                <p className="text-xs text-gray-600 mt-2">
                  Our team has been notified. Please include this ID when reporting the issue.
                </p>
              </div>
            )}
            {process.env.NODE_ENV === 'development' && this.state.error && (
              <details className="mb-4">
                <summary className="cursor-pointer text-sm text-gray-500 mb-2">
                  Error details (development only)
                </summary>
                <pre className="text-xs bg-gray-100 p-3 rounded overflow-auto">
                  {this.state.error.toString()}
                  {this.state.error.stack}
                </pre>
                {this.state.errorInfo?.componentStack && (
                  <>
                    <p className="text-xs text-gray-500 mt-2 mb-1">Component Stack:</p>
                    <pre className="text-xs bg-gray-100 p-3 rounded overflow-auto">
                      {this.state.errorInfo.componentStack}
                    </pre>
                  </>
                )}
              </details>
            )}
            <div className="flex space-x-3">
              <button
                onClick={() => window.location.reload()}
                className="flex-1 px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
              >
                Refresh Page
              </button>
              <button
                onClick={() => this.setState({ hasError: false, error: null, errorInfo: null, sentryEventId: null })}
                className="flex-1 px-4 py-2 border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
              >
                Try Again
              </button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}


