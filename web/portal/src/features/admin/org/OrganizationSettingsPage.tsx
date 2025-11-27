import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { organizationApi } from '../api/organization';
import { StaleDataBanner } from '../components/StaleDataBanner';
import { ConfirmDestructiveModal } from '@/components/ConfirmDestructiveModal';
import { Button, Input, LoadingSpinner } from '@/components/ui';
import type { UpdateOrganizationRequest } from '../types';

/**
 * Organization settings page - rebuilt with unified components
 */
export default function OrganizationSettingsPage() {
  const queryClient = useQueryClient();
  const [showConfirm, setShowConfirm] = useState(false);
  const [pendingUpdates, setPendingUpdates] = useState<UpdateOrganizationRequest>({});

  const { data: profile, isLoading, dataUpdatedAt } = useQuery({
    queryKey: ['organization', 'profile'],
    queryFn: () => organizationApi.getProfile(),
  });

  const updateMutation = useMutation({
    mutationFn: (updates: UpdateOrganizationRequest) =>
      organizationApi.updateProfile(updates),
    onMutate: async (updates) => {
      await queryClient.cancelQueries({ queryKey: ['organization', 'profile'] });
      const previousProfile = queryClient.getQueryData(['organization', 'profile']);
      queryClient.setQueryData(['organization', 'profile'], (old: any) => ({
        ...old,
        ...updates,
      }));
      return { previousProfile };
    },
    onError: (_err, _updates, context) => {
      if (context?.previousProfile) {
        queryClient.setQueryData(['organization', 'profile'], context.previousProfile);
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['organization', 'profile'] });
    },
  });

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formData = new FormData(e.currentTarget);
    const updates: UpdateOrganizationRequest = {
      name: formData.get('name') as string,
      billing_contact: {
        name: formData.get('billing_name') as string,
        email: formData.get('billing_email') as string,
        phone: formData.get('billing_phone') as string || undefined,
      },
      address: {
        country: formData.get('country') as string,
        region: formData.get('region') as string || undefined,
        postal_code: formData.get('postal_code') as string,
        street_lines: [(formData.get('street') as string) || ''],
      },
    };

    setPendingUpdates(updates);
    setShowConfirm(true);
  };

  const handleConfirm = async () => {
    await updateMutation.mutateAsync(pendingUpdates);
    setShowConfirm(false);
    setPendingUpdates({});
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (!profile) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500 dark:text-gray-400">Organization profile not found</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Organization Settings</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Manage your organization profile and billing information
        </p>
      </div>

      {/* Stale Data Banner */}
      {dataUpdatedAt && (
        <StaleDataBanner
          lastUpdated={new Date(dataUpdatedAt).toISOString()}
          onRefresh={() => queryClient.invalidateQueries({ queryKey: ['organization', 'profile'] })}
          isRefreshing={updateMutation.isPending}
        />
      )}

      {/* Form */}
      <form onSubmit={handleSubmit} className="space-y-8">
        {/* Organization Name */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
          <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Organization Details
          </h2>
          <Input
            label="Organization Name"
            name="name"
            defaultValue={profile.name}
            required
            maxLength={120}
          />
        </div>

        {/* Billing Contact */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
          <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Billing Contact
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Input
              label="Name"
              name="billing_name"
              defaultValue={profile.billing_contact.name}
              required
            />
            <Input
              label="Email"
              type="email"
              name="billing_email"
              defaultValue={profile.billing_contact.email}
              required
            />
            <Input
              label="Phone"
              type="tel"
              name="billing_phone"
              defaultValue={profile.billing_contact.phone}
            />
          </div>
        </div>

        {/* Address */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
          <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Address
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Input
              label="Country"
              name="country"
              defaultValue={profile.address.country}
              required
            />
            <Input
              label="Region/State"
              name="region"
              defaultValue={profile.address.region}
            />
            <Input
              label="Postal Code"
              name="postal_code"
              defaultValue={profile.address.postal_code}
              required
            />
            <Input
              label="Street Address"
              name="street"
              defaultValue={profile.address.street_lines[0]}
              required
            />
          </div>
        </div>

        {/* Actions */}
        <div className="flex justify-end space-x-3">
          <Button
            type="button"
            variant="secondary"
            onClick={() => window.history.back()}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            loading={updateMutation.isPending}
          >
            Save Changes
          </Button>
        </div>
      </form>

      {/* Confirmation Modal */}
      <ConfirmDestructiveModal
        isOpen={showConfirm}
        onClose={() => {
          setShowConfirm(false);
          setPendingUpdates({});
        }}
        onConfirm={handleConfirm}
        title="Confirm Organization Update"
        message="Are you sure you want to update the organization profile? Changes can be rolled back within 24 hours by contacting support."
        confirmText="Update Organization"
        confirmationText="UPDATE"
        isLoading={updateMutation.isPending}
      />
    </div>
  );
}
