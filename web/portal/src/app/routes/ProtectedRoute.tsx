import { ReactNode, useEffect } from 'react';
import { useNavigate, useLocation } from '@tanstack/react-router';
import { useAuth } from '@/providers/AuthProvider';
import { useRoutePermission, usePermissionGuard } from '@/features/access/hooks/usePermissionGuard';
import type { Permission } from '@/features/access/types';
import { LoadingSpinner } from '@/components/LoadingSpinner';

interface ProtectedRouteProps {
  children: ReactNode;
  permissions?: Permission | Permission[];
  requireAll?: boolean;
  featureFlag?: string;
  redirectTo?: string;
}

/**
 * Protected route wrapper that enforces permission checks
 * Redirects to access denied page if user lacks required permissions
 */
export function ProtectedRoute({
  children,
  permissions,
  requireAll = false,
  featureFlag,
  redirectTo = '/access-denied',
}: ProtectedRouteProps) {
  const { isAuthenticated, isLoading } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  // Use route-based permissions if no explicit permissions provided
  const routePermission = useRoutePermission(location.pathname);
  const explicitPermission = usePermissionGuard({
    permission: permissions || [],
    requireAll,
    featureFlag
  });

  const guard = permissions ? explicitPermission : routePermission;

  useEffect(() => {
    // Wait for auth to finish loading
    if (isLoading) return;

    // Redirect to login if not authenticated
    if (!isAuthenticated) {
      navigate({
        to: '/auth/login',
        search: { redirect: location.pathname },
      });
      return;
    }

    // Redirect to access denied if lacks permissions
    if (!guard.hasAccess) {
      navigate({
        to: redirectTo,
        search: (prev) => ({
          ...prev,
          from: location.pathname,
          reason: guard.reason || 'Insufficient permissions',
        }),
      });
    }
  }, [isAuthenticated, isLoading, guard.hasAccess, navigate, location.pathname, redirectTo, guard.reason]);

  // Show loading state while checking auth
  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <LoadingSpinner size="lg" aria-label="Checking permissions" />
          <p className="text-gray-600 mt-4">Checking permissions...</p>
        </div>
      </div>
    );
  }

  // Show access denied if not authenticated
  if (!isAuthenticated) {
    return null; // Will redirect in useEffect
  }

  // Show access denied if lacks permissions
  if (!guard.hasAccess) {
    return null; // Will redirect in useEffect
  }

  // User has access, render children
  return <>{children}</>;
}

/**
 * Higher-order component version for route definitions
 */
export function withProtectedRoute<P extends object>(
  Component: React.ComponentType<P>,
  options: Omit<ProtectedRouteProps, 'children'>
) {
  return function ProtectedComponent(props: P) {
    return (
      <ProtectedRoute {...options}>
        <Component {...props} />
      </ProtectedRoute>
    );
  };
}


