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
  CardFooter,
  Input,
  StatusBadge,
} from '@/components/ui';

/**
 * InferencePage - Test inference endpoints
 *
 * CLI equivalents:
 * - ai-aas-cli inference get-models
 * - ai-aas-cli inference send-request
 */
export default function InferencePage() {
  const [selectedModel, setSelectedModel] = useState('');
  const [prompt, setPrompt] = useState('');
  const [maxTokens, setMaxTokens] = useState(256);
  const [temperature, setTemperature] = useState(0.7);
  const [response, setResponse] = useState<{
    response: string;
    tokens_used: number;
    latency_ms: number;
  } | null>(null);

  // Fetch available models
  const { data: models = [], isLoading: isLoadingModels } = useQuery({
    queryKey: ['inference-models'],
    queryFn: () => modelsApi.inferenceGetModels(),
  });

  // Send request mutation
  const sendMutation = useMutation({
    mutationFn: () =>
      modelsApi.inferenceSendRequest(selectedModel, prompt, {
        max_tokens: maxTokens,
        temperature,
      }),
    onSuccess: (data) => {
      setResponse(data);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (selectedModel && prompt) {
      sendMutation.mutate();
    }
  };

  return (
    <AdminLayout>
      <PageHeader
        title="Inference Playground"
        description="Test inference endpoints with different models"
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Request Card */}
        <Card>
          <CardHeader title="Request" />
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <Select
                label="Model"
                value={selectedModel}
                onChange={(e) => setSelectedModel(e.target.value)}
                required
                disabled={isLoadingModels}
              >
                <option value="">Select a model</option>
                {models.map((model) => (
                  <option key={model.id} value={model.id} disabled={!model.available}>
                    {model.name} ({model.provider})
                    {!model.available && ' - Unavailable'}
                  </option>
                ))}
              </Select>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Prompt
                </label>
                <textarea
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                  rows={6}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
                  placeholder="Enter your prompt here..."
                  required
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <Input
                  label="Max Tokens"
                  type="number"
                  min={1}
                  max={4096}
                  value={maxTokens}
                  onChange={(e) => setMaxTokens(parseInt(e.target.value, 10))}
                />
                <Input
                  label="Temperature"
                  type="number"
                  min={0}
                  max={2}
                  step={0.1}
                  value={temperature}
                  onChange={(e) => setTemperature(parseFloat(e.target.value))}
                />
              </div>
            </form>
          </CardContent>
          <CardFooter>
            <Button
              onClick={() => sendMutation.mutate()}
              loading={sendMutation.isPending}
              disabled={!selectedModel || !prompt}
            >
              Send Request
            </Button>
          </CardFooter>
        </Card>

        {/* Response Card */}
        <Card>
          <CardHeader
            title="Response"
            actions={
              response && (
                <div className="flex items-center space-x-4 text-sm">
                  <span className="text-gray-500 dark:text-gray-400">
                    {response.tokens_used} tokens
                  </span>
                  <span className="text-gray-500 dark:text-gray-400">
                    {response.latency_ms}ms
                  </span>
                </div>
              )
            }
          />
          <CardContent>
            {sendMutation.isPending ? (
              <div className="flex items-center justify-center py-12">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
              </div>
            ) : sendMutation.isError ? (
              <div className="p-4 bg-red-50 dark:bg-red-900/30 rounded-md">
                <p className="text-sm text-red-600 dark:text-red-400">
                  Error: {sendMutation.error instanceof Error ? sendMutation.error.message : 'Unknown error'}
                </p>
              </div>
            ) : response ? (
              <div className="bg-gray-50 dark:bg-gray-800 rounded-md p-4">
                <pre className="text-sm whitespace-pre-wrap font-mono">
                  {response.response}
                </pre>
              </div>
            ) : (
              <p className="text-gray-500 dark:text-gray-400 text-center py-12">
                Send a request to see the response
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Available Models */}
      <Card className="mt-6">
        <CardHeader title="Available Models" />
        <CardContent>
          {isLoadingModels ? (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {models.map((model) => (
                <div
                  key={model.id}
                  className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700"
                >
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-medium">{model.name}</span>
                    <StatusBadge status={model.available ? 'success' : 'error'}>
                      {model.available ? 'Available' : 'Unavailable'}
                    </StatusBadge>
                  </div>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    Provider: {model.provider}
                  </p>
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-1 font-mono">
                    {model.id}
                  </p>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </AdminLayout>
  );
}
