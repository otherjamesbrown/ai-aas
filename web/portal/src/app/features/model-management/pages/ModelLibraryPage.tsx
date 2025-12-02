import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { modelsApi, LibraryEntry } from '../api/models';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  Card,
  CardHeader,
  CardContent,
  DataTable,
  StatusBadge,
  Modal,
  Input,
  ConfirmModal,
} from '@/components/ui';
import type { Column } from '@/components/ui';

interface HistoryEntry {
  action: string;
  timestamp: string;
  details: string;
}

/**
 * ModelLibraryPage - Manage model library
 *
 * CLI equivalents:
 * - ai-aas-cli model library list
 * - ai-aas-cli model library enable <model>
 * - ai-aas-cli model library disable <model>
 * - ai-aas-cli model library swap <model> <version>
 * - ai-aas-cli model library history <model>
 * - ai-aas-cli model library alias <model> <alias>
 */
export default function ModelLibraryPage() {
  const queryClient = useQueryClient();
  const [showHistoryModal, setShowHistoryModal] = useState(false);
  const [showAliasModal, setShowAliasModal] = useState(false);
  const [showSwapModal, setShowSwapModal] = useState(false);
  const [showDisableModal, setShowDisableModal] = useState(false);
  const [selectedModel, setSelectedModel] = useState<LibraryEntry | null>(null);
  const [newAlias, setNewAlias] = useState('');
  const [swapVersion, setSwapVersion] = useState('');

  // Fetch library models
  const { data: models = [], isLoading } = useQuery({
    queryKey: ['library-models'],
    queryFn: () => modelsApi.libraryList(),
  });

  // Fetch model history
  const { data: history = [] } = useQuery({
    queryKey: ['model-history', selectedModel?.model_name],
    queryFn: () => modelsApi.libraryHistory(selectedModel!.model_name),
    enabled: !!selectedModel && showHistoryModal,
  });

  // Enable mutation
  const enableMutation = useMutation({
    mutationFn: (modelName: string) => modelsApi.libraryEnable(modelName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['library-models'] });
    },
  });

  // Disable mutation
  const disableMutation = useMutation({
    mutationFn: (modelName: string) => modelsApi.libraryDisable(modelName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['library-models'] });
      setShowDisableModal(false);
      setSelectedModel(null);
    },
  });

  // Swap mutation
  const swapMutation = useMutation({
    mutationFn: ({ modelName, version }: { modelName: string; version: string }) =>
      modelsApi.librarySwap(modelName, version),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['library-models'] });
      setShowSwapModal(false);
      setSelectedModel(null);
      setSwapVersion('');
    },
  });

  // Alias mutation
  const aliasMutation = useMutation({
    mutationFn: ({ modelName, alias }: { modelName: string; alias: string }) =>
      modelsApi.libraryAlias(modelName, alias),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['library-models'] });
      setShowAliasModal(false);
      setSelectedModel(null);
      setNewAlias('');
    },
  });

  const columns: Column<LibraryEntry>[] = [
    {
      key: 'model_name',
      header: 'Model',
      sortable: true,
      render: (m) => (
        <div>
          <span className="font-medium">{m.model_name}</span>
          {m.alias && (
            <span className="ml-2 text-xs text-gray-500">alias: {m.alias}</span>
          )}
        </div>
      ),
    },
    {
      key: 'enabled',
      header: 'Status',
      render: (m) => (
        <StatusBadge status={m.enabled ? 'success' : 'neutral'}>
          {m.enabled ? 'Enabled' : 'Disabled'}
        </StatusBadge>
      ),
    },
    {
      key: 'swap_target',
      header: 'Swap Target',
      render: (m) =>
        m.swap_target ? (
          <span className="font-mono text-sm">{m.swap_target}</span>
        ) : (
          <span className="text-gray-400">-</span>
        ),
    },
    {
      key: 'created_at',
      header: 'Created',
      sortable: true,
      render: (m) => new Date(m.created_at).toLocaleDateString(),
    },
  ];

  const renderActions = (model: LibraryEntry) => (
    <div className="flex items-center space-x-2">
      {model.enabled ? (
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            setSelectedModel(model);
            setShowDisableModal(true);
          }}
        >
          Disable
        </Button>
      ) : (
        <Button
          size="sm"
          onClick={() => enableMutation.mutate(model.model_name)}
          loading={enableMutation.isPending}
        >
          Enable
        </Button>
      )}
      <Button
        variant="ghost"
        size="sm"
        onClick={() => {
          setSelectedModel(model);
          setSwapVersion(model.swap_target || '');
          setShowSwapModal(true);
        }}
      >
        Swap
      </Button>
      <Button
        variant="ghost"
        size="sm"
        onClick={() => {
          setSelectedModel(model);
          setNewAlias(model.alias || '');
          setShowAliasModal(true);
        }}
      >
        Alias
      </Button>
      <Button
        variant="ghost"
        size="sm"
        onClick={() => {
          setSelectedModel(model);
          setShowHistoryModal(true);
        }}
      >
        History
      </Button>
    </div>
  );

  const historyColumns: Column<HistoryEntry>[] = [
    {
      key: 'action',
      header: 'Action',
      render: (h) => <span className="font-medium">{h.action}</span>,
    },
    {
      key: 'timestamp',
      header: 'Time',
      render: (h) => new Date(h.timestamp).toLocaleString(),
    },
    {
      key: 'details',
      header: 'Details',
      render: (h) => <span className="text-gray-600 dark:text-gray-400">{h.details}</span>,
    },
  ];

  return (
    <AdminLayout>
      <PageHeader
        title="Model Library"
        description="Manage models available for deployment"
      />

      <Card>
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={models}
            keyExtractor={(m) => m.model_name}
            loading={isLoading}
            emptyMessage="No models in library."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* History Modal */}
      <Modal
        isOpen={showHistoryModal}
        onClose={() => {
          setShowHistoryModal(false);
          setSelectedModel(null);
        }}
        title={`History: ${selectedModel?.model_name}`}
        size="lg"
      >
        <DataTable
          columns={historyColumns}
          data={history}
          keyExtractor={(h) => `${h.action}-${h.timestamp}`}
          emptyMessage="No history found."
        />
      </Modal>

      {/* Alias Modal */}
      <Modal
        isOpen={showAliasModal}
        onClose={() => {
          setShowAliasModal(false);
          setSelectedModel(null);
          setNewAlias('');
        }}
        title={`Set Alias: ${selectedModel?.model_name}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowAliasModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() =>
                selectedModel &&
                aliasMutation.mutate({ modelName: selectedModel.model_name, alias: newAlias })
              }
              loading={aliasMutation.isPending}
            >
              Set Alias
            </Button>
          </>
        }
      >
        <Input
          label="Alias"
          value={newAlias}
          onChange={(e) => setNewAlias(e.target.value)}
          hint="A short, memorable name for this model"
        />
      </Modal>

      {/* Swap Modal */}
      <Modal
        isOpen={showSwapModal}
        onClose={() => {
          setShowSwapModal(false);
          setSelectedModel(null);
          setSwapVersion('');
        }}
        title={`Swap Target: ${selectedModel?.model_name}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowSwapModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() =>
                selectedModel &&
                swapMutation.mutate({ modelName: selectedModel.model_name, version: swapVersion })
              }
              loading={swapMutation.isPending}
              disabled={!swapVersion}
            >
              Set Swap Target
            </Button>
          </>
        }
      >
        <Input
          label="Target Model"
          value={swapVersion}
          onChange={(e) => setSwapVersion(e.target.value)}
          hint="Model to swap to when this model is requested"
        />
      </Modal>

      {/* Disable Confirmation */}
      <ConfirmModal
        isOpen={showDisableModal}
        onClose={() => {
          setShowDisableModal(false);
          setSelectedModel(null);
        }}
        onConfirm={() => selectedModel && disableMutation.mutate(selectedModel.model_name)}
        title="Disable Model"
        message={`Are you sure you want to disable "${selectedModel?.model_name}"? This will prevent new deployments of this model.`}
        confirmLabel="Disable"
        variant="danger"
        loading={disableMutation.isPending}
      />

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># List library models</p>
            <p className="text-primary">ai-aas-cli model library list</p>
            <p className="text-gray-500 mt-3"># Enable/disable model</p>
            <p className="text-primary">ai-aas-cli model library enable {'<model>'}</p>
            <p className="text-primary">ai-aas-cli model library disable {'<model>'}</p>
            <p className="text-gray-500 mt-3"># Swap to different model</p>
            <p className="text-primary">ai-aas-cli model library swap {'<model> <target>'}</p>
            <p className="text-gray-500 mt-3"># View history</p>
            <p className="text-primary">ai-aas-cli model library history {'<model>'}</p>
            <p className="text-gray-500 mt-3"># Set model alias</p>
            <p className="text-primary">ai-aas-cli model library alias {'<model> <alias>'}</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
