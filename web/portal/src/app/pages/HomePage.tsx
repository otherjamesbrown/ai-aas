import { Link } from '@tanstack/react-router';
import { useAuth } from '@/providers/AuthProvider';
import { LoadingSpinner } from '@/components/LoadingSpinner';

/**
 * Home/Landing page
 * Shows different content based on authentication status
 */
export default function HomePage() {
  const { isAuthenticated, isLoading, user } = useAuth();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <LoadingSpinner size="lg" aria-label="Loading" />
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      {/* Hero Section */}
      <div className="bg-gradient-to-br from-primary to-primary-dark text-white py-20">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center">
            <h1 className="text-4xl md:text-5xl font-bold mb-4">
              Welcome to AI-AAS Portal
            </h1>
            <p className="text-xl md:text-2xl mb-8 text-primary-100">
              Manage your organization, members, budgets, and API keys
            </p>
            {!isAuthenticated ? (
              <Link
                to="/auth/login"
                search={{ redirect: undefined }}
                className="inline-block px-6 py-3 bg-white text-primary rounded-lg font-semibold hover:bg-gray-100 transition-colors"
              >
                Get Started
              </Link>
            ) : (
              <div className="space-y-4">
                <p className="text-lg">Welcome back, {user?.email}</p>
                <div className="flex justify-center gap-4">
                  <Link
                    to="/admin/organization"
                    search={{}}
                    className="inline-block px-6 py-3 bg-white text-primary rounded-lg font-semibold hover:bg-gray-100 transition-colors"
                  >
                    Go to Dashboard
                  </Link>
                  <Link
                    to="/usage"
                    search={{}}
                    className="inline-block px-6 py-3 bg-white bg-opacity-20 text-white border-2 border-white rounded-lg font-semibold hover:bg-opacity-30 transition-colors"
                  >
                    View Usage
                  </Link>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Features Section */}
      <div className="py-16 bg-gray-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <h2 className="text-3xl font-bold text-center mb-12 text-gray-900">
            Features
          </h2>
          <div className="grid md:grid-cols-3 gap-8">
            <div className="bg-white p-6 rounded-lg shadow-sm">
              <h3 className="text-xl font-semibold mb-3 text-gray-900">
                Organization Management
              </h3>
              <p className="text-gray-600">
                Manage your organization settings, members, and roles with ease.
              </p>
            </div>
            <div className="bg-white p-6 rounded-lg shadow-sm">
              <h3 className="text-xl font-semibold mb-3 text-gray-900">
                Budget Controls
              </h3>
              <p className="text-gray-600">
                Set spending limits and alerts to keep your costs under control.
              </p>
            </div>
            <div className="bg-white p-6 rounded-lg shadow-sm">
              <h3 className="text-xl font-semibold mb-3 text-gray-900">
                Usage Insights
              </h3>
              <p className="text-gray-600">
                View detailed usage reports and analytics for better decision-making.
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Quick Links */}
      {isAuthenticated && (
        <div className="py-16 bg-white">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <h2 className="text-3xl font-bold text-center mb-12 text-gray-900">
              Quick Links
            </h2>
            <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-6">
              <Link
                to="/admin/organization"
                search={{}}
                className="block p-6 border-2 border-gray-200 rounded-lg hover:border-primary hover:shadow-md transition-all"
              >
                <h3 className="font-semibold text-gray-900 mb-2">
                  Organization Settings
                </h3>
                <p className="text-sm text-gray-600">
                  Manage organization profile and settings
                </p>
              </Link>
              <Link
                to="/admin/members"
                search={{}}
                className="block p-6 border-2 border-gray-200 rounded-lg hover:border-primary hover:shadow-md transition-all"
              >
                <h3 className="font-semibold text-gray-900 mb-2">
                  Member Management
                </h3>
                <p className="text-sm text-gray-600">
                  Invite and manage team members
                </p>
              </Link>
              <Link
                to="/admin/budgets"
                search={{}}
                className="block p-6 border-2 border-gray-200 rounded-lg hover:border-primary hover:shadow-md transition-all"
              >
                <h3 className="font-semibold text-gray-900 mb-2">
                  Budget Controls
                </h3>
                <p className="text-sm text-gray-600">
                  Configure spending limits and alerts
                </p>
              </Link>
              <Link
                to="/usage"
                search={{}}
                className="block p-6 border-2 border-gray-200 rounded-lg hover:border-primary hover:shadow-md transition-all"
              >
                <h3 className="font-semibold text-gray-900 mb-2">
                  Usage Dashboard
                </h3>
                <p className="text-sm text-gray-600">
                  View usage insights and reports
                </p>
              </Link>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

