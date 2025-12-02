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
  Select,
  ConfirmModal,
} from '@/components/ui';
import type { Column } from '@/components/ui';

interface ServiceEntry {
  name: string;
  type: string;
  endpoint: string;
  status: 'enabled' | 'disabled';
  health: 'healthy' | 'unhealthy' | 'unknown';
  registered_at: string;
}

/**
 * RegistryPage - Service registry management
 *
 * CLI equivalents:
 * - ai-aas-cli registry list
 * - ai-aas-cli registry register
 * - ai-aas-cli registry deregister
 * - ai-aas-cli registry enable
 * - ai-aas-cli registry disable
 */
export default function RegistryPage() {
  const queryClient = useQueryClient();
  const [showRegisterModal, setShowRegisterModal] = useState(false);
  const [showDeregisterModal, setShowDeregisterModal] = useState(false);
  const [selectedService, setSelectedService] = useState<ServiceEntry | null>(null);
  const [registerForm, setRegisterForm] = useState({
    name: '',
    type: 'inference',
    endpoint: '',
  });

  // Fetch services
  const { data: services = [], isLoading } = useQuery({
    queryKey: ['registry-services'],
    queryFn: () => platformApi.registryList(),
  });

  // Register mutation
  const registerMutation = useMutation({
    mutationFn: (data: { name: string; type: string; endpoint: string }) =>
      platformApi.registryRegister(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registry-services'] });
      setShowRegisterModal(false);
      setRegisterForm({ name: '', type: 'inference', endpoint: '' });
    },
  });

  // Deregister mutation
  const deregisterMutation = useMutation({
    mutationFn: (name: string) => platformApi.registryDeregister(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registry-services'] });
      setShowDeregisterModal(false);
      setSelectedService(null);
    },
  });

  // Enable mutation
  const enableMutation = useMutation({
    mutationFn: (name: string) => platformApi.registryEnable(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registry-services'] });
    },
  });

  // Disable mutation
  const disableMutation = useMutation({
    mutationFn: (name: string) => platformApi.registryDisable(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registry-services'] });
    },
  });

  const columns: Column<ServiceEntry>[] = [
    {
      key: 'name',
      header: 'Service Name',
      sortable: true,
      render: (s) => <span className="font-medium">{s.name}</span>,
    },
    {
      key: 'type',
      header: 'Type',
      sortable: true,
      render: (s) => (
        <span className="px-2 py-0.5 text-xs rounded bg-gray-100 dark:bg-gray-700">
          {s.type}
        </span>
      ),
    },
    {
      key: 'endpoint',
      header: 'Endpoint',
      render: (s) => (
        <code className="text-xs text-gray-600 dark:text-gray-400">
          {s.endpoint}
        </code>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (s) => (
        <StatusBadge status={s.status === 'enabled' ? 'success' : 'neutral'}>
          {s.status}
        </StatusBadge>
      ),
    },
    {
      key: 'health',
      header: 'Health',
      render: (s) => (
        <StatusBadge
          status={
            s.health === 'healthy'
              ? 'success'
              : s.health === 'unhealthy'
              ? 'error'
              : 'warning'
          }
        >
          {s.health}
        </StatusBadge>
      ),
    },
    {
      key: 'registered_at',
      header: 'Registered',
      sortable: true,
      render: (s) => new Date(s.registered_at).toLocaleDateString(),
    },
  ];

  const renderActions = (service: ServiceEntry) => (
    <div className="flex items-center gap-2">
      {service.status === 'enabled' ? (
        <Button
          size="sm"
          variant="secondary"
          onClick={() => disableMutation.mutate(service.name)}
          loading={disableMutation.isPending}
        >
          Disable
        </Button>
      ) : (
        <Button
          size="sm"
          variant="secondary"
          onClick={() => enableMutation.mutate(service.name)}
          loading={enableMutation.isPending}
        >
          Enable
        </Button>
      )}
      <Button
        size="sm"
        variant="ghost"
        className="text-red-600 hover:text-red-700"
        onClick={() => {
          setSelectedService(service);
          setShowDeregisterModal(true);
        }}
      >
        Deregister
      </Button>
    </div>
  );

  return (
    <AdminLayout>
      <PageHeader
        title="Service Registry"
        description="Manage registered services and their status"
        actions={
          <Button onClick={() => setShowRegisterModal(true)}>
            Register Service
          </Button>
        }
      />

      <Card>
        <CardHeader
          title="Registered Services"
          actions={
            <span className="text-sm text-gray-500">
              {services.length} service{services.length !== 1 ? 's' : ''}
            </span>
          }
        />
        <CardContent noPadding>
          <DataTable
            columns={columns}
            data={services}
            keyExtractor={(s) => s.name}
            loading={isLoading}
            emptyMessage="No services registered."
            actions={renderActions}
          />
        </CardContent>
      </Card>

      {/* Register Modal */}
      <Modal
        isOpen={showRegisterModal}
        onClose={() => {
          setShowRegisterModal(false);
          setRegisterForm({ name: '', type: 'inference', endpoint: '' });
        }}
        title="Register Service"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowRegisterModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => registerMutation.mutate(registerForm)}
              loading={registerMutation.isPending}
              disabled={!registerForm.name || !registerForm.endpoint}
            >
              Register
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Service Name"
            value={registerForm.name}
            onChange={(e) => setRegisterForm({ ...registerForm, name: e.target.value })}
            hint="Unique identifier for the service"
          />
          <Select
            label="Service Type"
            value={registerForm.type}
            onChange={(e) => setRegisterForm({ ...registerForm, type: e.target.value })}
          >
            <option value="inference">Inference</option>
            <option value="storage">Storage</option>
            <option value="cache">Cache</option>
            <option value="monitoring">Monitoring</option>
            <option value="other">Other</option>
          </Select>
          <Input
            label="Endpoint URL"
            value={registerForm.endpoint}
            onChange={(e) => setRegisterForm({ ...registerForm, endpoint: e.target.value })}
            hint="HTTP(S) endpoint for the service"
            placeholder="http://service:8080"
          />
        </div>
      </Modal>

      {/* Deregister Confirmation */}
      <ConfirmModal
        isOpen={showDeregisterModal}
        onClose={() => {
          setShowDeregisterModal(false);
          setSelectedService(null);
        }}
        onConfirm={() => selectedService && deregisterMutation.mutate(selectedService.name)}
        title="Deregister Service"
        message={`Are you sure you want to deregister "${selectedService?.name}"? This will remove it from the service registry.`}
        confirmLabel="Deregister"
        variant="danger"
        loading={deregisterMutation.isPending}
      />

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># List registered services</p>
            <p className="text-primary">ai-aas-cli registry list</p>
            <p className="text-gray-500 mt-3"># Register a service</p>
            <p className="text-primary">ai-aas-cli registry register --name {'<name>'} --type {'<type>'} --endpoint {'<url>'}</p>
            <p className="text-gray-500 mt-3"># Deregister a service</p>
            <p className="text-primary">ai-aas-cli registry deregister {'<name>'}</p>
            <p className="text-gray-500 mt-3"># Enable/disable a service</p>
            <p className="text-primary">ai-aas-cli registry enable {'<name>'}</p>
            <p className="text-primary">ai-aas-cli registry disable {'<name>'}</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
