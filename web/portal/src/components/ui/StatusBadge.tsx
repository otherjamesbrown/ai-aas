import { ReactNode } from 'react';

type StatusType = 'success' | 'warning' | 'error' | 'info' | 'neutral';

interface StatusBadgeProps {
  status: StatusType;
  children: ReactNode;
  size?: 'sm' | 'md';
  dot?: boolean;
}

const statusStyles: Record<StatusType, string> = {
  success: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  warning: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
  error: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  info: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  neutral: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
};

const dotStyles: Record<StatusType, string> = {
  success: 'bg-green-500',
  warning: 'bg-yellow-500',
  error: 'bg-red-500',
  info: 'bg-blue-500',
  neutral: 'bg-gray-500',
};

const sizeStyles = {
  sm: 'px-2 py-0.5 text-xs',
  md: 'px-2.5 py-0.5 text-sm',
};

/**
 * StatusBadge - Colored badge for status indication
 */
export function StatusBadge({ status, children, size = 'sm', dot = false }: StatusBadgeProps) {
  return (
    <span
      className={`inline-flex items-center font-medium rounded-full ${statusStyles[status]} ${sizeStyles[size]}`}
    >
      {dot && (
        <span className={`w-1.5 h-1.5 mr-1.5 rounded-full ${dotStyles[status]}`} />
      )}
      {children}
    </span>
  );
}

/**
 * StatusDot - Simple traffic light indicator
 */
interface StatusDotProps {
  status: StatusType;
  size?: 'sm' | 'md' | 'lg';
  pulse?: boolean;
  label?: string;
}

const dotSizeStyles = {
  sm: 'w-2 h-2',
  md: 'w-3 h-3',
  lg: 'w-4 h-4',
};

export function StatusDot({ status, size = 'md', pulse = false, label }: StatusDotProps) {
  return (
    <span className="inline-flex items-center">
      <span
        className={`rounded-full ${dotStyles[status]} ${dotSizeStyles[size]} ${pulse ? 'animate-pulse' : ''}`}
        title={label}
      />
      {label && (
        <span className="ml-2 text-sm text-gray-700 dark:text-gray-300">{label}</span>
      )}
    </span>
  );
}

/**
 * Map common status strings to StatusType
 */
export function getStatusType(status: string): StatusType {
  const lowerStatus = status.toLowerCase();

  if (['active', 'healthy', 'running', 'success', 'ready', 'enabled'].includes(lowerStatus)) {
    return 'success';
  }
  if (['warning', 'degraded', 'pending', 'processing'].includes(lowerStatus)) {
    return 'warning';
  }
  if (['error', 'failed', 'unhealthy', 'offline', 'revoked', 'disabled'].includes(lowerStatus)) {
    return 'error';
  }
  if (['info', 'new', 'created'].includes(lowerStatus)) {
    return 'info';
  }
  return 'neutral';
}
