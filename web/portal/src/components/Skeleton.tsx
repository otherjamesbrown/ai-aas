interface SkeletonProps {
  className?: string;
  lines?: number;
  width?: string;
  height?: string;
}

/**
 * Skeleton loader component for content placeholders
 */
export function Skeleton({ className = '', lines = 1, width, height }: SkeletonProps) {
  const baseClasses = 'animate-pulse bg-gray-200 rounded';
  const style: React.CSSProperties = {};
  
  if (width) style.width = width;
  if (height) style.height = height || '1rem';

  if (lines === 1) {
    return (
      <div className={`${baseClasses} ${className}`} style={style} aria-hidden="true" />
    );
  }

  return (
    <div className={`space-y-2 ${className}`} aria-hidden="true">
      {Array.from({ length: lines }).map((_, i) => (
        <div
          key={i}
          className={baseClasses}
          style={{
            ...style,
            width: i === lines - 1 && lines > 1 ? '75%' : width || '100%',
          }}
        />
      ))}
    </div>
  );
}

/**
 * Table skeleton loader
 */
export function TableSkeleton({ rows = 5, columns = 4 }: { rows?: number; columns?: number }) {
  return (
    <div className="space-y-3" aria-hidden="true">
      {/* Header */}
      <div className="flex space-x-4">
        {Array.from({ length: columns }).map((_, i) => (
          <Skeleton key={i} width="25%" height="1.5rem" />
        ))}
      </div>
      {/* Rows */}
      {Array.from({ length: rows }).map((_, rowIndex) => (
        <div key={rowIndex} className="flex space-x-4">
          {Array.from({ length: columns }).map((_, colIndex) => (
            <Skeleton key={colIndex} width="25%" height="1rem" />
          ))}
        </div>
      ))}
    </div>
  );
}

/**
 * Card skeleton loader
 */
export function CardSkeleton() {
  return (
    <div className="bg-white shadow rounded-lg p-6 space-y-4" aria-hidden="true">
      <Skeleton width="60%" height="1.5rem" />
      <Skeleton lines={3} />
      <div className="flex space-x-2">
        <Skeleton width="100px" height="2.5rem" />
        <Skeleton width="100px" height="2.5rem" />
      </div>
    </div>
  );
}

