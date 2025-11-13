import { useSearch } from '@tanstack/react-router';
import { Link } from '@tanstack/react-router';
import { useAuth } from '@/providers/AuthProvider';

interface AccessDeniedSearch {
  from?: string;
  reason?: string;
}

/**
 * Access denied page with support links
 * Shows when user attempts to access a resource they don't have permission for
 */
export default function AccessDeniedPage() {
  const { user } = useAuth();
  const search = useSearch({ from: '/' }) as AccessDeniedSearch;

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full text-center">
        <div className="mb-6">
          <svg
            className="mx-auto h-16 w-16 text-red-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
        </div>

        <h1 className="text-4xl font-bold text-gray-900 mb-4">Access Denied</h1>
        
        <p className="text-lg text-gray-600 mb-6">
          You don't have permission to access this resource.
        </p>

        {search.reason && (
          <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mb-6">
            <p className="text-sm text-yellow-800">
              <strong>Reason:</strong> {search.reason}
            </p>
          </div>
        )}

        {user && (
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-6">
            <p className="text-sm text-blue-800">
              <strong>Your role:</strong> {user.roles.join(', ')}
            </p>
            <p className="text-sm text-blue-700 mt-2">
              If you believe this is an error, please contact your organization administrator
              or support team to request additional permissions.
            </p>
          </div>
        )}

        <div className="bg-gray-50 border border-gray-200 rounded-lg p-6 mb-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-3">Need Help?</h2>
          <ul className="text-left text-sm text-gray-700 space-y-2">
            <li>
              <strong>Contact Support:</strong>{' '}
              <a
                href="mailto:support@ai-aas.com"
                className="text-primary hover:underline"
              >
                support@ai-aas.com
              </a>
            </li>
            <li>
              <strong>Documentation:</strong>{' '}
              <a
                href="/docs/access-control"
                className="text-primary hover:underline"
                target="_blank"
                rel="noopener noreferrer"
              >
                Access Control Guide
              </a>
            </li>
            <li>
              <strong>Request Access:</strong> Contact your organization administrator
              to request the necessary permissions.
            </li>
          </ul>
        </div>

        <div className="space-x-4">
          <Link
            to="/"
            className="inline-block bg-primary text-white px-6 py-2 rounded-md hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
          >
            Return to Home
          </Link>
          {search.from && search.from !== '/' && (
            <button
              onClick={() => window.history.back()}
              className="inline-block bg-white text-gray-700 px-6 py-2 rounded-md border border-gray-300 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
            >
              Go Back
            </button>
          )}
        </div>
      </div>
    </div>
  );
}


