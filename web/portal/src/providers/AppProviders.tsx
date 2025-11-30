import { ReactNode } from 'react';
import { QueryProvider } from '@/lib/query';
import { ToastProvider } from './ToastProvider';
import { AuthProvider } from './AuthProvider';

interface AppProvidersProps {
  children: ReactNode;
}

/**
 * AppProviders - Combined provider wrapper
 *
 * Flattens the provider hierarchy to maximum 4 levels as per spec requirements.
 * Combines QueryProvider and ToastProvider to reduce nesting.
 *
 * Provider Order:
 * 1. AuthProvider - Authentication state
 * 2. QueryProvider - TanStack Query for data fetching
 * 3. ToastProvider - Toast notifications
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
      <QueryProvider>
        <ToastProvider>
          {children}
        </ToastProvider>
      </QueryProvider>
    </AuthProvider>
  );
}
