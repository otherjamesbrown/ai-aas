import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { modelsApi, CacheEntry } from '../api/models';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  DataTable,
  StatusBadge,
  Card,
  CardContent,
  IconButton,
  ConfirmModal,
} from '@/components/ui';
import type { Column } from '@/components/ui';

/**
 * Format bytes to human readable string
 */
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

/**
 * ModelCachePage - Manage model cache
 *
 * CLI equivalents:
 * - ai-aas-cli model cache list
 * - ai-aas-cli model cache pull
 * - ai-aas-cli model cache delete
 * - ai-aas-cli model cache gc
 */
export default function ModelCachePage() {
  const queryClient = useQueryClient();
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [showGcModal, setShowGcModal] = useState(false);
  const [selectedModel, setSelectedModel] = useState<string | null>(null);

  // Fetch cached models
  const { data: cacheEntries = [], isLoading } = useQuery({
    queryKey: ['model-cache'],
    queryFn: () => modelsApi.cacheList(),
  });

  // Pull model mutation
  const pullMutation = useMutation({
    mutationFn: (name: string) => modelsApi.cachePull(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-cache'] });
    },
  });

  // Delete model from cache mutation
  const deleteMutation = useMutation({
    mutationFn: (name: string) => modelsApi.cacheDelete(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-cache'] });
      setShowDeleteModal(false);
      setSelectedModel(null);
    },
  });

  // Garbage collect mutation
  const gcMutation = useMutation({
    mutationFn: () => modelsApi.cacheGc(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-cache'] });
      setShowGcModal(false);
    },
  });

  // Calculate totals
  const totalSize = cacheEntries.reduce((sum, entry) => sum + entry.size_bytes, 0);

  const columns: Column<CacheEntry>[] = [
    {
      key: 'model_name',
      header: 'Model',
      sortable: true,
      render: (entry) => (
        <span className="font-medium text-gray-900 dark:text-white">{entry.model_name}</span>
      ),
    },
    {
      key: 'size_bytes',
      header: 'Size',
      sortable: true,
      render: (entry) => formatBytes(entry.size_bytes),
    },
    {
      key: 'status',
      header: 'Status',
      render: (entry) => (
        <StatusBadge
          status={entry.status === 'cached' ? 'success' : entry.status === 'downloading' ? 'warning' : 'error'}
        >
          {entry.status}
        </StatusBadge>
      ),
    },
    {
      key: 'cached_at',
      header: 'Cached At',
      sortable: true,
      render: (entry) => new Date(entry.cached_at).toLocaleDateString(),
    },
    {
      key: 'last_accessed',
      header: 'Last Accessed',
      sortable: true,
      render: (entry) => new Date(entry.last_accessed).toLocaleDateString(),
    },
  ];

  const renderActions = (entry: CacheEntry) => (
    <div className="flex items-center space-x-1">
      {entry.status === 'cached' && (
        <IconButton
          icon={
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          }
          label="Re-pull"
          variant="ghost"
          size="sm"
          onClick={() => pullMutation.mutate(entry.model_name)}
        />
      )}
      <IconButton
        icon={
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        }
        label="Delete from cache"
        variant="ghost"
        size="sm"
        className="text-red-600 hover:text-red-700 dark:text-red-400"
        onClick={() => {
          setSelectedModel(entry.model_name);
          setShowDeleteModal(true);
        }}
      />
    </div>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="Model Cache"
        description="Manage cached model weights"
        actions={
          <div className="flex items-center space-x-3">
            <Button
              variant="secondary"
              onClick={() => setShowGcModal(true)}
              leftIcon={
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              }
            >
              Garbage Collect
            </Button>
          </div>
        }
      />

      {/* Summary stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <Card>
          <CardContent>
            <p className="text-sm text-gray-500 dark:text-gray-400">Total Models</p>
            <p className="text-2xl font-bold text-gray-900 dark:text-white">{cacheEntries.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent>
            <p className="text-sm text-gray-500 dark:text-gray-400">Total Size</p>
            <p className="text-2xl font-bold text-gray-900 dark:text-white">{formatBytes(totalSize)}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent>
            <p className="text-sm text-gray-500 dark:text-gray-400">Downloading</p>
            <p className="text-2xl font-bold text-gray-900 dark:text-white">
              {cacheEntries.filter(e => e.status === 'downloading').length}
            </p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={cacheEntries}
            keyExtractor={(entry) => entry.model_name}
            loading={isLoading}
            emptyMessage="No models in cache. Pull a model from the registry to cache it."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* Delete Confirmation Modal */}
      <ConfirmModal
        isOpen={showDeleteModal}
        onClose={() => {
          setShowDeleteModal(false);
          setSelectedModel(null);
        }}
        onConfirm={() => selectedModel && deleteMutation.mutate(selectedModel)}
        title="Delete from Cache"
        message={`Are you sure you want to delete "${selectedModel}" from the cache? The model weights will need to be re-downloaded before deployment.`}
        confirmLabel="Delete"
        variant="danger"
        loading={deleteMutation.isPending}
      />

      {/* GC Confirmation Modal */}
      <ConfirmModal
        isOpen={showGcModal}
        onClose={() => setShowGcModal(false)}
        onConfirm={() => gcMutation.mutate()}
        title="Garbage Collect Cache"
        message="This will remove unused model weights from the cache. Models that are currently deployed will not be affected."
        confirmLabel="Run GC"
        loading={gcMutation.isPending}
      />
    </AdminLayout>
  );
}
