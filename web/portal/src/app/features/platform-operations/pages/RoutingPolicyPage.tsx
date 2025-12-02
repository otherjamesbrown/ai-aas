import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { platformApi } from '../api/platform';
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

interface RoutingPolicy {
  id: string;
  name: string;
  priority: number;
  conditions: Record<string, string>;
  target: string;
  enabled: boolean;
  created_at: string;
}

/**
 * RoutingPolicyPage - Routing policy management
 *
 * CLI equivalents:
 * - ai-aas-cli routing policy list
 * - ai-aas-cli routing policy create
 * - ai-aas-cli routing policy delete
 */
export default function RoutingPolicyPage() {
  const queryClient = useQueryClient();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [selectedPolicy, setSelectedPolicy] = useState<RoutingPolicy | null>(null);
  const [createForm, setCreateForm] = useState({
    name: '',
    priority: 100,
    conditions: '',
    target: '',
  });

  // Fetch policies
  const { data: policies = [], isLoading } = useQuery({
    queryKey: ['routing-policies'],
    queryFn: () => platformApi.routingPolicyList(),
  });

  // Create mutation
  const createMutation = useMutation({
    mutationFn: (data: { name: string; priority: number; conditions: Record<string, string>; target: string }) =>
      platformApi.routingPolicyCreate(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['routing-policies'] });
      setShowCreateModal(false);
      setCreateForm({ name: '', priority: 100, conditions: '', target: '' });
    },
  });

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (id: string) => platformApi.routingPolicyDelete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['routing-policies'] });
      setShowDeleteModal(false);
      setSelectedPolicy(null);
    },
  });

  const handleCreate = () => {
    let conditions: Record<string, string> = {};
    try {
      // Parse conditions from JSON string
      if (createForm.conditions.trim()) {
        conditions = JSON.parse(createForm.conditions);
      }
    } catch {
      // Invalid JSON - show error
      return;
    }

    createMutation.mutate({
      name: createForm.name,
      priority: createForm.priority,
      conditions,
      target: createForm.target,
    });
  };

  const columns: Column<RoutingPolicy>[] = [
    {
      key: 'name',
      header: 'Policy Name',
      sortable: true,
      render: (p) => <span className="font-medium">{p.name}</span>,
    },
    {
      key: 'priority',
      header: 'Priority',
      sortable: true,
      render: (p) => (
        <span className="px-2 py-0.5 text-xs bg-gray-100 dark:bg-gray-700 rounded">
          {p.priority}
        </span>
      ),
    },
    {
      key: 'conditions',
      header: 'Conditions',
      render: (p) => (
        <code className="text-xs text-gray-600 dark:text-gray-400">
          {Object.entries(p.conditions).map(([k, v]) => `${k}=${v}`).join(', ') || 'None'}
        </code>
      ),
    },
    {
      key: 'target',
      header: 'Target',
      render: (p) => (
        <span className="text-sm text-gray-700 dark:text-gray-300">{p.target}</span>
      ),
    },
    {
      key: 'enabled',
      header: 'Status',
      render: (p) => (
        <StatusBadge status={p.enabled ? 'success' : 'neutral'}>
          {p.enabled ? 'Enabled' : 'Disabled'}
        </StatusBadge>
      ),
    },
    {
      key: 'created_at',
      header: 'Created',
      sortable: true,
      render: (p) => new Date(p.created_at).toLocaleDateString(),
    },
  ];

  const renderActions = (policy: RoutingPolicy) => (
    <Button
      size="sm"
      variant="ghost"
      className="text-red-600 hover:text-red-700"
      onClick={() => {
        setSelectedPolicy(policy);
        setShowDeleteModal(true);
      }}
    >
      Delete
    </Button>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="Routing Policies"
        description="Manage request routing policies"
        actions={
          <Button onClick={() => setShowCreateModal(true)}>
            Create Policy
          </Button>
        }
      />

      <Card>
        <CardHeader
          title="Active Policies"
          actions={
            <span className="text-sm text-gray-500">
              {policies.length} polic{policies.length !== 1 ? 'ies' : 'y'}
            </span>
          }
        />
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={policies}
            keyExtractor={(p) => p.id}
            loading={isLoading}
            emptyMessage="No routing policies configured."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* Create Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => {
          setShowCreateModal(false);
          setCreateForm({ name: '', priority: 100, conditions: '', target: '' });
        }}
        title="Create Routing Policy"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowCreateModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleCreate}
              loading={createMutation.isPending}
              disabled={!createForm.name || !createForm.target}
            >
              Create
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Policy Name"
            value={createForm.name}
            onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
            hint="Unique identifier for the policy"
          />
          <Input
            label="Priority"
            type="number"
            value={createForm.priority.toString()}
            onChange={(e) => setCreateForm({ ...createForm, priority: parseInt(e.target.value) || 0 })}
            hint="Lower numbers = higher priority"
          />
          <Input
            label="Conditions (JSON)"
            value={createForm.conditions}
            onChange={(e) => setCreateForm({ ...createForm, conditions: e.target.value })}
            hint='JSON object, e.g., {"model": "gpt-4", "org": "acme"}'
            placeholder='{"key": "value"}'
          />
          <Input
            label="Target"
            value={createForm.target}
            onChange={(e) => setCreateForm({ ...createForm, target: e.target.value })}
            hint="Target service or endpoint"
            placeholder="vllm-gpt-4"
          />
        </div>
      </Modal>

      {/* Delete Confirmation */}
      <ConfirmModal
        isOpen={showDeleteModal}
        onClose={() => {
          setShowDeleteModal(false);
          setSelectedPolicy(null);
        }}
        onConfirm={() => selectedPolicy && deleteMutation.mutate(selectedPolicy.id)}
        title="Delete Routing Policy"
        message={`Are you sure you want to delete the routing policy "${selectedPolicy?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="danger"
        loading={deleteMutation.isPending}
      />

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># List routing policies</p>
            <p className="text-primary">ai-aas-cli routing policy list</p>
            <p className="text-gray-500 mt-3"># Create a routing policy</p>
            <p className="text-primary">ai-aas-cli routing policy create --name {'<name>'} --priority {'<num>'} --target {'<target>'}</p>
            <p className="text-gray-500 mt-3"># Delete a routing policy</p>
            <p className="text-primary">ai-aas-cli routing policy delete {'<id>'}</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
