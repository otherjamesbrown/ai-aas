import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { accessControlApi, User } from '../api/accessControl';
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
  Select,
  ConfirmModal,
} from '@/components/ui';
import type { Column } from '@/components/ui';

interface ModelAccess {
  model_name: string;
  granted: boolean;
  granted_at?: string;
  granted_by?: string;
}


/**
 * UserModelAccessPage - Manage user model access
 *
 * CLI equivalents:
 * - ai-aas-cli user model-access show <user>
 * - ai-aas-cli user model-access set-mode <user> <mode>
 * - ai-aas-cli user model-access grant <user> <model>
 * - ai-aas-cli user model-access revoke <user> <model>
 * - ai-aas-cli user model-access list <user>
 * - ai-aas-cli user model-access grant-all <user>
 */
export default function UserModelAccessPage() {
  const queryClient = useQueryClient();
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [showGrantModal, setShowGrantModal] = useState(false);
  const [showRevokeModal, setShowRevokeModal] = useState(false);
  const [showModeModal, setShowModeModal] = useState(false);
  const [modelToGrant, setModelToGrant] = useState('');
  const [selectedModel, setSelectedModel] = useState<ModelAccess | null>(null);
  const [newMode, setNewMode] = useState<'allowlist' | 'denylist' | 'all'>('allowlist');

  // Fetch users
  const { data: users = [], isLoading: usersLoading } = useQuery({
    queryKey: ['users'],
    queryFn: () => accessControlApi.userList(),
  });

  // Fetch model access for selected user
  const { data: accessInfo, isLoading: accessLoading } = useQuery({
    queryKey: ['user-model-access', selectedUser?.id],
    queryFn: () => accessControlApi.userModelAccessShow(selectedUser!.id),
    enabled: !!selectedUser,
  });

  // Available models for granting
  const { data: availableModels = [] } = useQuery({
    queryKey: ['available-models'],
    queryFn: () => accessControlApi.listAvailableModels(),
  });

  // Grant mutation
  const grantMutation = useMutation({
    mutationFn: ({ userId, modelName }: { userId: string; modelName: string }) =>
      accessControlApi.userModelAccessGrant(userId, modelName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-model-access', selectedUser?.id] });
      setShowGrantModal(false);
      setModelToGrant('');
    },
  });

  // Revoke mutation
  const revokeMutation = useMutation({
    mutationFn: ({ userId, modelName }: { userId: string; modelName: string }) =>
      accessControlApi.userModelAccessRevoke(userId, modelName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-model-access', selectedUser?.id] });
      setShowRevokeModal(false);
      setSelectedModel(null);
    },
  });

  // Set mode mutation
  const setModeMutation = useMutation({
    mutationFn: ({ userId, mode }: { userId: string; mode: 'allowlist' | 'denylist' | 'all' }) =>
      accessControlApi.userModelAccessSetMode(userId, mode),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-model-access', selectedUser?.id] });
      setShowModeModal(false);
    },
  });

  // Grant all mutation
  const grantAllMutation = useMutation({
    mutationFn: (userId: string) => accessControlApi.userModelAccessGrantAll(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-model-access', selectedUser?.id] });
    },
  });

  const userColumns: Column<User>[] = [
    {
      key: 'email',
      header: 'User',
      sortable: true,
      render: (user) => (
        <div>
          <span className="font-medium">{user.email}</span>
          {user.name && (
            <p className="text-sm text-gray-500">{user.name}</p>
          )}
        </div>
      ),
    },
    {
      key: 'roles',
      header: 'Roles',
      render: (user) => (
        <div className="flex flex-wrap gap-1">
          {user.roles.map((role) => (
            <span
              key={role}
              className="px-2 py-0.5 text-xs font-medium rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
            >
              {role}
            </span>
          ))}
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (user) => (
        <StatusBadge status={user.status === 'active' ? 'success' : 'neutral'}>
          {user.status}
        </StatusBadge>
      ),
    },
  ];

  const modelAccessColumns: Column<ModelAccess>[] = [
    {
      key: 'model_name',
      header: 'Model',
      sortable: true,
      render: (m) => <span className="font-medium">{m.model_name}</span>,
    },
    {
      key: 'granted',
      header: 'Access',
      render: (m) => (
        <StatusBadge status={m.granted ? 'success' : 'error'}>
          {m.granted ? 'Granted' : 'Denied'}
        </StatusBadge>
      ),
    },
    {
      key: 'granted_at',
      header: 'Granted At',
      render: (m) =>
        m.granted_at ? new Date(m.granted_at).toLocaleDateString() : '-',
    },
    {
      key: 'granted_by',
      header: 'Granted By',
      render: (m) => m.granted_by || '-',
    },
  ];

  const renderUserActions = (user: User) => (
    <Button
      size="sm"
      variant={selectedUser?.id === user.id ? 'primary' : 'secondary'}
      onClick={() => setSelectedUser(user)}
    >
      {selectedUser?.id === user.id ? 'Selected' : 'Select'}
    </Button>
  );

  const renderModelActions = (model: ModelAccess) => (
    <Button
      size="sm"
      variant="ghost"
      className="text-red-600 hover:text-red-700"
      onClick={() => {
        setSelectedModel(model);
        setShowRevokeModal(true);
      }}
      disabled={!model.granted}
    >
      Revoke
    </Button>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="User Model Access"
        description="Manage which models users can access"
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* User Selection */}
        <Card>
          <CardHeader title="Select User" />
          <CardContent noPadding>
            <DataTable
              columns={userColumns}
              data={users}
              keyExtractor={(u) => u.id}
              loading={usersLoading}
              emptyMessage="No users found."
              actions={renderUserActions}
            />
          </CardContent>
        </Card>

        {/* Model Access */}
        <Card>
          <CardHeader
            title={selectedUser ? `Access: ${selectedUser.email}` : 'Model Access'}
            actions={
              selectedUser && (
                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => {
                      setNewMode(accessInfo?.access_mode || 'allowlist');
                      setShowModeModal(true);
                    }}
                  >
                    Mode: {accessInfo?.access_mode || 'N/A'}
                  </Button>
                  <Button
                    size="sm"
                    onClick={() => setShowGrantModal(true)}
                  >
                    Grant Access
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => grantAllMutation.mutate(selectedUser.id)}
                    loading={grantAllMutation.isPending}
                  >
                    Grant All
                  </Button>
                </div>
              )
            }
          />
          <CardContent noPadding>
            {selectedUser ? (
              <DataTable
                columns={modelAccessColumns}
                data={accessInfo?.models || []}
                keyExtractor={(m) => m.model_name}
                loading={accessLoading}
                emptyMessage="No model access configured."
                actions={renderModelActions}
              />
            ) : (
              <div className="p-8 text-center text-gray-500">
                Select a user to view their model access
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Grant Modal */}
      <Modal
        isOpen={showGrantModal}
        onClose={() => {
          setShowGrantModal(false);
          setModelToGrant('');
        }}
        title={`Grant Model Access: ${selectedUser?.email}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowGrantModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() =>
                selectedUser &&
                grantMutation.mutate({ userId: selectedUser.id, modelName: modelToGrant })
              }
              loading={grantMutation.isPending}
              disabled={!modelToGrant}
            >
              Grant Access
            </Button>
          </>
        }
      >
        <Select
          label="Model"
          value={modelToGrant}
          onChange={(e) => setModelToGrant(e.target.value)}
        >
          <option value="">Select a model</option>
          {availableModels.map((model) => (
            <option key={model} value={model}>
              {model}
            </option>
          ))}
        </Select>
        <Input
          label="Or enter model name"
          value={modelToGrant}
          onChange={(e) => setModelToGrant(e.target.value)}
          className="mt-4"
          hint="Enter a model name if not in the list above"
        />
      </Modal>

      {/* Mode Modal */}
      <Modal
        isOpen={showModeModal}
        onClose={() => setShowModeModal(false)}
        title={`Set Access Mode: ${selectedUser?.email}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModeModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() =>
                selectedUser &&
                setModeMutation.mutate({ userId: selectedUser.id, mode: newMode })
              }
              loading={setModeMutation.isPending}
            >
              Set Mode
            </Button>
          </>
        }
      >
        <Select
          label="Access Mode"
          value={newMode}
          onChange={(e) => setNewMode(e.target.value as 'allowlist' | 'denylist' | 'all')}
        >
          <option value="allowlist">Allowlist - Only granted models accessible</option>
          <option value="denylist">Denylist - All models except denied</option>
          <option value="all">All - Access to all models</option>
        </Select>
      </Modal>

      {/* Revoke Confirmation */}
      <ConfirmModal
        isOpen={showRevokeModal}
        onClose={() => {
          setShowRevokeModal(false);
          setSelectedModel(null);
        }}
        onConfirm={() =>
          selectedUser &&
          selectedModel &&
          revokeMutation.mutate({ userId: selectedUser.id, modelName: selectedModel.model_name })
        }
        title="Revoke Model Access"
        message={`Are you sure you want to revoke access to "${selectedModel?.model_name}" for ${selectedUser?.email}?`}
        confirmLabel="Revoke"
        variant="danger"
        loading={revokeMutation.isPending}
      />

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># Show user model access</p>
            <p className="text-primary">ai-aas-cli user model-access show {'<user>'}</p>
            <p className="text-gray-500 mt-3"># Set access mode</p>
            <p className="text-primary">ai-aas-cli user model-access set-mode {'<user> <allowlist|denylist|all>'}</p>
            <p className="text-gray-500 mt-3"># Grant model access</p>
            <p className="text-primary">ai-aas-cli user model-access grant {'<user> <model>'}</p>
            <p className="text-gray-500 mt-3"># Revoke model access</p>
            <p className="text-primary">ai-aas-cli user model-access revoke {'<user> <model>'}</p>
            <p className="text-gray-500 mt-3"># List user's model access</p>
            <p className="text-primary">ai-aas-cli user model-access list {'<user>'}</p>
            <p className="text-gray-500 mt-3"># Grant access to all models</p>
            <p className="text-primary">ai-aas-cli user model-access grant-all {'<user>'}</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
