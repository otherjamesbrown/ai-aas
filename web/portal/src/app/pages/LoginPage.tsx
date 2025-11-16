import { useState, useEffect } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { useAuth } from '@/providers/AuthProvider';
import { LoadingSpinner } from '@/components/LoadingSpinner';
import { useToast } from '@/providers/ToastProvider';

/**
 * Login page with OAuth2/OIDC and password-based authentication
 * Supports both OAuth2/OIDC flow and username/password login
 */
export default function LoginPage() {
  const navigate = useNavigate();
  // @ts-expect-error - TanStack Router type inference issue with optional search params
  const search = useSearch({ from: '/auth/login', strict: false }) as { redirect?: string };
  const { login, loginWithPassword, isAuthenticated, isLoading: authLoading } = useAuth();
  const { showError, showSuccess } = useToast();
  const [isLoggingIn, setIsLoggingIn] = useState(false);
  const [loginMethod, setLoginMethod] = useState<'oauth' | 'password'>('password');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [orgId, setOrgId] = useState('');

  useEffect(() => {
    // Redirect if already authenticated
    if (isAuthenticated && !authLoading) {
      const redirect = search?.redirect;
      navigate({
        to: redirect || '/',
      });
    }
  }, [isAuthenticated, authLoading, navigate, search]);

  const handleOAuthLogin = async () => {
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

  const handlePasswordLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    console.log('Login form submitted', { email: email.trim(), hasPassword: !!password, orgId });
    
    if (!email.trim() || !password) {
      console.warn('Validation failed: missing email or password');
      showError('Validation failed', 'Please enter both email and password');
      return;
    }

    setIsLoggingIn(true);
    console.log('Starting login process...');
    try {
      await loginWithPassword(email, password, orgId || undefined);
      console.log('Login successful');
      showSuccess('Login successful', 'Redirecting...');
      // Navigation will happen via useEffect
    } catch (error) {
      console.error('Login error caught in LoginPage:', error);
      
      // Extract error message with fallbacks
      let errorMessage = 'Invalid email or password';
      if (error instanceof Error) {
        errorMessage = error.message || 'An unexpected error occurred';
      } else if (typeof error === 'string') {
        errorMessage = error;
      } else {
        errorMessage = 'An unexpected error occurred during login';
      }
      
      // Always show error toast
      console.log('Attempting to show error toast:', { title: 'Login failed', message: errorMessage });
      try {
        showError('Login failed', errorMessage);
        console.log('showError called successfully');
      } catch (toastError) {
        // Fallback if toast fails - use alert
        console.error('Failed to show toast, using alert:', toastError);
        alert(`Login failed: ${errorMessage}`);
      }
      
      // Also log to console for debugging
      console.error('Full error details:', {
        error,
        errorType: typeof error,
        errorConstructor: error?.constructor?.name,
        stack: error instanceof Error ? error.stack : undefined,
      });
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

        {/* Login method selector */}
        <div className="mb-6">
          <div className="flex rounded-lg border border-gray-300 p-1">
            <button
              type="button"
              onClick={() => setLoginMethod('password')}
              className={`flex-1 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                loginMethod === 'password'
                  ? 'bg-primary text-white'
                  : 'text-gray-700 hover:bg-gray-100'
              }`}
            >
              Email & Password
            </button>
            <button
              type="button"
              onClick={() => setLoginMethod('oauth')}
              className={`flex-1 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                loginMethod === 'oauth'
                  ? 'bg-primary text-white'
                  : 'text-gray-700 hover:bg-gray-100'
              }`}
            >
              OAuth
            </button>
          </div>
        </div>

        <div className="space-y-4">
          {loginMethod === 'password' ? (
            <form onSubmit={handlePasswordLogin} className="space-y-4">
              <div>
                <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1">
                  Email
                </label>
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  disabled={isLoggingIn}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:bg-gray-100 disabled:cursor-not-allowed"
                  placeholder="admin@example.com"
                  autoComplete="email"
                />
              </div>

              <div>
                <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1">
                  Password
                </label>
                <input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  disabled={isLoggingIn}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:bg-gray-100 disabled:cursor-not-allowed"
                  placeholder="Enter your password"
                  autoComplete="current-password"
                />
              </div>

              <div>
                <label htmlFor="orgId" className="block text-sm font-medium text-gray-700 mb-1">
                  Organization ID (optional)
                </label>
                <input
                  id="orgId"
                  type="text"
                  value={orgId}
                  onChange={(e) => setOrgId(e.target.value)}
                  disabled={isLoggingIn}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:bg-gray-100 disabled:cursor-not-allowed"
                  placeholder="Leave empty to use default"
                />
                <p className="mt-1 text-xs text-gray-500">
                  Optional: Specify organization slug or ID if you belong to multiple organizations
                </p>
              </div>

              <button
                type="submit"
                disabled={isLoggingIn}
                className="w-full px-4 py-3 bg-primary text-white rounded-lg font-semibold hover:bg-primary-dark disabled:opacity-50 disabled:cursor-not-allowed transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
              >
                {isLoggingIn ? (
                  <span className="flex items-center justify-center">
                    <LoadingSpinner size="sm" className="mr-2" />
                    Signing in...
                  </span>
                ) : (
                  'Sign In'
                )}
              </button>
            </form>
          ) : (
            <>
              <button
                onClick={handleOAuthLogin}
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

              <div className="text-center text-sm text-gray-500">
                <p className="mb-2">Additional authentication methods:</p>
                <ul className="list-disc list-inside space-y-1 text-left">
                  <li>SAML SSO (coming soon)</li>
                  <li>MFA (if enabled for your organization)</li>
                </ul>
              </div>
            </>
          )}
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

