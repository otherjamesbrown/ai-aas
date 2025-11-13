import { useState, useEffect } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { useAuth } from '@/providers/AuthProvider';
import { LoadingSpinner } from '@/components/LoadingSpinner';
import { useToast } from '@/providers/ToastProvider';

/**
 * Login page with OAuth2/OIDC integration
 * Supports OAuth2/OIDC flow with identity provider
 */
export default function LoginPage() {
  const navigate = useNavigate();
  // @ts-expect-error - TanStack Router type inference issue with optional search params
  const search = useSearch({ from: '/auth/login', strict: false }) as { redirect?: string };
  const { login, isAuthenticated, isLoading: authLoading } = useAuth();
  const { showError, showSuccess } = useToast();
  const [isLoggingIn, setIsLoggingIn] = useState(false);

  useEffect(() => {
    // Redirect if already authenticated
    if (isAuthenticated && !authLoading) {
      const redirect = search?.redirect;
      navigate({
        to: redirect || '/',
      });
    }
  }, [isAuthenticated, authLoading, navigate, search]);

  const handleLogin = async () => {
    setIsLoggingIn(true);
    try {
      await login();
      showSuccess('Login successful', 'Redirecting...');
      // Navigation will happen via useEffect
    } catch (error) {
      showError(
        'Login failed',
        error instanceof Error ? error.message : 'An error occurred during login'
      );
    } finally {
      setIsLoggingIn(false);
    }
  };

  if (authLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <LoadingSpinner size="lg" aria-label="Loading authentication" />
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 px-4">
      <div className="max-w-md w-full bg-white rounded-lg shadow-lg p-8">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-gray-900 mb-2">
            Sign In
          </h1>
          <p className="text-gray-600">
            Sign in to access the AI-AAS Portal
          </p>
        </div>

        <div className="space-y-4">
          {/* TODO: Replace with actual OAuth2/OIDC provider buttons */}
          <button
            onClick={handleLogin}
            disabled={isLoggingIn}
            className="w-full px-4 py-3 bg-primary text-white rounded-lg font-semibold hover:bg-primary-dark disabled:opacity-50 disabled:cursor-not-allowed transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
          >
            {isLoggingIn ? (
              <span className="flex items-center justify-center">
                <LoadingSpinner size="sm" className="mr-2" />
                Signing in...
              </span>
            ) : (
              'Sign In with OAuth'
            )}
          </button>

          <div className="relative my-6">
            <div className="absolute inset-0 flex items-center">
              <div className="w-full border-t border-gray-300" />
            </div>
            <div className="relative flex justify-center text-sm">
              <span className="px-2 bg-white text-gray-500">
                Or continue with
              </span>
            </div>
          </div>

          {/* Placeholder for additional auth methods */}
          <div className="text-center text-sm text-gray-500">
            <p className="mb-2">Additional authentication methods:</p>
            <ul className="list-disc list-inside space-y-1 text-left">
              <li>SAML SSO (coming soon)</li>
              <li>MFA (if enabled for your organization)</li>
            </ul>
          </div>
        </div>

        <div className="mt-8 pt-6 border-t border-gray-200">
          <p className="text-xs text-center text-gray-500">
            By signing in, you agree to our Terms of Service and Privacy Policy.
          </p>
        </div>
      </div>
    </div>
  );
}

