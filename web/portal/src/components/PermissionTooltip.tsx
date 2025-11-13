import { ReactNode } from 'react';
import { usePermissionGuard } from '@/features/access/hooks/usePermissionGuard';
import type { Permission } from '@/features/access/types';

interface PermissionTooltipProps {
  permission: Permission | Permission[];
  featureFlag?: string;
  children: ReactNode;
  tooltip?: string; // Custom tooltip message
}

/**
 * Component that wraps UI elements and shows a tooltip explaining why they're disabled
 * when the user lacks required permissions
 */
export function PermissionTooltip({
  permission,
  featureFlag,
  children,
  tooltip,
}: PermissionTooltipProps) {
  const { hasAccess, reason, missingFeatureFlag } = usePermissionGuard({
    permission,
    featureFlag,
  });

  // If user has access, render children normally
  if (hasAccess) {
    return <>{children}</>;
  }

  // Build tooltip message
  let tooltipMessage = tooltip;
  if (!tooltipMessage) {
    if (missingFeatureFlag) {
      tooltipMessage = `Feature "${missingFeatureFlag}" is not enabled for your organization.`;
    } else if (reason) {
      tooltipMessage = reason;
    } else {
      tooltipMessage = 'You do not have permission to perform this action.';
    }
  }

  return (
    <div className="relative group">
      <div className="pointer-events-none opacity-50">{children}</div>
      <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 px-3 py-2 bg-gray-900 text-white text-xs rounded-md opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-50 whitespace-nowrap">
        {tooltipMessage}
        <div className="absolute top-full left-1/2 transform -translate-x-1/2 -mt-1">
          <div className="border-4 border-transparent border-t-gray-900" />
        </div>
      </div>
    </div>
  );
}

/**
 * Hook version for programmatic access to permission state
 */
export function usePermissionTooltip(permission: Permission | Permission[], featureFlag?: string) {
  const guard = usePermissionGuard({ permission, featureFlag });
  
  return {
    ...guard,
    tooltipMessage: guard.missingFeatureFlag
      ? `Feature "${guard.missingFeatureFlag}" is not enabled.`
      : guard.reason || 'You do not have permission to perform this action.',
  };
}


