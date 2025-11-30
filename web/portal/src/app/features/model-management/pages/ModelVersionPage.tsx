import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { modelsApi } from '../api/models';
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

interface ModelVersion {
  model_name: string;
  current_version: string;
  latest_version: string;
  pinned: boolean;
  update_available: boolean;
  source: string;
}

/**
 * ModelVersionPage - Manage model versions
 *
 * CLI equivalents:
 * - ai-aas-cli model version check
 * - ai-aas-cli model version update <model>
 * - ai-aas-cli model version pin <model> <version>
 * - ai-aas-cli model version unpin <model>
 */
export default function ModelVersionPage() {
  const queryClient = useQueryClient();

  // Fetch version info for all models
  const { data: versions = [], isLoading } = useQuery({
    queryKey: ['model-versions'],
    queryFn: () => modelsApi.versionCheck(),
  });

  // Update model mutation
  const updateMutation = useMutation({
    mutationFn: (modelName: string) => modelsApi.versionUpdate(modelName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-versions'] });
    },
  });

  // Pin model mutation
  const pinMutation = useMutation({
    mutationFn: ({ modelName, version }: { modelName: string; version: string }) =>
      modelsApi.versionPin(modelName, version),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-versions'] });
    },
  });

  // Unpin model mutation
  const unpinMutation = useMutation({
    mutationFn: (modelName: string) => modelsApi.versionUnpin(modelName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-versions'] });
    },
  });

  const columns: Column<ModelVersion>[] = [
    {
      key: 'model_name',
      header: 'Model',
      sortable: true,
      render: (v) => <span className="font-medium">{v.model_name}</span>,
    },
    {
      key: 'current_version',
      header: 'Current Version',
      render: (v) => (
        <span className="font-mono text-sm">{v.current_version || 'N/A'}</span>
      ),
    },
    {
      key: 'latest_version',
      header: 'Latest Version',
      render: (v) => (
        <span className="font-mono text-sm">{v.latest_version || 'N/A'}</span>
      ),
    },
    {
      key: 'update_available',
      header: 'Status',
      render: (v) => (
        <div className="flex items-center gap-2">
          {v.pinned && (
            <StatusBadge status="warning">Pinned</StatusBadge>
          )}
          {v.update_available ? (
            <StatusBadge status="info">Update Available</StatusBadge>
          ) : (
            <StatusBadge status="success">Up to Date</StatusBadge>
          )}
        </div>
      ),
    },
    {
      key: 'source',
      header: 'Source',
      render: (v) => (
        <span className="text-sm text-gray-500">{v.source || 'huggingface'}</span>
      ),
    },
  ];

  const renderActions = (version: ModelVersion) => (
    <div className="flex items-center space-x-2">
      {version.update_available && !version.pinned && (
        <Button
          size="sm"
          onClick={() => updateMutation.mutate(version.model_name)}
          loading={updateMutation.isPending}
        >
          Update
        </Button>
      )}
      {version.pinned ? (
        <Button
          variant="secondary"
          size="sm"
          onClick={() => unpinMutation.mutate(version.model_name)}
          loading={unpinMutation.isPending}
        >
          Unpin
        </Button>
      ) : (
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            pinMutation.mutate({
              modelName: version.model_name,
              version: version.current_version,
            })
          }
          loading={pinMutation.isPending}
        >
          Pin
        </Button>
      )}
    </div>
  );

  const updateableCount = versions.filter((v) => v.update_available && !v.pinned).length;

  return (
    <AdminLayout>
      <PageHeader
        title="Model Versions"
        description="Check and manage model version updates"
        actions={
          updateableCount > 0 && (
            <Button
              onClick={() => {
                versions
                  .filter((v) => v.update_available && !v.pinned)
                  .forEach((v) => updateMutation.mutate(v.model_name));
              }}
              loading={updateMutation.isPending}
            >
              Update All ({updateableCount})
            </Button>
          )
        }
      />

      <Card>
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={versions}
            keyExtractor={(v) => v.model_name}
            loading={isLoading}
            emptyMessage="No models found in registry."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># Check all model versions</p>
            <p className="text-primary">ai-aas-cli model version check</p>
            <p className="text-gray-500 mt-3"># Update a model to latest version</p>
            <p className="text-primary">ai-aas-cli model version update {'<model>'}</p>
            <p className="text-gray-500 mt-3"># Pin model to specific version</p>
            <p className="text-primary">ai-aas-cli model version pin {'<model> <version>'}</p>
            <p className="text-gray-500 mt-3"># Unpin model</p>
            <p className="text-primary">ai-aas-cli model version unpin {'<model>'}</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
