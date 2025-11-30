import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { platformApi } from '../api/platform';
import { AdminLayout, PageHeader } from '@/components/layout/AdminLayout';
import {
  Button,
  Card,
  CardHeader,
  CardContent,
  Input,
  StatusBadge,
} from '@/components/ui';

interface BootstrapFormData {
  admin_email: string;
  admin_password: string;
  org_name: string;
  hf_token?: string;
  skip_model_pull?: boolean;
}

/**
 * BootstrapPage - Platform bootstrap/initialization
 *
 * CLI equivalent:
 * - ai-aas-cli bootstrap
 */
export default function BootstrapPage() {
  const [formData, setFormData] = useState<BootstrapFormData>({
    admin_email: '',
    admin_password: '',
    org_name: '',
    hf_token: '',
    skip_model_pull: false,
  });

  // Bootstrap mutation
  const bootstrapMutation = useMutation({
    mutationFn: (data: BootstrapFormData) => platformApi.bootstrap(data),
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    bootstrapMutation.mutate(formData);
  };

  const handleInputChange = (field: keyof BootstrapFormData, value: string | boolean) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
  };

  return (
    <AdminLayout>
      <PageHeader
        title="Platform Bootstrap"
        description="Initialize the AI-AAS platform with initial configuration"
      />

      {bootstrapMutation.isSuccess && (
        <Card className="mb-6 border-green-200 dark:border-green-800">
          <CardContent>
            <div className="flex items-center gap-3">
              <StatusBadge status="success">Complete</StatusBadge>
              <span className="text-sm text-green-600 dark:text-green-400">
                Platform has been successfully bootstrapped!
              </span>
            </div>
            {bootstrapMutation.data && (
              <div className="mt-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-md">
                <p className="text-sm font-medium mb-2">Admin API Key:</p>
                <code className="text-xs break-all text-gray-600 dark:text-gray-400">
                  {bootstrapMutation.data.api_key}
                </code>
                <p className="text-xs text-amber-600 dark:text-amber-400 mt-2">
                  Save this key securely - it will not be shown again.
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {bootstrapMutation.isError && (
        <Card className="mb-6 border-red-200 dark:border-red-800">
          <CardContent>
            <div className="flex items-center gap-3">
              <StatusBadge status="error">Error</StatusBadge>
              <span className="text-sm text-red-600 dark:text-red-400">
                {bootstrapMutation.error instanceof Error
                  ? bootstrapMutation.error.message
                  : 'Bootstrap failed'}
              </span>
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader title="Bootstrap Configuration" />
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="space-y-4">
              <h3 className="text-sm font-medium text-gray-900 dark:text-white">
                Admin Account
              </h3>
              <Input
                label="Admin Email"
                type="email"
                value={formData.admin_email}
                onChange={(e) => handleInputChange('admin_email', e.target.value)}
                required
                hint="Email address for the initial admin user"
              />
              <Input
                label="Admin Password"
                type="password"
                value={formData.admin_password}
                onChange={(e) => handleInputChange('admin_password', e.target.value)}
                required
                hint="Password for the initial admin user (min 8 characters)"
              />
            </div>

            <div className="space-y-4">
              <h3 className="text-sm font-medium text-gray-900 dark:text-white">
                Organization
              </h3>
              <Input
                label="Organization Name"
                value={formData.org_name}
                onChange={(e) => handleInputChange('org_name', e.target.value)}
                required
                hint="Name for the initial organization"
              />
            </div>

            <div className="space-y-4">
              <h3 className="text-sm font-medium text-gray-900 dark:text-white">
                Model Configuration (Optional)
              </h3>
              <Input
                label="Hugging Face Token"
                type="password"
                value={formData.hf_token || ''}
                onChange={(e) => handleInputChange('hf_token', e.target.value)}
                hint="Required for pulling gated models from Hugging Face"
              />
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="skip_model_pull"
                  checked={formData.skip_model_pull || false}
                  onChange={(e) => handleInputChange('skip_model_pull', e.target.checked)}
                  className="h-4 w-4 text-primary rounded border-gray-300"
                />
                <label
                  htmlFor="skip_model_pull"
                  className="text-sm text-gray-700 dark:text-gray-300"
                >
                  Skip initial model pull
                </label>
              </div>
            </div>

            <div className="pt-4 border-t border-gray-200 dark:border-gray-700">
              <Button
                type="submit"
                loading={bootstrapMutation.isPending}
                disabled={!formData.admin_email || !formData.admin_password || !formData.org_name}
              >
                Bootstrap Platform
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># Bootstrap the platform</p>
            <p className="text-primary">ai-aas-cli bootstrap</p>
            <p className="text-gray-500 mt-3"># Bootstrap with Hugging Face token</p>
            <p className="text-primary">ai-aas-cli bootstrap --hf-token {'<token>'}</p>
            <p className="text-gray-500 mt-3"># Bootstrap without model pull</p>
            <p className="text-primary">ai-aas-cli bootstrap --skip-model-pull</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
