import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { modelsApi, RegistryModel } from '../api/models';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  DataTable,
  StatusBadge,
  Modal,
  Input,
  Card,
  CardContent,
  IconButton,
} from '@/components/ui';
import type { Column } from '@/components/ui';

/**
 * ModelRegistryPage - Manage model registry
 *
 * CLI equivalents:
 * - ai-aas-cli model registry list
 * - ai-aas-cli model registry add
 * - ai-aas-cli model registry info
 * - ai-aas-cli model registry remove
 */
export default function ModelRegistryPage() {
  const queryClient = useQueryClient();
  const [showAddModal, setShowAddModal] = useState(false);
  const [showInfoModal, setShowInfoModal] = useState(false);
  const [selectedModel, setSelectedModel] = useState<RegistryModel | null>(null);
  const [huggingfaceId, setHuggingfaceId] = useState('');
  const [modelName, setModelName] = useState('');
  const [displayName, setDisplayName] = useState('');

  // Fetch registry models
  const { data: models = [], isLoading } = useQuery({
    queryKey: ['model-registry'],
    queryFn: () => modelsApi.registryList(),
  });

  // Add model mutation
  const addMutation = useMutation({
    mutationFn: () => modelsApi.registryAdd(huggingfaceId, modelName || undefined, displayName || undefined),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-registry'] });
      setShowAddModal(false);
      resetForm();
    },
  });

  // Remove model mutation
  const removeMutation = useMutation({
    mutationFn: (name: string) => modelsApi.registryRemove(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-registry'] });
    },
  });

  const resetForm = () => {
    setHuggingfaceId('');
    setModelName('');
    setDisplayName('');
  };

  const handleViewInfo = (model: RegistryModel) => {
    setSelectedModel(model);
    setShowInfoModal(true);
  };

  const columns: Column<RegistryModel>[] = [
    {
      key: 'name',
      header: 'Name',
      sortable: true,
      render: (model) => (
        <div>
          <span className="font-medium text-gray-900 dark:text-white">{model.name}</span>
          {model.display_name && (
            <p className="text-sm text-gray-500 dark:text-gray-400">{model.display_name}</p>
          )}
        </div>
      ),
    },
    {
      key: 'huggingface_id',
      header: 'HuggingFace ID',
      render: (model) => (
        <code className="text-sm text-gray-600 dark:text-gray-400">{model.huggingface_id}</code>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (model) => (
        <StatusBadge
          status={model.status === 'deployed' ? 'success' : model.status === 'cached' ? 'info' : 'neutral'}
        >
          {model.status}
        </StatusBadge>
      ),
    },
    {
      key: 'created_at',
      header: 'Added',
      sortable: true,
      render: (model) => new Date(model.created_at).toLocaleDateString(),
    },
  ];

  const renderActions = (model: RegistryModel) => (
    <div className="flex items-center space-x-1">
      <IconButton
        icon={
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        }
        label="View info"
        variant="ghost"
        size="sm"
        onClick={() => handleViewInfo(model)}
      />
      <IconButton
        icon={
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        }
        label="Remove"
        variant="ghost"
        size="sm"
        className="text-red-600 hover:text-red-700 dark:text-red-400"
        onClick={() => removeMutation.mutate(model.name)}
      />
    </div>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="Model Registry"
        description="Manage registered models for deployment"
        actions={
          <Button
            onClick={() => setShowAddModal(true)}
            leftIcon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
            }
          >
            Add Model
          </Button>
        }
      />

      <Card>
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={models}
            keyExtractor={(model) => model.id}
            loading={isLoading}
            emptyMessage="No models registered. Add a model from HuggingFace to get started."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* Add Model Modal */}
      <Modal
        isOpen={showAddModal}
        onClose={() => {
          setShowAddModal(false);
          resetForm();
        }}
        title="Add Model to Registry"
        description="Register a model from HuggingFace for deployment"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowAddModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => addMutation.mutate()}
              loading={addMutation.isPending}
              disabled={!huggingfaceId}
            >
              Add Model
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="HuggingFace ID"
            placeholder="e.g., meta-llama/Llama-2-7b-hf"
            value={huggingfaceId}
            onChange={(e) => setHuggingfaceId(e.target.value)}
            required
          />
          <Input
            label="Model Name (optional)"
            placeholder="e.g., llama-7b"
            value={modelName}
            onChange={(e) => setModelName(e.target.value)}
            hint="Short name for CLI commands. Auto-generated if not provided."
          />
          <Input
            label="Display Name (optional)"
            placeholder="e.g., Llama 2 7B"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
          />
        </div>
      </Modal>

      {/* Model Info Modal */}
      <Modal
        isOpen={showInfoModal}
        onClose={() => {
          setShowInfoModal(false);
          setSelectedModel(null);
        }}
        title="Model Information"
        size="lg"
      >
        {selectedModel && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Name</label>
                <p className="font-medium">{selectedModel.name}</p>
              </div>
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Status</label>
                <p>
                  <StatusBadge
                    status={selectedModel.status === 'deployed' ? 'success' : selectedModel.status === 'cached' ? 'info' : 'neutral'}
                  >
                    {selectedModel.status}
                  </StatusBadge>
                </p>
              </div>
              <div className="col-span-2">
                <label className="text-xs text-gray-500 dark:text-gray-400">HuggingFace ID</label>
                <p className="font-mono text-sm">{selectedModel.huggingface_id}</p>
              </div>
              {selectedModel.display_name && (
                <div className="col-span-2">
                  <label className="text-xs text-gray-500 dark:text-gray-400">Display Name</label>
                  <p>{selectedModel.display_name}</p>
                </div>
              )}
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Created</label>
                <p>{new Date(selectedModel.created_at).toLocaleString()}</p>
              </div>
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400">Updated</label>
                <p>{new Date(selectedModel.updated_at).toLocaleString()}</p>
              </div>
            </div>

            <div className="pt-4 border-t border-gray-200 dark:border-gray-700">
              <h4 className="text-sm font-medium mb-2">CLI Commands</h4>
              <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-3 space-y-1 font-mono text-xs">
                <p># View model info</p>
                <p className="text-primary">ai-aas-cli model registry info {selectedModel.name}</p>
                <p className="mt-2"># Cache model weights</p>
                <p className="text-primary">ai-aas-cli model cache pull {selectedModel.name}</p>
                <p className="mt-2"># Deploy model</p>
                <p className="text-primary">ai-aas-cli model deploy create {selectedModel.name} -e development</p>
              </div>
            </div>
          </div>
        )}
      </Modal>
    </AdminLayout>
  );
}
