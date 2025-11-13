interface StaleDataBannerProps {
  lastUpdated: string; // ISO-8601 timestamp
  onRefresh: () => void;
  isRefreshing?: boolean;
}

/**
 * Banner component that surfaces stale data conflicts
 * Shows when data may be outdated and provides refresh action
 */
export function StaleDataBanner({
  lastUpdated,
  onRefresh,
  isRefreshing = false,
}: StaleDataBannerProps) {
  const lastUpdatedDate = new Date(lastUpdated);
  const now = new Date();
  const ageMinutes = Math.floor((now.getTime() - lastUpdatedDate.getTime()) / 1000 / 60);

  // Show warning if data is older than 5 minutes
  if (ageMinutes < 5) return null;

  return (
    <div
      className="bg-yellow-50 border-l-4 border-yellow-400 p-4 mb-4"
      role="alert"
      aria-live="polite"
    >
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
          <p className="text-sm text-yellow-700">
            This data was last updated {ageMinutes} minute{ageMinutes !== 1 ? 's' : ''} ago.
            It may be outdated due to concurrent edits.
          </p>
          <div className="mt-2">
            <button
              onClick={onRefresh}
              disabled={isRefreshing}
              className="text-sm font-medium text-yellow-800 hover:text-yellow-900 focus:outline-none focus:underline disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isRefreshing ? 'Refreshing...' : 'Refresh data'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

