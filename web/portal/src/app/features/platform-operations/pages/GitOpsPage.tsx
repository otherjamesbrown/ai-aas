import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { platformApi, ArgoApplication } from '../api/platform';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  Card,
  CardContent,
  StatusBadge,
  Modal,
} from '@/components/ui';

/**
 * GitOpsPage - Manage ArgoCD applications
 *
 * CLI equivalents:
 * - ai-aas-cli gitops list
 * - ai-aas-cli gitops sync
 * - ai-aas-cli gitops refresh
 */
export default function GitOpsPage() {
  const queryClient = useQueryClient();
  const [showDetailsModal, setShowDetailsModal] = useState(false);
  const [selectedApp, setSelectedApp] = useState<ArgoApplication | null>(null);
  const [syncingApp, setSyncingApp] = useState<string | null>(null);

  // Fetch applications
  const { data: applications = [], isLoading, refetch } = useQuery({
    queryKey: ['gitops-applications'],
    queryFn: () => platformApi.gitopsList(),
  });

  // Sync mutation
  const syncMutation = useMutation({
    mutationFn: (name: string) => platformApi.gitopsSync(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['gitops-applications'] });
      setSyncingApp(null);
    },
    onError: () => {
      setSyncingApp(null);
    },
  });

  // Refresh mutation
  const refreshMutation = useMutation({
    mutationFn: (name: string) => platformApi.gitopsRefresh(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['gitops-applications'] });
    },
  });

  const handleSync = (name: string) => {
    setSyncingApp(name);
    syncMutation.mutate(name);
  };

  const getSyncStatusBadge = (status: string) => {
    switch (status) {
      case 'Synced':
        return 'success';
      case 'OutOfSync':
        return 'warning';
      default:
        return 'neutral';
    }
  };

  const getHealthStatusBadge = (status: string) => {
    switch (status) {
      case 'Healthy':
        return 'success';
      case 'Degraded':
        return 'error';
      case 'Progressing':
        return 'warning';
      case 'Missing':
        return 'error';
      default:
        return 'neutral';
    }
  };

  return (
    <AdminLayout>
      <PageHeader
        title="GitOps Applications"
        description="Manage ArgoCD application deployments"
        actions={
          <Button
            variant="secondary"
            onClick={() => refetch()}
            leftIcon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            }
          >
            Refresh
          </Button>
        }
      />

      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
        </div>
      ) : applications.length === 0 ? (
        <Card>
          <CardContent>
            <div className="text-center py-12">
              <svg
                className="mx-auto h-12 w-12 text-gray-400"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1}
                  d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                />
              </svg>
              <p className="mt-4 text-sm text-gray-500 dark:text-gray-400">
                No ArgoCD applications found.
              </p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {applications.map((app) => (
            <Card key={app.name}>
              <CardContent>
                <div className="flex items-start justify-between mb-4">
                  <div>
                    <h3 className="font-semibold text-gray-900 dark:text-white">
                      {app.name}
                    </h3>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                      {app.namespace}
                    </p>
                  </div>
                  <div className="flex flex-col items-end space-y-1">
                    <StatusBadge status={getSyncStatusBadge(app.sync_status)}>
                      {app.sync_status}
                    </StatusBadge>
                    <StatusBadge status={getHealthStatusBadge(app.health_status)}>
                      {app.health_status}
                    </StatusBadge>
                  </div>
                </div>

                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-gray-500 dark:text-gray-400">Project:</span>
                    <span>{app.project}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-500 dark:text-gray-400">Path:</span>
                    <span className="font-mono text-xs truncate max-w-[150px]" title={app.source.path}>
                      {app.source.path}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-500 dark:text-gray-400">Branch:</span>
                    <span>{app.source.target_revision}</span>
                  </div>
                  {app.last_sync_time && (
                    <div className="flex justify-between">
                      <span className="text-gray-500 dark:text-gray-400">Last Sync:</span>
                      <span>{new Date(app.last_sync_time).toLocaleString()}</span>
                    </div>
                  )}
                </div>

                <div className="flex items-center justify-end space-x-2 mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => {
                      setSelectedApp(app);
                      setShowDetailsModal(true);
                    }}
                  >
                    Details
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => refreshMutation.mutate(app.name)}
                    loading={refreshMutation.isPending}
                  >
                    Refresh
                  </Button>
                  <Button
                    size="sm"
                    onClick={() => handleSync(app.name)}
                    loading={syncingApp === app.name}
                    disabled={app.sync_status === 'Synced'}
                  >
                    Sync
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Details Modal */}
      <Modal
        isOpen={showDetailsModal}
        onClose={() => {
          setShowDetailsModal(false);
          setSelectedApp(null);
        }}
        title={selectedApp?.name || 'Application Details'}
        size="lg"
      >
        {selectedApp && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Name</label>
                <p className="font-medium">{selectedApp.name}</p>
              </div>
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Namespace</label>
                <p>{selectedApp.namespace}</p>
              </div>
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Project</label>
                <p>{selectedApp.project}</p>
              </div>
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Sync Status</label>
                <p>
                  <StatusBadge status={getSyncStatusBadge(selectedApp.sync_status)}>
                    {selectedApp.sync_status}
                  </StatusBadge>
                </p>
              </div>
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Health Status</label>
                <p>
                  <StatusBadge status={getHealthStatusBadge(selectedApp.health_status)}>
                    {selectedApp.health_status}
                  </StatusBadge>
                </p>
              </div>
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Last Sync</label>
                <p>
                  {selectedApp.last_sync_time
                    ? new Date(selectedApp.last_sync_time).toLocaleString()
                    : 'Never'}
                </p>
              </div>
            </div>

            <div>
              <label className="text-xs text-gray-500 dark:text-gray-400">Source</label>
              <div className="mt-1 p-3 bg-gray-50 dark:bg-gray-800 rounded-md font-mono text-sm">
                <p><span className="text-gray-500">Repo:</span> {selectedApp.source.repo_url}</p>
                <p><span className="text-gray-500">Path:</span> {selectedApp.source.path}</p>
                <p><span className="text-gray-500">Branch:</span> {selectedApp.source.target_revision}</p>
              </div>
            </div>

            <div className="pt-4 border-t border-gray-200 dark:border-gray-700">
              <h4 className="text-sm font-medium mb-2">CLI Commands</h4>
              <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-3 space-y-1 font-mono text-xs">
                <p># View application details</p>
                <p className="text-primary">ai-aas-cli gitops get {selectedApp.name}</p>
                <p className="mt-2"># Sync application</p>
                <p className="text-primary">ai-aas-cli gitops sync {selectedApp.name}</p>
                <p className="mt-2"># Refresh application</p>
                <p className="text-primary">ai-aas-cli gitops refresh {selectedApp.name}</p>
              </div>
            </div>
          </div>
        )}
      </Modal>
    </AdminLayout>
  );
}
