import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { platformApi } from '../api/platform';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  Card,
  CardHeader,
  CardContent,
  DataTable,
  StatusBadge,
} from '@/components/ui';
import type { Column } from '@/components/ui';

interface SyncTarget {
  name: string;
  last_sync: string;
  status: string;
  message?: string;
}

/**
 * SyncPage - Platform sync operations
 *
 * CLI equivalents:
 * - ai-aas-cli sync status
 * - ai-aas-cli sync trigger
 */
export default function SyncPage() {
  const queryClient = useQueryClient();

  // Fetch sync status
  const { data: syncData, isLoading, refetch } = useQuery({
    queryKey: ['sync-status'],
    queryFn: () => platformApi.syncStatus(),
    refetchInterval: 5000, // Auto-refresh every 5 seconds
  });

  // Trigger sync mutation
  const triggerMutation = useMutation({
    mutationFn: (target?: string) => platformApi.syncTrigger(target),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sync-status'] });
    },
  });

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'synced':
      case 'success':
        return 'success';
      case 'syncing':
      case 'pending':
        return 'warning';
      case 'failed':
      case 'error':
        return 'error';
      default:
        return 'neutral';
    }
  };

  const columns: Column<SyncTarget>[] = [
    {
      key: 'name',
      header: 'Target',
      sortable: true,
      render: (t) => <span className="font-medium">{t.name}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      render: (t) => (
        <StatusBadge status={getStatusBadge(t.status)}>
          {t.status}
        </StatusBadge>
      ),
    },
    {
      key: 'last_sync',
      header: 'Last Sync',
      sortable: true,
      render: (t) => (
        <span className="text-sm text-gray-600 dark:text-gray-400">
          {t.last_sync ? new Date(t.last_sync).toLocaleString() : 'Never'}
        </span>
      ),
    },
    {
      key: 'message',
      header: 'Message',
      render: (t) => (
        <span className="text-sm text-gray-500 dark:text-gray-400">
          {t.message || '-'}
        </span>
      ),
    },
  ];

  const renderActions = (target: SyncTarget) => (
    <Button
      size="sm"
      variant="secondary"
      onClick={() => triggerMutation.mutate(target.name)}
      loading={triggerMutation.isPending}
    >
      Sync
    </Button>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="Platform Sync"
        description="Synchronize platform state across services"
        actions={
          <div className="flex items-center gap-2">
            <Button variant="secondary" onClick={() => refetch()}>
              Refresh Status
            </Button>
            <Button
              onClick={() => triggerMutation.mutate(undefined)}
              loading={triggerMutation.isPending}
            >
              Sync All
            </Button>
          </div>
        }
      />

      {/* Global Status */}
      <Card className="mb-6">
        <CardContent>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <div>
                <p className="text-sm text-gray-500 dark:text-gray-400">Overall Status</p>
                <div className="mt-1">
                  <StatusBadge
                    status={getStatusBadge(syncData?.status || 'unknown')}
                  >
                    {syncData?.status || 'Unknown'}
                  </StatusBadge>
                </div>
              </div>
              <div className="border-l border-gray-200 dark:border-gray-700 pl-4">
                <p className="text-sm text-gray-500 dark:text-gray-400">Last Sync</p>
                <p className="text-sm font-medium text-gray-900 dark:text-white mt-1">
                  {syncData?.last_sync
                    ? new Date(syncData.last_sync).toLocaleString()
                    : 'Never'}
                </p>
              </div>
            </div>
            {syncData?.status === 'syncing' && (
              <div className="flex items-center gap-2 text-amber-600">
                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-amber-600" />
                <span className="text-sm">Sync in progress...</span>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Sync Targets */}
      <Card>
        <CardHeader
          title="Sync Targets"
          actions={
            <span className="text-sm text-gray-500">
              {syncData?.targets?.length || 0} target{(syncData?.targets?.length || 0) !== 1 ? 's' : ''}
            </span>
          }
        />
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={syncData?.targets || []}
            keyExtractor={(t) => t.name}
            loading={isLoading}
            emptyMessage="No sync targets configured."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* Recent Sync Results */}
      {triggerMutation.isSuccess && triggerMutation.data && (
        <Card className="mt-6">
          <CardHeader title="Recent Sync Result" />
          <CardContent>
            <div className="flex items-center gap-3">
              <StatusBadge status="success">Triggered</StatusBadge>
              <span className="text-sm text-gray-600 dark:text-gray-400">
                Job ID: {triggerMutation.data.job_id}
              </span>
            </div>
            <p className="mt-2 text-sm text-gray-500">
              {triggerMutation.data.message}
            </p>
          </CardContent>
        </Card>
      )}

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># Check sync status</p>
            <p className="text-primary">ai-aas-cli sync status</p>
            <p className="text-gray-500 mt-3"># Trigger sync for all targets</p>
            <p className="text-primary">ai-aas-cli sync trigger</p>
            <p className="text-gray-500 mt-3"># Trigger sync for specific target</p>
            <p className="text-primary">ai-aas-cli sync trigger --target {'<name>'}</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
