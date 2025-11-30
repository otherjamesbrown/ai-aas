import { ReactNode } from 'react';

interface ContentAreaProps {
  children: ReactNode;
  className?: string;
  /** Maximum width constraint */
  maxWidth?: 'sm' | 'md' | 'lg' | 'xl' | '2xl' | 'full';
  /** Add padding to content area */
  padding?: boolean;
}

const maxWidthClasses = {
  sm: 'max-w-screen-sm',
  md: 'max-w-screen-md',
  lg: 'max-w-screen-lg',
  xl: 'max-w-screen-xl',
  '2xl': 'max-w-screen-2xl',
  full: 'max-w-full',
};

/**
 * ContentArea - Main content wrapper with proper spacing and padding
 *
 * Used within AdminLayout to provide consistent content spacing.
 */
export function ContentArea({
  children,
  className = '',
  maxWidth = 'full',
  padding = true,
}: ContentAreaProps) {
  return (
    <div
      className={`
        ${maxWidthClasses[maxWidth]}
        ${padding ? 'p-6' : ''}
        ${className}
      `}
    >
      {children}
    </div>
  );
}
