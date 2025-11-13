import { ReactNode } from 'react';
import { useReadOnlyGuard } from '../hooks/useImpersonationGuard';

interface ReadOnlyGuardProps {
  action: string;
  children: ReactNode;
  fallback?: ReactNode;
}

/**
 * Component that blocks children when in read-only impersonation mode
 * Shows fallback or disabled state with explanation
 */
export function ReadOnlyGuard({ action, children, fallback }: ReadOnlyGuardProps) {
  const { isBlocked, message } = useReadOnlyGuard(action);

  if (!isBlocked) {
    return <>{children}</>;
  }

  if (fallback) {
    return <>{fallback}</>;
  }

  // Default: show disabled version with tooltip
  return (
    <div className="relative group">
      <div className="pointer-events-none opacity-50">{children}</div>
      <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 px-3 py-2 bg-gray-900 text-white text-xs rounded-md opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-50 whitespace-nowrap max-w-xs">
        {message}
        <div className="absolute top-full left-1/2 transform -translate-x-1/2 -mt-1">
          <div className="border-4 border-transparent border-t-gray-900" />
        </div>
      </div>
    </div>
  );
}

