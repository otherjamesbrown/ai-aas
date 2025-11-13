import { ReactNode } from 'react';
import { Link } from '@tanstack/react-router';
import { useAuth } from '@/providers/AuthProvider';
import { useImpersonationGuard } from '@/features/support/hooks/useImpersonationGuard';
import { ImpersonationBanner } from '@/features/support/components/ImpersonationBanner';
import { RoleAwareNav } from './components/RoleAwareNav';

interface LayoutProps {
  children: ReactNode;
}

/**
 * Shared layout component with app bar, navigation, and notification slot
 * Includes impersonation banner when support engineer is impersonating
 */
export default function Layout({ children }: LayoutProps) {
  const { user, isAuthenticated, logout } = useAuth();
  const { session, isImpersonating } = useImpersonationGuard();

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Skip to content link for accessibility */}
      <a href="#main-content" className="skip-to-content">
        Skip to main content
      </a>

      {/* Impersonation banner - shown when support is impersonating */}
      {isImpersonating && session && (
        <ImpersonationBanner
          session={session}
          onRevoke={() => {
            // Session will be cleared by the guard hook
            window.location.reload();
          }}
        />
      )}

      {/* App Bar */}
      <header className="bg-white shadow-sm border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between items-center h-16">
            {/* Logo */}
            <div className="flex items-center">
              <Link to="/" className="flex items-center space-x-2">
                <span className="text-2xl font-bold text-primary">AI-AAS</span>
                <span className="text-sm text-gray-600">Portal</span>
              </Link>
            </div>

            {/* Navigation */}
            <RoleAwareNav />

            {/* User menu */}
            <div className="flex items-center space-x-4">
              {isAuthenticated && user ? (
                <>
                  <span className="text-sm text-gray-700">{user.email}</span>
                  {isImpersonating && (
                    <span className="text-xs px-2 py-1 bg-yellow-100 text-yellow-800 rounded">
                      Impersonating
                    </span>
                  )}
                  <button
                    onClick={() => logout()}
                    className="text-sm text-gray-700 hover:text-primary focus-visible-ring px-3 py-2 rounded-md"
                  >
                    Logout
                  </button>
                </>
              ) : (
                <Link
                  to="/auth/login"
                  search={{ redirect: undefined }}
                  className="text-sm text-primary hover:text-primary-dark focus-visible-ring px-3 py-2 rounded-md"
                >
                  Login
                </Link>
              )}
            </div>
          </div>
        </div>
      </header>

      {/* Toast notifications are handled by ToastProvider */}

      {/* Main content */}
      <main id="main-content" className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {children}
      </main>

      {/* Footer */}
      <footer className="bg-white border-t border-gray-200 mt-auto">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <p className="text-center text-sm text-gray-600">
            © {new Date().getFullYear()} AI-AAS Portal. All rights reserved.
          </p>
        </div>
      </footer>
    </div>
  );
}

