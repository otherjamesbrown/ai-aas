import { useQuery } from '@tanstack/react-query';
import { usageApi } from '../api';

interface StatusBadgeProps {
  className?: string;
}

/**
 * Status badge component showing last successful sync time
 */
export function StatusBadge({ className = '' }: StatusBadgeProps) {
  const { data: reportData } = useQuery({
    queryKey: ['usage', 'report', {}],
    queryFn: () => usageApi.getReport({}),
    select: (data) => ({
      lastSync: data.last_sync_at,
      syncStatus: data.sync_status,
      source: data.report.source,
    }),
    refetchInterval: 30 * 1000, // Refresh every 30 seconds
  });

  if (!reportData) {
    return null;
  }

  const { lastSync, syncStatus, source } = reportData;

  const getStatusColor = () => {
    if (syncStatus === 'error' || source === 'degraded') {
      return 'bg-red-100 text-red-800 border-red-200';
    }
    if (syncStatus === 'degraded') {
      return 'bg-yellow-100 text-yellow-800 border-yellow-200';
    }
    return 'bg-green-100 text-green-800 border-green-200';
  };

  const getStatusText = () => {
    if (syncStatus === 'error' || source === 'degraded') {
      return 'Sync Error';
    }
    if (syncStatus === 'degraded') {
      return 'Degraded';
    }
    return 'Synced';
  };

  const formatLastSync = (timestamp?: string) => {
    if (!timestamp) return 'Never';
    
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 1000 / 60);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours}h ago`;
    
    const diffDays = Math.floor(diffHours / 24);
    return `${diffDays}d ago`;
  };

  return (
    <div
      className={`inline-flex items-center px-3 py-1 rounded-full text-xs font-medium border ${getStatusColor()} ${className}`}
      title={lastSync ? `Last synced: ${new Date(lastSync).toLocaleString()}` : 'Never synced'}
    >
      <span className="mr-2">{getStatusText()}</span>
      {lastSync && (
        <span className="text-xs opacity-75">{formatLastSync(lastSync)}</span>
      )}
    </div>
  );
}

