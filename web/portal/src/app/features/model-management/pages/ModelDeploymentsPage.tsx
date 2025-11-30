import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { modelsApi, Deployment } from '../api/models';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  DataTable,
  StatusBadge,
  Modal,
  Select,
  Input,
  Card,
  CardContent,
  IconButton,
  ConfirmModal,
  FilterPanel,
  FilterSelect,
} from '@/components/ui';
import type { Column } from '@/components/ui';

/**
 * ModelDeploymentsPage - Manage model deployments
 *
 * CLI equivalents:
 * - ai-aas-cli model deploy list
 * - ai-aas-cli model deploy create
 * - ai-aas-cli model deploy delete
 * - ai-aas-cli model deploy scale
 * - ai-aas-cli model deploy status
 */
export default function ModelDeploymentsPage() {
  const queryClient = useQueryClient();
  const [environmentFilter, setEnvironmentFilter] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showScaleModal, setShowScaleModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [showStatusModal, setShowStatusModal] = useState(false);
  const [selectedDeployment, setSelectedDeployment] = useState<Deployment | null>(null);

  // Form state
  const [newModelName, setNewModelName] = useState('');
  const [newEnvironment, setNewEnvironment] = useState('development');
  const [newReplicas, setNewReplicas] = useState(1);
  const [scaleReplicas, setScaleReplicas] = useState(1);

  // Fetch deployments
  const { data: deployments = [], isLoading } = useQuery({
    queryKey: ['model-deployments', environmentFilter],
    queryFn: () => modelsApi.deployList(environmentFilter || undefined),
  });

  // Fetch registry models for dropdown
  const { data: registryModels = [] } = useQuery({
    queryKey: ['model-registry'],
    queryFn: () => modelsApi.registryList(),
  });

  // Fetch deployment status when modal is open
  const { data: deploymentStatus, isLoading: isLoadingStatus } = useQuery({
    queryKey: ['deployment-status', selectedDeployment?.model_name, selectedDeployment?.environment],
    queryFn: () =>
      selectedDeployment
        ? modelsApi.deployStatus(selectedDeployment.model_name, selectedDeployment.environment)
        : null,
    enabled: showStatusModal && !!selectedDeployment,
  });

  // Create deployment mutation
  const createMutation = useMutation({
    mutationFn: () => modelsApi.deployCreate(newModelName, newEnvironment, newReplicas),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-deployments'] });
      setShowCreateModal(false);
      resetForm();
    },
  });

  // Scale deployment mutation
  const scaleMutation = useMutation({
    mutationFn: () =>
      selectedDeployment
        ? modelsApi.deployScale(selectedDeployment.model_name, selectedDeployment.environment, scaleReplicas)
        : Promise.reject('No deployment selected'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-deployments'] });
      setShowScaleModal(false);
      setSelectedDeployment(null);
    },
  });

  // Delete deployment mutation
  const deleteMutation = useMutation({
    mutationFn: () =>
      selectedDeployment
        ? modelsApi.deployDelete(selectedDeployment.model_name, selectedDeployment.environment)
        : Promise.reject('No deployment selected'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-deployments'] });
      setShowDeleteModal(false);
      setSelectedDeployment(null);
    },
  });

  const resetForm = () => {
    setNewModelName('');
    setNewEnvironment('development');
    setNewReplicas(1);
  };

  const columns: Column<Deployment>[] = [
    {
      key: 'model_name',
      header: 'Model',
      sortable: true,
      render: (deployment) => (
        <span className="font-medium text-gray-900 dark:text-white">{deployment.model_name}</span>
      ),
    },
    {
      key: 'environment',
      header: 'Environment',
      render: (deployment) => (
        <span className={`px-2 py-0.5 text-xs font-medium rounded ${
          deployment.environment === 'production'
            ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
            : 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
        }`}>
          {deployment.environment}
        </span>
      ),
    },
    {
      key: 'replicas',
      header: 'Replicas',
      align: 'center',
      render: (deployment) => deployment.replicas,
    },
    {
      key: 'status',
      header: 'Status',
      render: (deployment) => (
        <StatusBadge
          status={
            deployment.status === 'running' ? 'success' :
            deployment.status === 'pending' ? 'warning' :
            deployment.status === 'failed' ? 'error' : 'neutral'
          }
        >
          {deployment.status}
        </StatusBadge>
      ),
    },
    {
      key: 'endpoint',
      header: 'Endpoint',
      render: (deployment) =>
        deployment.endpoint ? (
          <code className="text-xs text-gray-600 dark:text-gray-400">{deployment.endpoint}</code>
        ) : (
          <span className="text-gray-400">-</span>
        ),
    },
    {
      key: 'created_at',
      header: 'Created',
      sortable: true,
      render: (deployment) => new Date(deployment.created_at).toLocaleDateString(),
    },
  ];

  const renderActions = (deployment: Deployment) => (
    <div className="flex items-center space-x-1">
      <IconButton
        icon={
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
        }
        label="View status"
        variant="ghost"
        size="sm"
        onClick={() => {
          setSelectedDeployment(deployment);
          setShowStatusModal(true);
        }}
      />
      <IconButton
        icon={
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
          </svg>
        }
        label="Scale"
        variant="ghost"
        size="sm"
        onClick={() => {
          setSelectedDeployment(deployment);
          setScaleReplicas(deployment.replicas);
          setShowScaleModal(true);
        }}
      />
      <IconButton
        icon={
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        }
        label="Delete"
        variant="ghost"
        size="sm"
        className="text-red-600 hover:text-red-700 dark:text-red-400"
        onClick={() => {
          setSelectedDeployment(deployment);
          setShowDeleteModal(true);
        }}
      />
    </div>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="Model Deployments"
        description="Manage model deployments across environments"
        actions={
          <Button
            onClick={() => setShowCreateModal(true)}
            leftIcon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
            }
          >
            Create Deployment
          </Button>
        }
      />

      {/* Filters */}
      <FilterPanel onReset={() => setEnvironmentFilter('')} className="mb-6">
        <FilterSelect
          label="Environment"
          value={environmentFilter}
          options={[
            { value: 'development', label: 'Development' },
            { value: 'staging', label: 'Staging' },
            { value: 'production', label: 'Production' },
          ]}
          onChange={setEnvironmentFilter}
        />
      </FilterPanel>

      <Card>
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={deployments}
            keyExtractor={(deployment) => deployment.id}
            loading={isLoading}
            emptyMessage="No deployments found. Create a deployment to serve a model."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* Create Deployment Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => {
          setShowCreateModal(false);
          resetForm();
        }}
        title="Create Deployment"
        description="Deploy a model to an environment"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowCreateModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => createMutation.mutate()}
              loading={createMutation.isPending}
              disabled={!newModelName}
            >
              Deploy
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Select
            label="Model"
            value={newModelName}
            onChange={(e) => setNewModelName(e.target.value)}
            required
          >
            <option value="">Select a model</option>
            {registryModels.map((model) => (
              <option key={model.id} value={model.name}>
                {model.display_name || model.name}
              </option>
            ))}
          </Select>
          <Select
            label="Environment"
            value={newEnvironment}
            onChange={(e) => setNewEnvironment(e.target.value)}
          >
            <option value="development">Development</option>
            <option value="staging">Staging</option>
            <option value="production">Production</option>
          </Select>
          <Input
            label="Replicas"
            type="number"
            min={1}
            max={10}
            value={newReplicas}
            onChange={(e) => setNewReplicas(parseInt(e.target.value, 10))}
          />
        </div>
      </Modal>

      {/* Scale Modal */}
      <Modal
        isOpen={showScaleModal}
        onClose={() => {
          setShowScaleModal(false);
          setSelectedDeployment(null);
        }}
        title="Scale Deployment"
        description={`Scale ${selectedDeployment?.model_name} in ${selectedDeployment?.environment}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowScaleModal(false)}>
              Cancel
            </Button>
            <Button onClick={() => scaleMutation.mutate()} loading={scaleMutation.isPending}>
              Scale
            </Button>
          </>
        }
      >
        <Input
          label="Replicas"
          type="number"
          min={0}
          max={10}
          value={scaleReplicas}
          onChange={(e) => setScaleReplicas(parseInt(e.target.value, 10))}
          hint="Set to 0 to scale down completely"
        />
      </Modal>

      {/* Delete Confirmation Modal */}
      <ConfirmModal
        isOpen={showDeleteModal}
        onClose={() => {
          setShowDeleteModal(false);
          setSelectedDeployment(null);
        }}
        onConfirm={() => deleteMutation.mutate()}
        title="Delete Deployment"
        message={`Are you sure you want to delete the deployment of "${selectedDeployment?.model_name}" in ${selectedDeployment?.environment}? This will stop the model from being available for inference.`}
        confirmLabel="Delete"
        variant="danger"
        loading={deleteMutation.isPending}
      />

      {/* Status Modal */}
      <Modal
        isOpen={showStatusModal}
        onClose={() => {
          setShowStatusModal(false);
          setSelectedDeployment(null);
        }}
        title="Deployment Status"
        size="lg"
      >
        {isLoadingStatus ? (
          <div className="flex items-center justify-center py-8">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
          </div>
        ) : deploymentStatus ? (
          <div className="space-y-6">
            <div>
              <h4 className="text-sm font-medium mb-2">Pod Status</h4>
              <div className="space-y-2">
                {deploymentStatus.pod_status.map((pod) => (
                  <div
                    key={pod.name}
                    className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-md"
                  >
                    <div>
                      <p className="font-mono text-sm">{pod.name}</p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        Restarts: {pod.restarts}
                      </p>
                    </div>
                    <StatusBadge status={pod.ready ? 'success' : 'warning'}>
                      {pod.status}
                    </StatusBadge>
                  </div>
                ))}
              </div>
            </div>

            <div>
              <h4 className="text-sm font-medium mb-2">Endpoint Health</h4>
              <div className="p-3 bg-gray-50 dark:bg-gray-800 rounded-md">
                <div className="flex items-center justify-between">
                  <span>Endpoint Check</span>
                  <StatusBadge status={deploymentStatus.endpoint_status.healthy ? 'success' : 'error'}>
                    {deploymentStatus.endpoint_status.healthy ? 'Healthy' : 'Unhealthy'}
                  </StatusBadge>
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Last check: {new Date(deploymentStatus.endpoint_status.last_check).toLocaleString()}
                </p>
              </div>
            </div>
          </div>
        ) : (
          <p className="text-gray-500">No status available</p>
        )}
      </Modal>
    </AdminLayout>
  );
}
