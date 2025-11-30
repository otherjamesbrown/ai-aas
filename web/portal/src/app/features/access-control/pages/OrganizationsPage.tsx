import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { accessControlApi, Organization, CreateOrgRequest, UpdateOrgRequest } from '../api/accessControl';
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
  ConfirmModal,
} from '@/components/ui';
import type { Column } from '@/components/ui';

/**
 * OrganizationsPage - Manage organizations
 *
 * CLI equivalents:
 * - ai-aas-cli org list
 * - ai-aas-cli org create
 * - ai-aas-cli org update
 * - ai-aas-cli org delete
 */
export default function OrganizationsPage() {
  const queryClient = useQueryClient();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [selectedOrg, setSelectedOrg] = useState<Organization | null>(null);

  // Form state
  const [orgName, setOrgName] = useState('');
  const [orgSlug, setOrgSlug] = useState('');

  // Fetch organizations
  const { data: organizations = [], isLoading } = useQuery({
    queryKey: ['organizations'],
    queryFn: () => accessControlApi.orgList(),
  });

  // Create mutation
  const createMutation = useMutation({
    mutationFn: (request: CreateOrgRequest) => accessControlApi.orgCreate(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      setShowCreateModal(false);
      resetForm();
    },
  });

  // Update mutation
  const updateMutation = useMutation({
    mutationFn: ({ id, request }: { id: string; request: UpdateOrgRequest }) =>
      accessControlApi.orgUpdate(id, request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      setShowEditModal(false);
      setSelectedOrg(null);
      resetForm();
    },
  });

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (id: string) => accessControlApi.orgDelete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      setShowDeleteModal(false);
      setSelectedOrg(null);
    },
  });

  const resetForm = () => {
    setOrgName('');
    setOrgSlug('');
  };

  const handleEdit = (org: Organization) => {
    setSelectedOrg(org);
    setOrgName(org.name);
    setShowEditModal(true);
  };

  const columns: Column<Organization>[] = [
    {
      key: 'name',
      header: 'Name',
      sortable: true,
      render: (org) => (
        <span className="font-medium text-gray-900 dark:text-white">{org.name}</span>
      ),
    },
    {
      key: 'slug',
      header: 'Slug',
      render: (org) => (
        <code className="text-sm text-gray-600 dark:text-gray-400">{org.slug}</code>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (org) => (
        <StatusBadge
          status={org.status === 'active' ? 'success' : org.status === 'trial' ? 'info' : 'warning'}
        >
          {org.status}
        </StatusBadge>
      ),
    },
    {
      key: 'created_at',
      header: 'Created',
      sortable: true,
      render: (org) => new Date(org.created_at).toLocaleDateString(),
    },
  ];

  const renderActions = (org: Organization) => (
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
        onClick={() => handleEdit(org)}
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
          setSelectedOrg(org);
          setShowDeleteModal(true);
        }}
      />
    </div>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="Organizations"
        description="Manage organizations on the platform"
        actions={
          <Button
            onClick={() => setShowCreateModal(true)}
            leftIcon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
            }
          >
            Create Organization
          </Button>
        }
      />

      <Card>
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={organizations}
            keyExtractor={(org) => org.id}
            loading={isLoading}
            emptyMessage="No organizations found."
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
        title="Create Organization"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowCreateModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => createMutation.mutate({ name: orgName, slug: orgSlug || undefined })}
              loading={createMutation.isPending}
              disabled={!orgName}
            >
              Create
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Organization Name"
            value={orgName}
            onChange={(e) => setOrgName(e.target.value)}
            required
          />
          <Input
            label="Slug (optional)"
            value={orgSlug}
            onChange={(e) => setOrgSlug(e.target.value)}
            hint="URL-friendly identifier. Auto-generated if not provided."
          />
        </div>
      </Modal>

      {/* Edit Modal */}
      <Modal
        isOpen={showEditModal}
        onClose={() => {
          setShowEditModal(false);
          setSelectedOrg(null);
          resetForm();
        }}
        title="Edit Organization"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowEditModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() =>
                selectedOrg &&
                updateMutation.mutate({ id: selectedOrg.id, request: { name: orgName } })
              }
              loading={updateMutation.isPending}
              disabled={!orgName}
            >
              Save
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Organization Name"
            value={orgName}
            onChange={(e) => setOrgName(e.target.value)}
            required
          />
        </div>
      </Modal>

      {/* Delete Confirmation */}
      <ConfirmModal
        isOpen={showDeleteModal}
        onClose={() => {
          setShowDeleteModal(false);
          setSelectedOrg(null);
        }}
        onConfirm={() => selectedOrg && deleteMutation.mutate(selectedOrg.id)}
        title="Delete Organization"
        message={`Are you sure you want to delete "${selectedOrg?.name}"? This will remove all associated users and data.`}
        confirmLabel="Delete"
        variant="danger"
        loading={deleteMutation.isPending}
      />
    </AdminLayout>
  );
}
