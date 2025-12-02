import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { accessControlApi, Credentials } from '../api/accessControl';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  StatusBadge,
  Modal,
  Input,
  Select,
  Card,
  CardContent,
  CardHeader,
  IconButton,
  ConfirmModal,
} from '@/components/ui';

const CREDENTIAL_TYPES = [
  { value: 'huggingface', label: 'HuggingFace', description: 'Access to HuggingFace Hub models' },
  { value: 'aws', label: 'AWS', description: 'AWS credentials for S3, SageMaker' },
  { value: 'gcp', label: 'Google Cloud', description: 'GCP service account' },
  { value: 'azure', label: 'Azure', description: 'Azure credentials' },
  { value: 'openai', label: 'OpenAI', description: 'OpenAI API key' },
  { value: 'anthropic', label: 'Anthropic', description: 'Anthropic API key' },
];

/**
 * CredentialsPage - Manage platform credentials
 *
 * CLI equivalents:
 * - ai-aas-cli credentials list
 * - ai-aas-cli credentials set
 * - ai-aas-cli credentials test
 */
export default function CredentialsPage() {
  const queryClient = useQueryClient();
  const [showSetModal, setShowSetModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [selectedCredential, setSelectedCredential] = useState<Credentials | null>(null);
  const [testingType, setTestingType] = useState<string | null>(null);

  // Form state
  const [credType, setCredType] = useState('');
  const [credName, setCredName] = useState('');
  const [credValue, setCredValue] = useState('');

  // Fetch credentials
  const { data: credentials = [], isLoading } = useQuery({
    queryKey: ['credentials'],
    queryFn: () => accessControlApi.credentialsList(),
  });

  // Set credential mutation
  const setMutation = useMutation({
    mutationFn: () => accessControlApi.credentialsSet(credType, credName, credValue),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['credentials'] });
      setShowSetModal(false);
      resetForm();
    },
  });

  // Test credential mutation
  const testMutation = useMutation({
    mutationFn: (type: string) => accessControlApi.credentialsTest(type),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['credentials'] });
      setTestingType(null);
    },
    onError: () => {
      queryClient.invalidateQueries({ queryKey: ['credentials'] });
      setTestingType(null);
    },
  });

  // Delete credential mutation
  const deleteMutation = useMutation({
    mutationFn: (type: string) => accessControlApi.credentialsDelete(type),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['credentials'] });
      setShowDeleteModal(false);
      setSelectedCredential(null);
    },
  });

  const resetForm = () => {
    setCredType('');
    setCredName('');
    setCredValue('');
  };

  const handleTest = (type: string) => {
    setTestingType(type);
    testMutation.mutate(type);
  };

  // Get unconfigured credential types
  const configuredTypes = new Set(credentials.map((c) => c.type));
  const availableTypes = CREDENTIAL_TYPES.filter((t) => !configuredTypes.has(t.value));

  return (
    <AdminLayout>
      <PageHeader
        title="Credentials"
        description="Manage external service credentials"
        actions={
          <Button
            onClick={() => setShowSetModal(true)}
            disabled={availableTypes.length === 0}
            leftIcon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
            }
          >
            Add Credential
          </Button>
        }
      />

      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
        </div>
      ) : credentials.length === 0 ? (
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
                  d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
                />
              </svg>
              <p className="mt-4 text-sm text-gray-500 dark:text-gray-400">
                No credentials configured. Add credentials to connect external services.
              </p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {credentials.map((cred) => {
            const typeInfo = CREDENTIAL_TYPES.find((t) => t.value === cred.type);
            const isTesting = testingType === cred.type;

            return (
              <Card key={cred.type}>
                <CardHeader
                  title={typeInfo?.label || cred.type}
                  actions={
                    <StatusBadge
                      status={
                        cred.test_result === 'success'
                          ? 'success'
                          : cred.test_result === 'failure'
                          ? 'error'
                          : cred.configured
                          ? 'info'
                          : 'neutral'
                      }
                    >
                      {cred.test_result === 'success'
                        ? 'Verified'
                        : cred.test_result === 'failure'
                        ? 'Failed'
                        : cred.configured
                        ? 'Configured'
                        : 'Not Set'}
                    </StatusBadge>
                  }
                />
                <CardContent>
                  <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                    {typeInfo?.description || 'External service credential'}
                  </p>

                  <div className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span className="text-gray-500 dark:text-gray-400">Name:</span>
                      <span className="font-medium">{cred.name}</span>
                    </div>
                    {cred.last_tested && (
                      <div className="flex justify-between">
                        <span className="text-gray-500 dark:text-gray-400">Last tested:</span>
                        <span>{new Date(cred.last_tested).toLocaleDateString()}</span>
                      </div>
                    )}
                  </div>

                  <div className="flex items-center justify-end space-x-2 mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => handleTest(cred.type)}
                      loading={isTesting}
                    >
                      Test
                    </Button>
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
                        setSelectedCredential(cred);
                        setShowDeleteModal(true);
                      }}
                    />
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      {/* Set Credential Modal */}
      <Modal
        isOpen={showSetModal}
        onClose={() => {
          setShowSetModal(false);
          resetForm();
        }}
        title="Add Credential"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowSetModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => setMutation.mutate()}
              loading={setMutation.isPending}
              disabled={!credType || !credName || !credValue}
            >
              Save
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Select
            label="Credential Type"
            value={credType}
            onChange={(e) => setCredType(e.target.value)}
            required
          >
            <option value="">Select a type</option>
            {availableTypes.map((type) => (
              <option key={type.value} value={type.value}>
                {type.label}
              </option>
            ))}
          </Select>
          <Input
            label="Name / Key ID"
            value={credName}
            onChange={(e) => setCredName(e.target.value)}
            placeholder="e.g., hf_xxx, AKIAIOSFODNN7EXAMPLE"
            required
          />
          <Input
            label="Secret / Token"
            type="password"
            value={credValue}
            onChange={(e) => setCredValue(e.target.value)}
            required
          />
          {credType && (
            <p className="text-sm text-gray-500 dark:text-gray-400">
              {CREDENTIAL_TYPES.find((t) => t.value === credType)?.description}
            </p>
          )}
        </div>
      </Modal>

      {/* Delete Confirmation */}
      <ConfirmModal
        isOpen={showDeleteModal}
        onClose={() => {
          setShowDeleteModal(false);
          setSelectedCredential(null);
        }}
        onConfirm={() => selectedCredential && deleteMutation.mutate(selectedCredential.type)}
        title="Delete Credential"
        message={`Are you sure you want to delete the ${selectedCredential?.type} credential? Services using this credential will stop working.`}
        confirmLabel="Delete"
        variant="danger"
        loading={deleteMutation.isPending}
      />
    </AdminLayout>
  );
}
