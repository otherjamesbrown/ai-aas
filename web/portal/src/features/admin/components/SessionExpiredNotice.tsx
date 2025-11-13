import { useEffect, useState } from 'react';
import { useAuth } from '@/providers/AuthProvider';

interface SessionExpiredNoticeProps {
  onReauth?: () => void;
}

/**
 * Session expiry notice component
 * Detects session expiry and shows re-authentication banner
 */
export function SessionExpiredNotice({ onReauth }: SessionExpiredNoticeProps) {
  const { isAuthenticated } = useAuth();
  const [showNotice, setShowNotice] = useState(false);

  useEffect(() => {
    // Check for session expiry indicators
    const checkSession = () => {
      const token = sessionStorage.getItem('auth_token');
      if (!token && isAuthenticated) {
        setShowNotice(true);
      }
    };

    // Check periodically
    const interval = setInterval(checkSession, 5000);
    checkSession(); // Initial check

    return () => clearInterval(interval);
  }, [isAuthenticated]);

  const handleReauth = () => {
    setShowNotice(false);
    sessionStorage.removeItem('auth_token');
    if (onReauth) {
      onReauth();
    } else {
      window.location.href = '/auth/login';
    }
  };

  if (!showNotice) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-gray-500 bg-opacity-75"
      role="alert"
      aria-live="assertive"
    >
      <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
        <div className="p-6">
          <div className="flex items-start">
            <div className="flex-shrink-0">
              <svg
                className="h-6 w-6 text-yellow-600"
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
            </div>
            <div className="ml-3 flex-1">
              <h3 className="text-lg font-medium text-gray-900">Session Expired</h3>
              <div className="mt-2">
                <p className="text-sm text-gray-500">
                  Your session has expired. Please sign in again to continue.
                </p>
              </div>
              <div className="mt-4">
                <button
                  onClick={handleReauth}
                  className="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-primary text-base font-medium text-white hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary sm:text-sm"
                >
                  Sign In Again
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

