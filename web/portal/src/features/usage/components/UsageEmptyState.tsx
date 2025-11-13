
interface UsageEmptyStateProps {
  message?: string;
  showDocsLink?: boolean;
}

/**
 * Empty state component for usage dashboard
 * Shows when no usage data exists
 */
export function UsageEmptyState({
  message,
  showDocsLink = true,
}: UsageEmptyStateProps) {
  return (
    <div className="text-center py-12 px-4">
      <svg
        className="mx-auto h-12 w-12 text-gray-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        aria-hidden="true"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
        />
      </svg>
      <h3 className="mt-4 text-lg font-medium text-gray-900">No Usage Data</h3>
      <p className="mt-2 text-sm text-gray-500">
        {message ||
          "You don't have any usage data yet. Start using the API to see usage insights here."}
      </p>
      {showDocsLink && (
        <div className="mt-6">
          <a
            href="/docs/api"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center px-4 py-2 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-primary hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
          >
            View API Documentation
          </a>
        </div>
      )}
    </div>
  );
}

/**
 * Degraded state component for usage dashboard
 * Shows when data source is unavailable
 */
export function UsageDegradedState({
  onRetry,
  isRetrying = false,
}: {
  onRetry?: () => void;
  isRetrying?: boolean;
}) {
  return (
    <div className="bg-yellow-50 border-l-4 border-yellow-400 p-4 mb-6">
      <div className="flex">
        <div className="flex-shrink-0">
          <svg
            className="h-5 w-5 text-yellow-400"
            fill="currentColor"
            viewBox="0 0 20 20"
            aria-hidden="true"
          >
            <path
              fillRule="evenodd"
              d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
              clipRule="evenodd"
            />
          </svg>
        </div>
        <div className="ml-3 flex-1">
          <h3 className="text-sm font-medium text-yellow-800">
            Usage data temporarily unavailable
          </h3>
          <div className="mt-2 text-sm text-yellow-700">
            <p>
              We're having trouble fetching the latest usage data. You may be seeing cached or
              incomplete information. Please try again in a few moments.
            </p>
          </div>
          {onRetry && (
            <div className="mt-4">
              <button
                onClick={onRetry}
                disabled={isRetrying}
                className="bg-yellow-100 px-3 py-2 rounded-md text-sm font-medium text-yellow-800 hover:bg-yellow-200 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-yellow-500 disabled:opacity-50"
              >
                {isRetrying ? 'Retrying...' : 'Retry'}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

