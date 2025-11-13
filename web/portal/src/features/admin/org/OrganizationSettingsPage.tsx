import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { organizationApi } from '../api/organization';
import { StaleDataBanner } from '../components/StaleDataBanner';
import { ConfirmDestructiveModal } from '@/components/ConfirmDestructiveModal';
import type { UpdateOrganizationRequest } from '../types';

/**
 * Organization settings page with optimistic updates
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
      // Cancel outgoing refetches
      await queryClient.cancelQueries({ queryKey: ['organization', 'profile'] });

      // Snapshot previous value
      const previousProfile = queryClient.getQueryData(['organization', 'profile']);

      // Optimistically update
      queryClient.setQueryData(['organization', 'profile'], (old: any) => ({
        ...old,
        ...updates,
      }));

      return { previousProfile };
    },
    onError: (_err, _updates, context) => {
      // Rollback on error
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
    return <div className="flex items-center justify-center min-h-screen">Loading...</div>;
  }

  if (!profile) {
    return <div>Organization profile not found</div>;
  }

  return (
    <div className="max-w-4xl mx-auto">
      <h1 className="text-3xl font-bold text-gray-900 mb-6">Organization Settings</h1>

      {dataUpdatedAt && (
        <StaleDataBanner
          lastUpdated={new Date(dataUpdatedAt).toISOString()}
          onRefresh={() => queryClient.invalidateQueries({ queryKey: ['organization', 'profile'] })}
          isRefreshing={updateMutation.isPending}
        />
      )}

      <form onSubmit={handleSubmit} className="bg-white shadow rounded-lg p-6 space-y-6">
        <div>
          <label htmlFor="name" className="block text-sm font-medium text-gray-700">
            Organization Name
          </label>
          <input
            type="text"
            id="name"
            name="name"
            defaultValue={profile.name}
            required
            maxLength={120}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
          />
        </div>

        <div>
          <h2 className="text-lg font-medium text-gray-900 mb-4">Billing Contact</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label htmlFor="billing_name" className="block text-sm font-medium text-gray-700">
                Name
              </label>
              <input
                type="text"
                id="billing_name"
                name="billing_name"
                defaultValue={profile.billing_contact.name}
                required
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
            </div>
            <div>
              <label htmlFor="billing_email" className="block text-sm font-medium text-gray-700">
                Email
              </label>
              <input
                type="email"
                id="billing_email"
                name="billing_email"
                defaultValue={profile.billing_contact.email}
                required
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
            </div>
            <div>
              <label htmlFor="billing_phone" className="block text-sm font-medium text-gray-700">
                Phone
              </label>
              <input
                type="tel"
                id="billing_phone"
                name="billing_phone"
                defaultValue={profile.billing_contact.phone}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
            </div>
          </div>
        </div>

        <div>
          <h2 className="text-lg font-medium text-gray-900 mb-4">Address</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label htmlFor="country" className="block text-sm font-medium text-gray-700">
                Country
              </label>
              <input
                type="text"
                id="country"
                name="country"
                defaultValue={profile.address.country}
                required
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
            </div>
            <div>
              <label htmlFor="region" className="block text-sm font-medium text-gray-700">
                Region/State
              </label>
              <input
                type="text"
                id="region"
                name="region"
                defaultValue={profile.address.region}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
            </div>
            <div>
              <label htmlFor="postal_code" className="block text-sm font-medium text-gray-700">
                Postal Code
              </label>
              <input
                type="text"
                id="postal_code"
                name="postal_code"
                defaultValue={profile.address.postal_code}
                required
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
            </div>
            <div>
              <label htmlFor="street" className="block text-sm font-medium text-gray-700">
                Street Address
              </label>
              <input
                type="text"
                id="street"
                name="street"
                defaultValue={profile.address.street_lines[0]}
                required
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
            </div>
          </div>
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
            {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
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
        title="Confirm Organization Update"
        message="Are you sure you want to update the organization profile? Changes can be rolled back within 24 hours by contacting support."
        confirmText="Update Organization"
        confirmationText="UPDATE"
        isLoading={updateMutation.isPending}
      />
    </div>
  );
}

