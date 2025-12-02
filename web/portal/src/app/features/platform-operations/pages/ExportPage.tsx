import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { platformApi } from '../api/platform';
import { accessControlApi } from '../../access-control/api/accessControl';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  Card,
  CardHeader,
  CardContent,
  Input,
  Select,
  StatusBadge,
} from '@/components/ui';

/**
 * ExportPage - Data export functionality
 *
 * CLI equivalents:
 * - ai-aas-cli export usage
 * - ai-aas-cli export memberships
 */
export default function ExportPage() {
  const [usageParams, setUsageParams] = useState({
    start_date: '',
    end_date: '',
    format: 'csv' as 'json' | 'csv',
  });
  const [membershipParams, setMembershipParams] = useState({
    org_id: '',
    format: 'csv' as 'json' | 'csv',
  });

  // Fetch organizations for dropdown
  const { data: organizations = [] } = useQuery({
    queryKey: ['organizations'],
    queryFn: () => accessControlApi.orgList(),
  });

  // Export usage mutation
  const exportUsageMutation = useMutation({
    mutationFn: () => platformApi.exportUsage(usageParams),
    onSuccess: (blob) => {
      downloadBlob(blob, `usage-export.${usageParams.format}`);
    },
  });

  // Export memberships mutation
  const exportMembershipsMutation = useMutation({
    mutationFn: () => platformApi.exportMemberships(membershipParams),
    onSuccess: (blob) => {
      downloadBlob(blob, `memberships-export.${membershipParams.format}`);
    },
  });

  const downloadBlob = (blob: Blob, filename: string) => {
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
  };

  return (
    <AdminLayout>
      <PageHeader
        title="Data Export"
        description="Export platform data for analysis and reporting"
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Usage Export */}
        <Card>
          <CardHeader title="Usage Data Export" />
          <CardContent>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
              Export API usage metrics and statistics for a specified date range.
            </p>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <Input
                  label="Start Date"
                  type="date"
                  value={usageParams.start_date}
                  onChange={(e) => setUsageParams({ ...usageParams, start_date: e.target.value })}
                />
                <Input
                  label="End Date"
                  type="date"
                  value={usageParams.end_date}
                  onChange={(e) => setUsageParams({ ...usageParams, end_date: e.target.value })}
                />
              </div>
              <Select
                label="Export Format"
                value={usageParams.format}
                onChange={(e) => setUsageParams({ ...usageParams, format: e.target.value as 'json' | 'csv' })}
              >
                <option value="csv">CSV</option>
                <option value="json">JSON</option>
              </Select>
              <Button
                onClick={() => exportUsageMutation.mutate()}
                loading={exportUsageMutation.isPending}
                className="w-full"
              >
                Export Usage Data
              </Button>
              {exportUsageMutation.isSuccess && (
                <div className="flex items-center gap-2">
                  <StatusBadge status="success">Downloaded</StatusBadge>
                  <span className="text-sm text-gray-600 dark:text-gray-400">
                    Export file has been downloaded
                  </span>
                </div>
              )}
              {exportUsageMutation.isError && (
                <div className="flex items-center gap-2">
                  <StatusBadge status="error">Error</StatusBadge>
                  <span className="text-sm text-red-600 dark:text-red-400">
                    Failed to export usage data
                  </span>
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Memberships Export */}
        <Card>
          <CardHeader title="Memberships Export" />
          <CardContent>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
              Export organization membership data including users and roles.
            </p>
            <div className="space-y-4">
              <Select
                label="Organization (Optional)"
                value={membershipParams.org_id}
                onChange={(e) => setMembershipParams({ ...membershipParams, org_id: e.target.value })}
              >
                <option value="">All Organizations</option>
                {organizations.map((org) => (
                  <option key={org.id} value={org.id}>
                    {org.name}
                  </option>
                ))}
              </Select>
              <Select
                label="Export Format"
                value={membershipParams.format}
                onChange={(e) => setMembershipParams({ ...membershipParams, format: e.target.value as 'json' | 'csv' })}
              >
                <option value="csv">CSV</option>
                <option value="json">JSON</option>
              </Select>
              <Button
                onClick={() => exportMembershipsMutation.mutate()}
                loading={exportMembershipsMutation.isPending}
                className="w-full"
              >
                Export Memberships
              </Button>
              {exportMembershipsMutation.isSuccess && (
                <div className="flex items-center gap-2">
                  <StatusBadge status="success">Downloaded</StatusBadge>
                  <span className="text-sm text-gray-600 dark:text-gray-400">
                    Export file has been downloaded
                  </span>
                </div>
              )}
              {exportMembershipsMutation.isError && (
                <div className="flex items-center gap-2">
                  <StatusBadge status="error">Error</StatusBadge>
                  <span className="text-sm text-red-600 dark:text-red-400">
                    Failed to export memberships
                  </span>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Export History / Info */}
      <Card className="mt-6">
        <CardHeader title="Export Information" />
        <CardContent>
          <div className="space-y-4">
            <div className="p-4 bg-blue-50 dark:bg-blue-900/20 rounded-md">
              <h4 className="text-sm font-medium text-blue-700 dark:text-blue-400 mb-2">
                Usage Export
              </h4>
              <p className="text-sm text-blue-600 dark:text-blue-300">
                Includes: Request counts, token usage, latency metrics, error rates, and cost estimates per model/user/organization.
              </p>
            </div>
            <div className="p-4 bg-green-50 dark:bg-green-900/20 rounded-md">
              <h4 className="text-sm font-medium text-green-700 dark:text-green-400 mb-2">
                Memberships Export
              </h4>
              <p className="text-sm text-green-600 dark:text-green-300">
                Includes: User information, organization assignments, roles, API key counts, and status for each membership.
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># Export usage data</p>
            <p className="text-primary">ai-aas-cli export usage --start-date 2024-01-01 --end-date 2024-12-31 --format csv</p>
            <p className="text-gray-500 mt-3"># Export memberships</p>
            <p className="text-primary">ai-aas-cli export memberships --format csv</p>
            <p className="text-gray-500 mt-3"># Export memberships for specific org</p>
            <p className="text-primary">ai-aas-cli export memberships --org-id {'<org-id>'} --format json</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
