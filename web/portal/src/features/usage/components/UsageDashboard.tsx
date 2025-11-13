import { useState } from 'react';
import { useUsageReport, useAvailableModels } from '../hooks/useUsageReport';
import { UsageEmptyState, UsageDegradedState } from './UsageEmptyState';
import { StatusBadge } from './StatusBadge';
import { exportUsageCsv } from '../workers/exportUsageCsv';
import type { TimeRange, OperationType } from '../types';

/**
 * Usage dashboard UI with charts and KPIs
 */
export default function UsageDashboard() {
  const [timeRange, setTimeRange] = useState<TimeRange>('last_7d');
  const [selectedModel, setSelectedModel] = useState<string>('');
  const [selectedOperation, setSelectedOperation] = useState<OperationType | ''>('');
  const [isExporting, setIsExporting] = useState(false);

  const filters = {
    time_range: timeRange,
    model: selectedModel || undefined,
    operation: selectedOperation || undefined,
  };

  const { data, isLoading, error, refetch } = useUsageReport(filters);
  const { data: availableModels } = useAvailableModels();

  const handleExport = async () => {
    setIsExporting(true);
    try {
      await exportUsageCsv(filters);
    } catch (err) {
      console.error('Export failed:', err);
      alert('Failed to export usage data. Please try again.');
    } finally {
      setIsExporting(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto mb-4" />
          <p className="text-gray-600">Loading usage data...</p>
        </div>
      </div>
    );
  }

  if (error && !data) {
    return (
      <div className="max-w-4xl mx-auto">
        <UsageDegradedState onRetry={() => refetch()} isRetrying={isLoading} />
      </div>
    );
  }

  const report = data?.report;
  const isEmpty = !report || report.breakdowns.length === 0;
  const isDegraded = report?.source === 'degraded';

  return (
    <div className="max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-3xl font-bold text-gray-900">Usage Insights</h1>
        <StatusBadge />
      </div>

      {isDegraded && (
        <UsageDegradedState onRetry={() => refetch()} isRetrying={isLoading} />
      )}

      {/* Filters */}
      <div className="bg-white shadow rounded-lg p-6 mb-6">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <label htmlFor="time-range" className="block text-sm font-medium text-gray-700 mb-2">
              Time Range
            </label>
            <select
              id="time-range"
              value={timeRange}
              onChange={(e) => setTimeRange(e.target.value as TimeRange)}
              className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
            >
              <option value="last_24h">Last 24 Hours</option>
              <option value="last_7d">Last 7 Days</option>
              <option value="last_30d">Last 30 Days</option>
            </select>
          </div>

          <div>
            <label htmlFor="model" className="block text-sm font-medium text-gray-700 mb-2">
              Model
            </label>
            <select
              id="model"
              value={selectedModel}
              onChange={(e) => setSelectedModel(e.target.value)}
              className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
            >
              <option value="">All Models</option>
              {availableModels?.map((model) => (
                <option key={model} value={model}>
                  {model}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="operation" className="block text-sm font-medium text-gray-700 mb-2">
              Operation
            </label>
            <select
              id="operation"
              value={selectedOperation}
              onChange={(e) => setSelectedOperation(e.target.value as OperationType | '')}
              className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
            >
              <option value="">All Operations</option>
              <option value="chat">Chat</option>
              <option value="embeddings">Embeddings</option>
              <option value="fine-tune">Fine-tune</option>
            </select>
          </div>
        </div>

        <div className="mt-4 flex justify-end">
          <button
            onClick={handleExport}
            disabled={isExporting || isEmpty}
            className="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isExporting ? 'Exporting...' : 'Export CSV'}
          </button>
        </div>
      </div>

      {isEmpty ? (
        <UsageEmptyState />
      ) : (
        <>
          {/* KPIs */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
            <div className="bg-white shadow rounded-lg p-6">
              <div className="text-sm font-medium text-gray-500 mb-1">Total Requests</div>
              <div className="text-3xl font-bold text-gray-900">
                {report.totals.requests.toLocaleString()}
              </div>
            </div>
            <div className="bg-white shadow rounded-lg p-6">
              <div className="text-sm font-medium text-gray-500 mb-1">Total Tokens</div>
              <div className="text-3xl font-bold text-gray-900">
                {report.totals.tokens.toLocaleString()}
              </div>
            </div>
            <div className="bg-white shadow rounded-lg p-6">
              <div className="text-sm font-medium text-gray-500 mb-1">Total Cost</div>
              <div className="text-3xl font-bold text-gray-900">
                ${(report.totals.cost_cents / 100).toFixed(2)}
              </div>
            </div>
          </div>

          {/* Usage Table */}
          <div className="bg-white shadow rounded-lg overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200">
              <h2 className="text-lg font-medium text-gray-900">Usage Breakdown</h2>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Time Window
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Model
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Operation
                    </th>
                    <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Requests
                    </th>
                    <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Tokens
                    </th>
                    <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Cost
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {report.breakdowns.map((snapshot, index) => (
                    <tr key={index}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {new Date(snapshot.window_start).toLocaleDateString()}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {snapshot.model}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {snapshot.operation}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 text-right">
                        {snapshot.requests.toLocaleString()}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 text-right">
                        {snapshot.tokens.toLocaleString()}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 text-right">
                        ${(snapshot.cost_cents / 100).toFixed(2)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

