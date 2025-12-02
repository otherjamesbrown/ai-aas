import { ReactNode } from 'react';
import { QueryProvider } from '@/lib/query';
import { ToastProvider } from './ToastProvider';
import { AuthProvider, useAuth } from './AuthProvider';
import { FeatureFlagProvider } from './FeatureFlagProvider';

interface AppProvidersProps {
  children: ReactNode;
}

/**
 * Inner providers that need auth context
 */
function AuthenticatedProviders({ children }: { children: ReactNode }) {
  const { user, isAuthenticated } = useAuth();

  return (
    <FeatureFlagProvider
      userId={user?.id}
      organizationId={user?.organizationId}
      isAuthenticated={isAuthenticated}
    >
      <QueryProvider>
        <ToastProvider>
          {children}
        </ToastProvider>
      </QueryProvider>
    </FeatureFlagProvider>
  );
}

/**
 * AppProviders - Combined provider wrapper
 *
 * Flattens the provider hierarchy to maximum 4 levels as per spec requirements.
 * Combines QueryProvider and ToastProvider to reduce nesting.
 *
 * Provider Order:
 * 1. AuthProvider - Authentication state
 * 2. FeatureFlagProvider - Feature flags (needs auth context)
 * 3. QueryProvider - TanStack Query for data fetching
 * 4. ToastProvider - Toast notifications
 *
 * Usage:
 * ```tsx
 * <AppProviders>
 *   <RouterProvider router={router} />
 * </AppProviders>
 * ```
 */
export function AppProviders({ children }: AppProvidersProps) {
  return (
    <AuthProvider>
      <AuthenticatedProviders>
        {children}
      </AuthenticatedProviders>
    </AuthProvider>
  );
}
