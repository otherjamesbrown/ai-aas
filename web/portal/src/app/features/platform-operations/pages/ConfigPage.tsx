import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
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

interface ConfigItem {
  key: string;
  value: string;
  description?: string;
  editable: boolean;
  type: 'string' | 'number' | 'boolean' | 'json';
}

/**
 * ConfigPage - Platform configuration management
 *
 * CLI equivalents:
 * - ai-aas-cli config show
 * - ai-aas-cli config set <key> <value>
 * - ai-aas-cli config test
 */
export default function ConfigPage() {
  const queryClient = useQueryClient();
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');

  // Fetch config
  const { data: config = [], isLoading } = useQuery({
    queryKey: ['platform-config'],
    queryFn: () => platformApi.configShow(),
  });

  // Set config mutation
  const setConfigMutation = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) =>
      platformApi.configSet(key, value),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['platform-config'] });
      setEditingKey(null);
      setEditValue('');
    },
  });

  // Test config mutation
  const testConfigMutation = useMutation({
    mutationFn: () => platformApi.configTest(),
  });

  const handleEdit = (item: ConfigItem) => {
    setEditingKey(item.key);
    setEditValue(item.value);
  };

  const handleSave = () => {
    if (editingKey) {
      setConfigMutation.mutate({ key: editingKey, value: editValue });
    }
  };

  const handleCancel = () => {
    setEditingKey(null);
    setEditValue('');
  };

  // Group configs by category
  const groupedConfig = config.reduce<Record<string, ConfigItem[]>>((acc, item) => {
    const category = item.key.split('.')[0] || 'general';
    if (!acc[category]) {
      acc[category] = [];
    }
    acc[category].push(item);
    return acc;
  }, {});

  return (
    <AdminLayout>
      <PageHeader
        title="Platform Configuration"
        description="View and modify platform settings"
        actions={
          <Button
            variant="secondary"
            onClick={() => testConfigMutation.mutate()}
            loading={testConfigMutation.isPending}
          >
            Test Configuration
          </Button>
        }
      />

      {testConfigMutation.data && (
        <Card className="mb-6">
          <CardContent>
            <div className="flex items-center gap-3">
              <StatusBadge status={testConfigMutation.data.valid ? 'success' : 'error'}>
                {testConfigMutation.data.valid ? 'Valid' : 'Invalid'}
              </StatusBadge>
              <span className="text-sm text-gray-600 dark:text-gray-400">
                {testConfigMutation.data.message}
              </span>
            </div>
            {testConfigMutation.data.errors && testConfigMutation.data.errors.length > 0 && (
              <ul className="mt-3 space-y-1">
                {testConfigMutation.data.errors.map((error, i) => (
                  <li key={i} className="text-sm text-red-600 dark:text-red-400">
                    • {error}
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      )}

      {isLoading ? (
        <Card>
          <CardContent>
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
            </div>
          </CardContent>
        </Card>
      ) : (
        Object.entries(groupedConfig).map(([category, items]) => (
          <Card key={category} className="mb-6">
            <CardHeader
              title={category.charAt(0).toUpperCase() + category.slice(1)}
            />
            <CardContent>
              <div className="space-y-4">
                {items.map((item) => (
                  <div
                    key={item.key}
                    className="flex items-start justify-between py-3 border-b border-gray-100 dark:border-gray-700 last:border-0"
                  >
                    <div className="flex-1 min-w-0 mr-4">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-gray-900 dark:text-white">
                          {item.key}
                        </span>
                        {!item.editable && (
                          <span className="px-2 py-0.5 text-xs bg-gray-100 dark:bg-gray-700 rounded">
                            read-only
                          </span>
                        )}
                      </div>
                      {item.description && (
                        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                          {item.description}
                        </p>
                      )}
                      {editingKey === item.key ? (
                        <div className="mt-2 flex items-center gap-2">
                          <Input
                            value={editValue}
                            onChange={(e) => setEditValue(e.target.value)}
                            className="flex-1"
                          />
                          <Button
                            size="sm"
                            onClick={handleSave}
                            loading={setConfigMutation.isPending}
                          >
                            Save
                          </Button>
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={handleCancel}
                          >
                            Cancel
                          </Button>
                        </div>
                      ) : (
                        <div className="mt-1 font-mono text-sm bg-gray-50 dark:bg-gray-800 px-2 py-1 rounded">
                          {item.type === 'boolean' ? (
                            <StatusBadge status={item.value === 'true' ? 'success' : 'neutral'}>
                              {item.value}
                            </StatusBadge>
                          ) : item.value.length > 100 ? (
                            <span className="text-gray-600 dark:text-gray-400">
                              {item.value.substring(0, 100)}...
                            </span>
                          ) : (
                            item.value || <span className="text-gray-400">(empty)</span>
                          )}
                        </div>
                      )}
                    </div>
                    {item.editable && editingKey !== item.key && (
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleEdit(item)}
                      >
                        Edit
                      </Button>
                    )}
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        ))
      )}

      {/* CLI Reference */}
      <Card className="mt-6">
        <CardHeader title="CLI Commands" />
        <CardContent>
          <div className="bg-gray-100 dark:bg-gray-800 rounded-md p-4 font-mono text-sm space-y-2">
            <p className="text-gray-500"># Show all configuration</p>
            <p className="text-primary">ai-aas-cli config show</p>
            <p className="text-gray-500 mt-3"># Set a configuration value</p>
            <p className="text-primary">ai-aas-cli config set {'<key> <value>'}</p>
            <p className="text-gray-500 mt-3"># Test configuration validity</p>
            <p className="text-primary">ai-aas-cli config test</p>
          </div>
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
