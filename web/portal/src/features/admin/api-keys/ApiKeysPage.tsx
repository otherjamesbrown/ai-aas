import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiKeysApi } from '../api/apiKeys';
import { ConfirmDestructiveModal } from '@/components/ConfirmDestructiveModal';
import { CreateApiKeyModal } from './CreateApiKeyModal';
import { ViewApiKeyModal } from './ViewApiKeyModal';
import { Button, DataTable, StatusBadge, IconButton } from '@/components/ui';
import type { Column } from '@/components/ui';
import type { ApiKeyCredential, CreateApiKeyRequest } from '../types';

/**
 * API key management UI - rebuilt with unified components
 */
export default function ApiKeysPage() {
  const queryClient = useQueryClient();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showRevokeModal, setShowRevokeModal] = useState(false);
  const [showRotateModal, setShowRotateModal] = useState(false);
  const [showViewModal, setShowViewModal] = useState(false);
  const [selectedKey, setSelectedKey] = useState<ApiKeyCredential | null>(null);
  const [newKeySecret, setNewKeySecret] = useState<string | null>(null);

  const { data: keys, isLoading } = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => apiKeysApi.list(),
  });

  const createMutation = useMutation({
    mutationFn: (request: CreateApiKeyRequest) => apiKeysApi.create(request),
    onSuccess: (response) => {
      queryClient.invalidateQueries({ queryKey: ['api-keys'] });
      setShowCreateModal(false);
      if (response.secret) {
        setNewKeySecret(response.secret);
        setShowViewModal(true);
      }
    },
  });

  const rotateMutation = useMutation({
    mutationFn: (keyId: string) => apiKeysApi.rotate(keyId),
    onSuccess: (response) => {
      queryClient.invalidateQueries({ queryKey: ['api-keys'] });
      setShowRotateModal(false);
      if (response.secret) {
        setNewKeySecret(response.secret);
        setShowViewModal(true);
      }
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (keyId: string) => apiKeysApi.revoke(keyId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['api-keys'] });
      setShowRevokeModal(false);
      setSelectedKey(null);
    },
  });

  const maskFingerprint = (fingerprint: string) => `••••${fingerprint.slice(-4)}`;

  const columns: Column<ApiKeyCredential>[] = [
    {
      key: 'display_name',
      header: 'Name',
      sortable: true,
      render: (item) => (
        <span className="font-medium text-gray-900 dark:text-white">{item.display_name}</span>
      ),
    },
    {
      key: 'fingerprint',
      header: 'Fingerprint',
      render: (item) => (
        <code className="text-sm text-gray-500 dark:text-gray-400 font-mono">
          {maskFingerprint(item.fingerprint)}
        </code>
      ),
    },
    {
      key: 'scopes',
      header: 'Scopes',
      render: (item) => (
        <div className="flex flex-wrap gap-1">
          {item.scopes.slice(0, 2).map((scope) => (
            <span
              key={scope}
              className="px-2 py-0.5 text-xs font-medium rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
            >
              {scope}
            </span>
          ))}
          {item.scopes.length > 2 && (
            <span className="px-2 py-0.5 text-xs text-gray-500 dark:text-gray-400">
              +{item.scopes.length - 2}
            </span>
          )}
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (item) => (
        <StatusBadge
          status={item.status === 'active' ? 'success' : item.status === 'revoked' ? 'error' : 'warning'}
        >
          {item.status}
        </StatusBadge>
      ),
    },
    {
      key: 'created_at',
      header: 'Created',
      sortable: true,
      render: (item) => (
        <span className="text-gray-500 dark:text-gray-400">
          {new Date(item.created_at).toLocaleDateString()}
        </span>
      ),
    },
  ];

  const renderActions = (key: ApiKeyCredential) => {
    if (key.status !== 'active') return null;

    return (
      <div className="flex items-center space-x-1">
        <IconButton
          icon={
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          }
          label="Rotate key"
          variant="ghost"
          size="sm"
          onClick={() => {
            setSelectedKey(key);
            setShowRotateModal(true);
          }}
        />
        <IconButton
          icon={
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          }
          label="Revoke key"
          variant="ghost"
          size="sm"
          className="text-red-600 hover:text-red-700 dark:text-red-400"
          onClick={() => {
            setSelectedKey(key);
            setShowRevokeModal(true);
          }}
        />
      </div>
    );
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">API Keys</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Manage API keys for programmatic access to the platform
          </p>
        </div>
        <Button
          onClick={() => setShowCreateModal(true)}
          leftIcon={
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
          }
        >
          Create API Key
        </Button>
      </div>

      {/* Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
        <DataTable
          columns={columns}
          data={keys || []}
          keyExtractor={(item) => item.key_id}
          loading={isLoading}
          emptyMessage="No API keys found. Create your first key to get started."
          actions={renderActions}
        />
      </div>

      {/* Modals */}
      <CreateApiKeyModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={(request) => createMutation.mutate(request)}
        isLoading={createMutation.isPending}
      />

      {selectedKey && (
        <>
          <ConfirmDestructiveModal
            isOpen={showRevokeModal}
            onClose={() => {
              setShowRevokeModal(false);
              setSelectedKey(null);
            }}
            onConfirm={() => selectedKey && revokeMutation.mutate(selectedKey.key_id)}
            title="Revoke API Key"
            message={`Are you sure you want to revoke the API key "${selectedKey.display_name}"? This action cannot be undone and the key will immediately stop working.`}
            confirmText="Revoke API Key"
            confirmationText="REVOKE"
            isLoading={revokeMutation.isPending}
          />

          <ConfirmDestructiveModal
            isOpen={showRotateModal}
            onClose={() => {
              setShowRotateModal(false);
              setSelectedKey(null);
            }}
            onConfirm={() => selectedKey && rotateMutation.mutate(selectedKey.key_id)}
            title="Rotate API Key"
            message={`Are you sure you want to rotate the API key "${selectedKey.display_name}"? A new secret will be generated and the old key will be revoked.`}
            confirmText="Rotate API Key"
            confirmationText="ROTATE"
            isLoading={rotateMutation.isPending}
          />
        </>
      )}

      {newKeySecret && (
        <ViewApiKeyModal
          isOpen={showViewModal}
          onClose={() => {
            setShowViewModal(false);
            setNewKeySecret(null);
          }}
          secret={newKeySecret}
        />
      )}
    </div>
  );
}
