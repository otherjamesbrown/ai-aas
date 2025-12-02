import { ReactNode } from 'react';

interface CardProps {
  children: ReactNode;
  className?: string;
  hover?: boolean;
}

/**
 * Card - Container for content sections
 */
export function Card({ children, className = '', hover = false }: CardProps) {
  return (
    <div
      className={`bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 ${
        hover ? 'hover:shadow-md transition-shadow' : ''
      } ${className}`}
    >
      {children}
    </div>
  );
}

interface CardHeaderProps {
  title: string;
  description?: string;
  actions?: ReactNode;
  className?: string;
}

/**
 * CardHeader - Header section for Card
 */
export function CardHeader({ title, description, actions, className = '' }: CardHeaderProps) {
  return (
    <div
      className={`flex items-start justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700 ${className}`}
    >
      <div>
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">{title}</h3>
        {description && (
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center space-x-2">{actions}</div>}
    </div>
  );
}

interface CardContentProps {
  children: ReactNode;
  className?: string;
  noPadding?: boolean;
}

/**
 * CardContent - Main content area for Card
 */
export function CardContent({ children, className = '', noPadding = false }: CardContentProps) {
  return (
    <div className={`${noPadding ? '' : 'px-6 py-4'} ${className}`}>
      {children}
    </div>
  );
}

interface CardFooterProps {
  children: ReactNode;
  className?: string;
}

/**
 * CardFooter - Footer section for Card
 */
export function CardFooter({ children, className = '' }: CardFooterProps) {
  return (
    <div
      className={`flex items-center justify-end px-6 py-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50 rounded-b-lg ${className}`}
    >
      {children}
    </div>
  );
}

interface StatsCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  icon?: ReactNode;
  trend?: {
    value: number;
    isPositive: boolean;
  };
  className?: string;
}

/**
 * StatsCard - Card for displaying statistics/metrics
 */
export function StatsCard({ title, value, subtitle, icon, trend, className = '' }: StatsCardProps) {
  return (
    <Card className={className}>
      <CardContent>
        <div className="flex items-start justify-between">
          <div>
            <p className="text-sm font-medium text-gray-500 dark:text-gray-400">{title}</p>
            <p className="mt-2 text-3xl font-bold text-gray-900 dark:text-gray-100">{value}</p>
            {subtitle && (
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{subtitle}</p>
            )}
            {trend && (
              <div className="flex items-center mt-2">
                <span
                  className={`text-sm font-medium ${
                    trend.isPositive ? 'text-green-600' : 'text-red-600'
                  }`}
                >
                  {trend.isPositive ? '↑' : '↓'} {Math.abs(trend.value)}%
                </span>
                <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">vs last period</span>
              </div>
            )}
          </div>
          {icon && (
            <div className="p-3 bg-primary/10 dark:bg-primary/20 rounded-lg text-primary">
              {icon}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
