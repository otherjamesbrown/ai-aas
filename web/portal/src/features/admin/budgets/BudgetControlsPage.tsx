import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { budgetsApi } from '../api/budgets';
import { ConfirmDestructiveModal } from '@/components/ConfirmDestructiveModal';
import { StaleDataBanner } from '../components/StaleDataBanner';
import type { UpdateBudgetRequest, EnforcementMode } from '../types';

/**
 * Budget controls UI with confirmations
 */
export default function BudgetControlsPage() {
  const queryClient = useQueryClient();
  const [showConfirm, setShowConfirm] = useState(false);
  const [pendingUpdates, setPendingUpdates] = useState<UpdateBudgetRequest>({});

  const { data: policy, isLoading, dataUpdatedAt } = useQuery({
    queryKey: ['budget', 'policy'],
    queryFn: () => budgetsApi.getPolicy(),
  });

  const updateMutation = useMutation({
    mutationFn: (updates: UpdateBudgetRequest) => budgetsApi.updatePolicy(updates),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['budget'] });
      setShowConfirm(false);
      setPendingUpdates({});
    },
  });

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formData = new FormData(e.currentTarget);
    const updates: UpdateBudgetRequest = {
      monthly_limit_cents: Math.round(parseFloat(formData.get('monthly_limit') as string) * 100),
      alert_thresholds: (formData.get('alert_thresholds') as string)
        .split(',')
        .map((t) => parseFloat(t.trim()))
        .filter((t) => !isNaN(t)),
      currency: formData.get('currency') as string,
      enforcement_mode: formData.get('enforcement_mode') as EnforcementMode,
      alert_recipients: (formData.get('alert_recipients') as string)
        .split(',')
        .map((e) => e.trim())
        .filter((e) => e.length > 0),
    };

    setPendingUpdates(updates);
    setShowConfirm(true);
  };

  const handleConfirm = async () => {
    await updateMutation.mutateAsync(pendingUpdates);
  };

  if (isLoading) {
    return <div className="flex items-center justify-center min-h-screen">Loading...</div>;
  }

  if (!policy) {
    return <div>Budget policy not found</div>;
  }

  const monthlyLimitDollars = policy.monthly_limit_cents / 100;

  return (
    <div className="max-w-4xl mx-auto">
      <h1 className="text-3xl font-bold text-gray-900 mb-6">Budget Controls</h1>

      {dataUpdatedAt && (
        <StaleDataBanner
          lastUpdated={new Date(dataUpdatedAt).toISOString()}
          onRefresh={() => queryClient.invalidateQueries({ queryKey: ['budget', 'policy'] })}
          isRefreshing={updateMutation.isPending}
        />
      )}

      <form onSubmit={handleSubmit} className="bg-white shadow rounded-lg p-6 space-y-6">
        <div>
          <label htmlFor="monthly_limit" className="block text-sm font-medium text-gray-700">
            Monthly Limit ({policy.currency})
          </label>
          <input
            type="number"
            id="monthly_limit"
            name="monthly_limit"
            defaultValue={monthlyLimitDollars}
            min="0"
            step="0.01"
            required
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
          />
          <p className="mt-1 text-sm text-gray-500">
            Set the maximum monthly spending limit. Changes require confirmation.
          </p>
        </div>

        <div>
          <label htmlFor="alert_thresholds" className="block text-sm font-medium text-gray-700">
            Alert Thresholds (%)
          </label>
          <input
            type="text"
            id="alert_thresholds"
            name="alert_thresholds"
            defaultValue={policy.alert_thresholds.join(', ')}
            placeholder="50, 75, 90"
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
          />
          <p className="mt-1 text-sm text-gray-500">
            Comma-separated percentages at which to send alerts (e.g., 50, 75, 90)
          </p>
        </div>

        <div>
          <label htmlFor="enforcement_mode" className="block text-sm font-medium text-gray-700">
            Enforcement Mode
          </label>
          <select
            id="enforcement_mode"
            name="enforcement_mode"
            defaultValue={policy.enforcement_mode}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
          >
            <option value="monitor">Monitor Only</option>
            <option value="warn">Warn on Exceed</option>
            <option value="block">Block on Exceed</option>
          </select>
          <p className="mt-1 text-sm text-gray-500">
            How to handle spending that exceeds the limit
          </p>
        </div>

        <div>
          <label htmlFor="alert_recipients" className="block text-sm font-medium text-gray-700">
            Alert Recipients
          </label>
          <textarea
            id="alert_recipients"
            name="alert_recipients"
            rows={3}
            defaultValue={policy.alert_recipients.join(', ')}
            placeholder="admin@example.com, finance@example.com"
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
          />
          <p className="mt-1 text-sm text-gray-500">
            Comma-separated email addresses to receive budget alerts
          </p>
        </div>

        <div className="flex justify-end space-x-3">
          <button
            type="button"
            onClick={() => window.history.back()}
            className="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={updateMutation.isPending}
            className="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-primary hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50"
          >
            {updateMutation.isPending ? 'Saving...' : 'Update Budget'}
          </button>
        </div>
      </form>

      <ConfirmDestructiveModal
        isOpen={showConfirm}
        onClose={() => {
          setShowConfirm(false);
          setPendingUpdates({});
        }}
        onConfirm={handleConfirm}
        title="Confirm Budget Policy Update"
        message={`Are you sure you want to update the budget policy? This will change the monthly limit to ${pendingUpdates.monthly_limit_cents ? `$${(pendingUpdates.monthly_limit_cents / 100).toFixed(2)}` : 'the new amount'} and enforcement mode to ${pendingUpdates.enforcement_mode || policy.enforcement_mode}.`}
        confirmText="Update Budget Policy"
        confirmationText="UPDATE"
        isLoading={updateMutation.isPending}
      />
    </div>
  );
}

