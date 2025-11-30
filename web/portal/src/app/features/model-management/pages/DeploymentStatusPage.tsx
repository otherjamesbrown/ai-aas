import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
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

interface DeploymentStatus {
  name: string;
  model: string;
  replicas: {
    ready: number;
    desired: number;
  };
  status: 'running' | 'pending' | 'failed' | 'unknown';
  sources: {
    kubernetes?: {
      status: string;
      message?: string;
      conditions?: Array<{ type: string; status: string; message?: string }>;
    };
    argocd?: {
      sync_status: string;
      health_status: string;
      message?: string;
    };
    vllm?: {
      status: string;
      model_loaded: boolean;
      gpu_memory_used?: string;
    };
  };
  created_at: string;
  updated_at: string;
}

/**
 * DeploymentStatusPage - Multi-source deployment status inspection
 *
 * CLI equivalent:
 * - ai-aas-cli deployment status
 * - ai-aas-cli deployment status <name>
 */
export default function DeploymentStatusPage() {
  const [selectedDeployment, setSelectedDeployment] = useState<DeploymentStatus | null>(null);

  // Fetch all deployments status
  const { data: deployments = [], isLoading, refetch } = useQuery({
    queryKey: ['deployment-status'],
    queryFn: () => modelsApi.deploymentStatusAll(),
  });

  const getStatusBadge = (status: string) => {
    switch (status.toLowerCase()) {
      case 'running':
      case 'healthy':
      case 'synced':
      case 'true':
        return 'success';
      case 'pending':
      case 'progressing':
      case 'outofsync':
        return 'warning';
      case 'failed':
      case 'error':
      case 'degraded':
      case 'false':
        return 'error';
      default:
        return 'neutral';
    }
  };

  const columns: Column<DeploymentStatus>[] = [
    {
      key: 'name',
      header: 'Deployment',
      sortable: true,
      render: (d) => <span className="font-medium">{d.name}</span>,
    },
    {
      key: 'model',
      header: 'Model',
      sortable: true,
      render: (d) => (
        <code className="text-sm text-gray-600 dark:text-gray-400">{d.model}</code>
      ),
    },
    {
      key: 'replicas',
      header: 'Replicas',
      render: (d) => (
        <span className={d.replicas.ready < d.replicas.desired ? 'text-amber-600' : 'text-green-600'}>
          {d.replicas.ready}/{d.replicas.desired}
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (d) => (
        <StatusBadge status={getStatusBadge(d.status)}>
          {d.status}
        </StatusBadge>
      ),
    },
    {
      key: 'updated_at',
      header: 'Last Update',
      sortable: true,
      render: (d) => (
        <span className="text-sm text-gray-500">
          {new Date(d.updated_at).toLocaleString()}
        </span>
      ),
    },
  ];

  const renderActions = (d: DeploymentStatus) => (
    <Button
      size="sm"
      variant={selectedDeployment?.name === d.name ? 'primary' : 'secondary'}
      onClick={() => setSelectedDeployment(selectedDeployment?.name === d.name ? null : d)}
    >
      {selectedDeployment?.name === d.name ? 'Hide Details' : 'View Details'}
    </Button>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="Deployment Status"
        description="Multi-source status inspection for model deployments"
        actions={
          <Button variant="secondary" onClick={() => refetch()}>
            Refresh
          </Button>
        }
      />

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-gray-500">Total</p>
            <p className="text-2xl font-bold">{deployments.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-gray-500">Running</p>
            <p className="text-2xl font-bold text-green-600">
              {deployments.filter((d) => d.status === 'running').length}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-gray-500">Pending</p>
            <p className="text-2xl font-bold text-amber-600">
              {deployments.filter((d) => d.status === 'pending').length}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-gray-500">Failed</p>
            <p className="text-2xl font-bold text-red-600">
              {deployments.filter((d) => d.status === 'failed').length}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Deployments Table */}
      <Card>
        <CardHeader title="Deployments" />
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={deployments}
            keyExtractor={(d) => d.name}
            loading={isLoading}
            emptyMessage="No deployments found."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* Selected Deployment Details */}
      {selectedDeployment && (
        <Card className="mt-6">
          <CardHeader title={`Source Details: ${selectedDeployment.name}`} />
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {/* Kubernetes Status */}
              <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                  Kubernetes
                </h4>
                {selectedDeployment.sources.kubernetes ? (
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-gray-500">Status</span>
                      <StatusBadge status={getStatusBadge(selectedDeployment.sources.kubernetes.status)}>
                        {selectedDeployment.sources.kubernetes.status}
                      </StatusBadge>
                    </div>
                    {selectedDeployment.sources.kubernetes.message && (
                      <p className="text-xs text-gray-500">{selectedDeployment.sources.kubernetes.message}</p>
                    )}
                    {selectedDeployment.sources.kubernetes.conditions && (
                      <div className="mt-2 space-y-1">
                        {selectedDeployment.sources.kubernetes.conditions.map((c, i) => (
                          <div key={i} className="flex items-center justify-between text-xs">
                            <span className="text-gray-500">{c.type}</span>
                            <StatusBadge status={getStatusBadge(c.status)}>
                              {c.status}
                            </StatusBadge>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ) : (
                  <span className="text-xs text-gray-400">Not available</span>
                )}
              </div>

              {/* ArgoCD Status */}
              <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                  ArgoCD
                </h4>
                {selectedDeployment.sources.argocd ? (
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-gray-500">Sync</span>
                      <StatusBadge status={getStatusBadge(selectedDeployment.sources.argocd.sync_status)}>
                        {selectedDeployment.sources.argocd.sync_status}
                      </StatusBadge>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-gray-500">Health</span>
                      <StatusBadge status={getStatusBadge(selectedDeployment.sources.argocd.health_status)}>
                        {selectedDeployment.sources.argocd.health_status}
                      </StatusBadge>
                    </div>
                    {selectedDeployment.sources.argocd.message && (
                      <p className="text-xs text-gray-500 mt-2">{selectedDeployment.sources.argocd.message}</p>
                    )}
                  </div>
                ) : (
                  <span className="text-xs text-gray-400">Not available</span>
                )}
              </div>

              {/* vLLM Status */}
              <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                  vLLM
                </h4>
                {selectedDeployment.sources.vllm ? (
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-gray-500">Status</span>
                      <StatusBadge status={getStatusBadge(selectedDeployment.sources.vllm.status)}>
                        {selectedDeployment.sources.vllm.status}
                      </StatusBadge>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-gray-500">Model Loaded</span>
                      <StatusBadge status={selectedDeployment.sources.vllm.model_loaded ? 'success' : 'error'}>
                        {selectedDeployment.sources.vllm.model_loaded ? 'Yes' : 'No'}
                      </StatusBadge>
                    </div>
                    {selectedDeployment.sources.vllm.gpu_memory_used && (
                      <div className="flex items-center justify-between">
                        <span className="text-xs text-gray-500">GPU Memory</span>
                        <span className="text-xs font-mono">{selectedDeployment.sources.vllm.gpu_memory_used}</span>
                      </div>
                    )}
                  </div>
                ) : (
                  <span className="text-xs text-gray-400">Not available</span>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># Get status of all deployments</p>
            <p className="text-primary">ai-aas-cli deployment status</p>
            <p className="text-gray-500 mt-3"># Get detailed status of specific deployment</p>
            <p className="text-primary">ai-aas-cli deployment status {'<name>'}</p>
            <p className="text-gray-500 mt-3"># Get status with specific source</p>
            <p className="text-primary">ai-aas-cli deployment status {'<name>'} --source kubernetes</p>
            <p className="text-primary">ai-aas-cli deployment status {'<name>'} --source argocd</p>
            <p className="text-primary">ai-aas-cli deployment status {'<name>'} --source vllm</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
