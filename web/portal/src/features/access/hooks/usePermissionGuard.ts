import { useMemo } from 'react';
import { useAuth } from '@/providers/AuthProvider';
import { useFeatureFlags } from '@/providers/FeatureFlagProvider';
import { hasPermission, ROUTE_PERMISSIONS, type Permission } from '../types';
import type { MemberRole } from '../../admin/types';

interface UsePermissionGuardOptions {
  permission: Permission | Permission[];
  requireAll?: boolean; // If true, requires ALL permissions; if false, requires ANY
  featureFlag?: string; // Optional feature flag that must be enabled
}

interface PermissionGuardResult {
  hasAccess: boolean;
  missingPermissions: Permission[];
  missingFeatureFlag?: string;
  reason?: string;
}

/**
 * Hook to check if the current user has required permissions
 * Also checks feature flags if specified
 */
export function usePermissionGuard(options: UsePermissionGuardOptions): PermissionGuardResult {
  const { user } = useAuth();
  const { isEnabled } = useFeatureFlags();

  const permissions = Array.isArray(options.permission) 
    ? options.permission 
    : [options.permission];
  const requireAll = options.requireAll ?? false;

  return useMemo(() => {
    if (!user) {
      return {
        hasAccess: false,
        missingPermissions: permissions,
        reason: 'Not authenticated',
      };
    }

    // Check feature flag if specified
    if (options.featureFlag && !isEnabled(options.featureFlag)) {
      return {
        hasAccess: false,
        missingPermissions: [],
        missingFeatureFlag: options.featureFlag,
        reason: `Feature flag "${options.featureFlag}" is not enabled`,
      };
    }

    // Check permissions
    const userPermissions = permissions.filter((permission) =>
      hasPermission(user.roles[0] as MemberRole, permission, user.scopes)
    );

    const missingPermissions = permissions.filter(
      (permission) => !userPermissions.includes(permission)
    );

    let hasAccess: boolean;
    if (requireAll) {
      // Require ALL permissions
      hasAccess = missingPermissions.length === 0;
    } else {
      // Require ANY permission
      hasAccess = userPermissions.length > 0;
    }

    return {
      hasAccess,
      missingPermissions,
      reason: hasAccess 
        ? undefined 
        : `Missing required permission${missingPermissions.length > 1 ? 's' : ''}: ${missingPermissions.join(', ')}`,
    };
  }, [user, permissions, requireAll, options.featureFlag, isEnabled]);
}

/**
 * Hook to check if user can access a specific route
 */
export function useRoutePermission(route: string): PermissionGuardResult {
  const routePermissions = ROUTE_PERMISSIONS[route] || [];

  return usePermissionGuard({
    permission: routePermissions,
    requireAll: false, // User needs ANY of the route permissions
  });
}

