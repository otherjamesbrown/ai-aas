import { useState } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import { modelsApi } from '../api/models';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  Select,
  Card,
  CardHeader,
  CardContent,
  StatusBadge,
} from '@/components/ui';

/**
 * ModelTroubleshootPage - Troubleshoot model deployments
 *
 * CLI equivalents:
 * - ai-aas-cli model troubleshoot logs <deployment>
 * - ai-aas-cli model troubleshoot events <deployment>
 * - ai-aas-cli model troubleshoot test-inference <deployment>
 */
export default function ModelTroubleshootPage() {
  const [selectedDeployment, setSelectedDeployment] = useState('');
  const [logLines, setLogLines] = useState(100);

  // Fetch deployments for selection
  const { data: deployments = [] } = useQuery({
    queryKey: ['deployments'],
    queryFn: () => modelsApi.deployList(),
  });

  // Fetch logs
  const { data: logs, isLoading: logsLoading, refetch: refetchLogs } = useQuery({
    queryKey: ['deployment-logs', selectedDeployment, logLines],
    queryFn: () => modelsApi.troubleshootLogs(selectedDeployment, logLines),
    enabled: !!selectedDeployment,
  });

  // Fetch events
  const { data: events, isLoading: eventsLoading } = useQuery({
    queryKey: ['deployment-events', selectedDeployment],
    queryFn: () => modelsApi.troubleshootEvents(selectedDeployment),
    enabled: !!selectedDeployment,
  });

  // Test inference mutation
  const testInferenceMutation = useMutation({
    mutationFn: () => modelsApi.troubleshootTestInference(selectedDeployment),
  });

  const handleTestInference = () => {
    testInferenceMutation.mutate();
  };

  return (
    <AdminLayout>
      <PageHeader
        title="Model Troubleshooting"
        description="Diagnose and troubleshoot model deployment issues"
      />

      {/* Deployment Selection */}
      <Card className="mb-6">
        <CardContent>
          <div className="flex items-end gap-4">
            <div className="flex-1">
              <Select
                label="Select Deployment"
                value={selectedDeployment}
                onChange={(e) => setSelectedDeployment(e.target.value)}
                placeholder="Choose a deployment to troubleshoot"
              >
                <option value="">Select a deployment</option>
                {deployments.map((d) => (
                  <option key={d.id} value={d.model_name}>
                    {d.model_name} ({d.environment})
                  </option>
                ))}
              </Select>
            </div>
            <Button
              variant="secondary"
              onClick={handleTestInference}
              disabled={!selectedDeployment}
              loading={testInferenceMutation.isPending}
            >
              Test Inference
            </Button>
          </div>

          {testInferenceMutation.data && (
            <div className="mt-4 p-3 bg-gray-50 dark:bg-gray-800 rounded-md">
              <p className="text-sm font-medium mb-2">Inference Test Result:</p>
              <StatusBadge status={testInferenceMutation.data.success ? 'success' : 'error'}>
                {testInferenceMutation.data.success ? 'Success' : 'Failed'}
              </StatusBadge>
              {testInferenceMutation.data.latency_ms && (
                <span className="ml-2 text-sm text-gray-500">
                  Latency: {testInferenceMutation.data.latency_ms}ms
                </span>
              )}
              {testInferenceMutation.data.error && (
                <p className="mt-2 text-sm text-red-600 dark:text-red-400">
                  {testInferenceMutation.data.error}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {selectedDeployment && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Logs */}
          <Card>
            <CardHeader
              title="Pod Logs"
              actions={
                <div className="flex items-center gap-2">
                  <select
                    value={logLines}
                    onChange={(e) => setLogLines(parseInt(e.target.value, 10))}
                    className="text-sm border rounded px-2 py-1 bg-white dark:bg-gray-800"
                  >
                    <option value={50}>50 lines</option>
                    <option value={100}>100 lines</option>
                    <option value={500}>500 lines</option>
                    <option value={1000}>1000 lines</option>
                  </select>
                  <Button variant="ghost" size="sm" onClick={() => refetchLogs()}>
                    Refresh
                  </Button>
                </div>
              }
            />
            <CardContent>
              {logsLoading ? (
                <div className="flex items-center justify-center py-8">
                  <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
                </div>
              ) : (
                <pre className="bg-gray-900 text-gray-100 p-4 rounded-md text-xs overflow-auto max-h-96 font-mono">
                  {logs?.logs || 'No logs available'}
                </pre>
              )}
            </CardContent>
          </Card>

          {/* Events */}
          <Card>
            <CardHeader title="Kubernetes Events" />
            <CardContent>
              {eventsLoading ? (
                <div className="flex items-center justify-center py-8">
                  <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
                </div>
              ) : events?.events && events.events.length > 0 ? (
                <div className="space-y-2 max-h-96 overflow-auto">
                  {events.events.map((event, index) => (
                    <div
                      key={index}
                      className={`p-3 rounded-md text-sm ${
                        event.type === 'Warning'
                          ? 'bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800'
                          : event.type === 'Error'
                            ? 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'
                            : 'bg-gray-50 dark:bg-gray-800'
                      }`}
                    >
                      <div className="flex items-center justify-between mb-1">
                        <span className="font-medium">{event.reason}</span>
                        <span className="text-xs text-gray-500">
                          {event.last_timestamp
                            ? new Date(event.last_timestamp).toLocaleString()
                            : 'N/A'}
                        </span>
                      </div>
                      <p className="text-gray-600 dark:text-gray-400">{event.message}</p>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-center text-gray-500 py-8">No events found</p>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># View pod logs</p>
            <p className="text-primary">ai-aas-cli model troubleshoot logs {'<deployment>'}</p>
            <p className="text-gray-500 mt-3"># View Kubernetes events</p>
            <p className="text-primary">ai-aas-cli model troubleshoot events {'<deployment>'}</p>
            <p className="text-gray-500 mt-3"># Test inference endpoint</p>
            <p className="text-primary">ai-aas-cli model troubleshoot test-inference {'<deployment>'}</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
