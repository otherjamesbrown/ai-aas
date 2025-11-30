import { useQuery } from '@tanstack/react-query';
import { platformApi } from '../api/platform';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  Card,
  CardHeader,
  CardContent,
  StatusBadge,
  StatsCard,
} from '@/components/ui';

/**
 * PlatformStatusPage - View platform health and status
 *
 * CLI equivalent: ai-aas-cli status
 */
export default function PlatformStatusPage() {
  // Fetch platform status
  const { data: status, isLoading, refetch } = useQuery({
    queryKey: ['platform-status'],
    queryFn: () => platformApi.getStatus(),
    refetchInterval: 30000, // Refresh every 30 seconds
  });

  // Fetch readiness
  const { data: readiness } = useQuery({
    queryKey: ['platform-readiness'],
    queryFn: () => platformApi.getReadiness(),
    refetchInterval: 30000,
  });

  const getComponentStatus = (componentStatus: string) => {
    if (componentStatus === 'ready' || componentStatus === 'healthy') return 'success';
    if (componentStatus === 'degraded') return 'warning';
    if (componentStatus === 'not_configured') return 'neutral';
    return 'error';
  };

  const healthyServices = status?.services
    ? Object.values(status.services).filter((s) => s.status === 'healthy').length
    : 0;
  const totalServices = status?.services ? Object.keys(status.services).length : 0;

  return (
    <AdminLayout>
      <PageHeader
        title="Platform Status"
        description="Monitor platform health and service status"
        actions={
          <Button
            variant="secondary"
            onClick={() => refetch()}
            leftIcon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            }
          >
            Refresh
          </Button>
        }
      />

      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
        </div>
      ) : (
        <>
          {/* Summary Stats */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
            <StatsCard
              title="Overall Status"
              value={status?.overall || 'Unknown'}
              icon={
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              }
            />
            <StatsCard
              title="Services"
              value={`${healthyServices}/${totalServices}`}
              subtitle="Healthy"
              icon={
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                </svg>
              }
            />
            <StatsCard
              title="Components"
              value={readiness?.components ? Object.keys(readiness.components).length : 0}
              subtitle="Configured"
              icon={
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
                </svg>
              }
            />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Services */}
            <Card>
              <CardHeader title="Services" />
              <CardContent>
                {status?.services && Object.keys(status.services).length > 0 ? (
                  <div className="space-y-3">
                    {Object.entries(status.services).map(([name, service]) => (
                      <div
                        key={name}
                        className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
                      >
                        <div>
                          <p className="font-medium capitalize">
                            {name.replace(/_/g, ' ')}
                          </p>
                          {service.message && (
                            <p className="text-sm text-gray-500 dark:text-gray-400">
                              {service.message}
                            </p>
                          )}
                        </div>
                        <div className="flex items-center space-x-3">
                          {service.response_ms && (
                            <span className="text-sm text-gray-500 dark:text-gray-400">
                              {service.response_ms}ms
                            </span>
                          )}
                          <StatusBadge status={getComponentStatus(service.status)}>
                            {service.status}
                          </StatusBadge>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-gray-500 dark:text-gray-400 text-center py-4">
                    No services available
                  </p>
                )}
              </CardContent>
            </Card>

            {/* Components */}
            <Card>
              <CardHeader title="Components" />
              <CardContent>
                {readiness?.components && Object.keys(readiness.components).length > 0 ? (
                  <div className="space-y-3">
                    {Object.entries(readiness.components).map(([name, component]) => {
                      const componentData = typeof component === 'string'
                        ? { status: component }
                        : component;

                      // Skip not_configured components
                      if (componentData.status === 'not_configured') return null;

                      return (
                        <div
                          key={name}
                          className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
                        >
                          <div>
                            <p className="font-medium capitalize">
                              {name.replace(/_/g, ' ')}
                            </p>
                            {componentData.message && (
                              <p className="text-sm text-gray-500 dark:text-gray-400">
                                {componentData.message}
                              </p>
                            )}
                          </div>
                          <StatusBadge status={getComponentStatus(componentData.status)}>
                            {componentData.status}
                          </StatusBadge>
                        </div>
                      );
                    })}
                  </div>
                ) : (
                  <p className="text-gray-500 dark:text-gray-400 text-center py-4">
                    No components available
                  </p>
                )}
              </CardContent>
            </Card>
          </div>

          {/* CLI Reference */}
          <Card className="mt-6">
            <CardHeader title="CLI Commands" />
            <CardContent>
              <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
                <p className="text-gray-500"># Check overall platform status</p>
                <p className="text-primary">ai-aas-cli status</p>
                <p className="text-gray-500 mt-3"># Check health endpoint</p>
                <p className="text-primary">ai-aas-cli status --health</p>
                <p className="text-gray-500 mt-3"># Check readiness endpoint</p>
                <p className="text-primary">ai-aas-cli status --ready</p>
              </div>
            </CardContent>
          </Card>
        </>
      )}
    </AdminLayout>
  );
}
