import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiKeysApi } from '../api/apiKeys';
import { ConfirmDestructiveModal } from '@/components/ConfirmDestructiveModal';
import { CreateApiKeyModal } from './CreateApiKeyModal';
import { ViewApiKeyModal } from './ViewApiKeyModal';
import type { ApiKeyCredential, CreateApiKeyRequest } from '../types';

/**
 * API key management UI (create/rotate/revoke with masked display)
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

  if (isLoading) {
    return <div className="flex items-center justify-center min-h-screen">Loading...</div>;
  }

  const handleRevoke = (key: ApiKeyCredential) => {
    setSelectedKey(key);
    setShowRevokeModal(true);
  };

  const handleRotate = (key: ApiKeyCredential) => {
    setSelectedKey(key);
    setShowRotateModal(true);
  };

  const maskFingerprint = (fingerprint: string) => {
    return `••••${fingerprint.slice(-4)}`;
  };

  return (
    <div className="max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-3xl font-bold text-gray-900">API Keys</h1>
        <button
          onClick={() => setShowCreateModal(true)}
          className="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
        >
          Create API Key
        </button>
      </div>

      {!keys || keys.length === 0 ? (
        <div className="bg-white shadow rounded-lg p-12 text-center">
          <svg
            className="mx-auto h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
            />
          </svg>
          <h3 className="mt-2 text-sm font-semibold text-gray-900">No API keys</h3>
          <p className="mt-1 text-sm text-gray-500">
            Get started by creating your first API key to access the platform programmatically.
          </p>
          <div className="mt-6">
            <button
              onClick={() => setShowCreateModal(true)}
              className="inline-flex items-center px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
            >
              Create API Key
            </button>
          </div>
        </div>
      ) : (
        <div className="bg-white shadow rounded-lg overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Fingerprint
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Scopes
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Created
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {keys.map((key) => (
                <tr key={key.key_id}>
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                    {key.display_name}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 font-mono">
                    {maskFingerprint(key.fingerprint)}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    <div className="flex flex-wrap gap-1">
                      {key.scopes.slice(0, 3).map((scope) => (
                        <span
                          key={scope}
                          className="px-2 py-1 text-xs font-semibold rounded bg-gray-100 text-gray-800"
                        >
                          {scope}
                        </span>
                      ))}
                      {key.scopes.length > 3 && (
                        <span className="px-2 py-1 text-xs text-gray-500">
                          +{key.scopes.length - 3} more
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    <span
                      className={`px-2 py-1 text-xs font-semibold rounded-full ${
                        key.status === 'active'
                          ? 'bg-green-100 text-green-800'
                          : key.status === 'revoked'
                          ? 'bg-red-100 text-red-800'
                          : 'bg-yellow-100 text-yellow-800'
                      }`}
                    >
                      {key.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {new Date(key.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <div className="flex justify-end space-x-2">
                      {key.status === 'active' && (
                        <>
                          <button
                            onClick={() => handleRotate(key)}
                            className="text-primary hover:text-primary-dark"
                          >
                            Rotate
                          </button>
                          <button
                            onClick={() => handleRevoke(key)}
                            className="text-red-600 hover:text-red-900"
                          >
                            Revoke
                          </button>
                        </>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

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

