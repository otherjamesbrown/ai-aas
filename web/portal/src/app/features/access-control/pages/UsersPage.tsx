import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { accessControlApi, User, CreateUserRequest } from '../api/accessControl';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  DataTable,
  StatusBadge,
  Modal,
  Input,
  Select,
  Card,
  CardContent,
  IconButton,
  ConfirmModal,
  FilterPanel,
  FilterSelect,
} from '@/components/ui';
import type { Column } from '@/components/ui';

const AVAILABLE_ROLES = [
  { value: 'admin', label: 'Admin' },
  { value: 'user', label: 'User' },
  { value: 'viewer', label: 'Viewer' },
];

/**
 * UsersPage - Manage users
 *
 * CLI equivalents:
 * - ai-aas-cli user list
 * - ai-aas-cli user create
 * - ai-aas-cli user update
 * - ai-aas-cli user delete
 */
export default function UsersPage() {
  const queryClient = useQueryClient();
  const [organizationFilter, setOrganizationFilter] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);

  // Form state
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [organizationId, setOrganizationId] = useState('');
  const [selectedRoles, setSelectedRoles] = useState<string[]>(['user']);

  // Fetch users
  const { data: users = [], isLoading } = useQuery({
    queryKey: ['users', organizationFilter],
    queryFn: () => accessControlApi.userList(organizationFilter || undefined),
  });

  // Fetch organizations for filter and dropdown
  const { data: organizations = [] } = useQuery({
    queryKey: ['organizations'],
    queryFn: () => accessControlApi.orgList(),
  });

  // Create mutation
  const createMutation = useMutation({
    mutationFn: (request: CreateUserRequest) => accessControlApi.userCreate(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setShowCreateModal(false);
      resetForm();
    },
  });

  // Update mutation
  const updateMutation = useMutation({
    mutationFn: ({ id, request }: { id: string; request: { name?: string; roles?: string[] } }) =>
      accessControlApi.userUpdate(id, request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setShowEditModal(false);
      setSelectedUser(null);
      resetForm();
    },
  });

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (id: string) => accessControlApi.userDelete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setShowDeleteModal(false);
      setSelectedUser(null);
    },
  });

  const resetForm = () => {
    setEmail('');
    setName('');
    setOrganizationId('');
    setSelectedRoles(['user']);
  };

  const handleEdit = (user: User) => {
    setSelectedUser(user);
    setName(user.name || '');
    setSelectedRoles(user.roles);
    setShowEditModal(true);
  };

  const toggleRole = (role: string) => {
    if (selectedRoles.includes(role)) {
      setSelectedRoles(selectedRoles.filter((r) => r !== role));
    } else {
      setSelectedRoles([...selectedRoles, role]);
    }
  };

  const columns: Column<User>[] = [
    {
      key: 'email',
      header: 'Email',
      sortable: true,
      render: (user) => (
        <div>
          <span className="font-medium text-gray-900 dark:text-white">{user.email}</span>
          {user.name && (
            <p className="text-sm text-gray-500 dark:text-gray-400">{user.name}</p>
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
        <StatusBadge
          status={user.status === 'active' ? 'success' : user.status === 'pending' ? 'warning' : 'error'}
        >
          {user.status}
        </StatusBadge>
      ),
    },
    {
      key: 'last_login_at',
      header: 'Last Login',
      sortable: true,
      render: (user) =>
        user.last_login_at ? new Date(user.last_login_at).toLocaleDateString() : 'Never',
    },
    {
      key: 'created_at',
      header: 'Created',
      sortable: true,
      render: (user) => new Date(user.created_at).toLocaleDateString(),
    },
  ];

  const renderActions = (user: User) => (
    <div className="flex items-center space-x-1">
      <IconButton
        icon={
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
          </svg>
        }
        label="Edit"
        variant="ghost"
        size="sm"
        onClick={() => handleEdit(user)}
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
          setSelectedUser(user);
          setShowDeleteModal(true);
        }}
      />
    </div>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="Users"
        description="Manage platform users"
        actions={
          <Button
            onClick={() => setShowCreateModal(true)}
            leftIcon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
            }
          >
            Create User
          </Button>
        }
      />

      {/* Filters */}
      <FilterPanel onReset={() => setOrganizationFilter('')} className="mb-6">
        <FilterSelect
          label="Organization"
          value={organizationFilter}
          options={organizations.map((org) => ({ value: org.id, label: org.name }))}
          onChange={setOrganizationFilter}
        />
      </FilterPanel>

      <Card>
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={users}
            keyExtractor={(user) => user.id}
            loading={isLoading}
            emptyMessage="No users found."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* Create Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => {
          setShowCreateModal(false);
          resetForm();
        }}
        title="Create User"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowCreateModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() =>
                createMutation.mutate({
                  email,
                  name: name || undefined,
                  organization_id: organizationId,
                  roles: selectedRoles,
                })
              }
              loading={createMutation.isPending}
              disabled={!email || !organizationId}
            >
              Create
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          <Input
            label="Name (optional)"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <Select
            label="Organization"
            value={organizationId}
            onChange={(e) => setOrganizationId(e.target.value)}
            required
          >
            <option value="">Select an organization</option>
            {organizations.map((org) => (
              <option key={org.id} value={org.id}>
                {org.name}
              </option>
            ))}
          </Select>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Roles
            </label>
            <div className="flex flex-wrap gap-2">
              {AVAILABLE_ROLES.map((role) => (
                <button
                  key={role.value}
                  type="button"
                  onClick={() => toggleRole(role.value)}
                  className={`px-3 py-1 text-sm rounded-full transition-colors ${
                    selectedRoles.includes(role.value)
                      ? 'bg-primary text-white'
                      : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                  }`}
                >
                  {role.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      </Modal>

      {/* Edit Modal */}
      <Modal
        isOpen={showEditModal}
        onClose={() => {
          setShowEditModal(false);
          setSelectedUser(null);
          resetForm();
        }}
        title="Edit User"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowEditModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() =>
                selectedUser &&
                updateMutation.mutate({
                  id: selectedUser.id,
                  request: { name: name || undefined, roles: selectedRoles },
                })
              }
              loading={updateMutation.isPending}
            >
              Save
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Email"
            value={selectedUser?.email || ''}
            disabled
          />
          <Input
            label="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Roles
            </label>
            <div className="flex flex-wrap gap-2">
              {AVAILABLE_ROLES.map((role) => (
                <button
                  key={role.value}
                  type="button"
                  onClick={() => toggleRole(role.value)}
                  className={`px-3 py-1 text-sm rounded-full transition-colors ${
                    selectedRoles.includes(role.value)
                      ? 'bg-primary text-white'
                      : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                  }`}
                >
                  {role.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      </Modal>

      {/* Delete Confirmation */}
      <ConfirmModal
        isOpen={showDeleteModal}
        onClose={() => {
          setShowDeleteModal(false);
          setSelectedUser(null);
        }}
        onConfirm={() => selectedUser && deleteMutation.mutate(selectedUser.id)}
        title="Delete User"
        message={`Are you sure you want to delete "${selectedUser?.email}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="danger"
        loading={deleteMutation.isPending}
      />
    </AdminLayout>
  );
}
