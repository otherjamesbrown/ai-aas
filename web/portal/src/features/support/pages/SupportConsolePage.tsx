import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { impersonationApi, type ImpersonationRequest } from '../api/impersonation';
import { ImpersonationConsentModal } from '../components/ImpersonationConsentModal';
import { ImpersonationBanner } from '../components/ImpersonationBanner';
import { useImpersonationGuard } from '../hooks/useImpersonationGuard';

/**
 * Support console page for managing impersonation sessions
 */
export default function SupportConsolePage() {
  const queryClient = useQueryClient();
  const [showConsentModal, setShowConsentModal] = useState(false);
  const [selectedOrgId, setSelectedOrgId] = useState<string>('');
  const [selectedOrgName, setSelectedOrgName] = useState<string>('');

  const { session, isImpersonating } = useImpersonationGuard();

  const { data: sessions, isLoading } = useQuery({
    queryKey: ['impersonation', 'sessions'],
    queryFn: () => impersonationApi.listSessions(),
    enabled: !isImpersonating, // Only fetch when not already impersonating
  });

  const createMutation = useMutation({
    mutationFn: (request: ImpersonationRequest) => impersonationApi.createSession(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['impersonation'] });
      setShowConsentModal(false);
      // Refresh current session
      queryClient.invalidateQueries({ queryKey: ['impersonation', 'current'] });
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (sessionId: string) => impersonationApi.revokeSession(sessionId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['impersonation'] });
    },
  });

  const handleRequestAccess = (orgId: string, orgName: string) => {
    setSelectedOrgId(orgId);
    setSelectedOrgName(orgName);
    setShowConsentModal(true);
  };

  const handleSubmitConsent = async (request: ImpersonationRequest) => {
    await createMutation.mutateAsync(request);
  };

  // If impersonating, show banner and redirect to org view
  if (isImpersonating && session) {
    return (
      <div>
        <ImpersonationBanner
          session={session}
          onRevoke={() => {
            queryClient.invalidateQueries({ queryKey: ['impersonation'] });
          }}
        />
        <div className="max-w-7xl mx-auto p-6">
          <div className="bg-white shadow rounded-lg p-6">
            <h2 className="text-2xl font-bold text-gray-900 mb-4">
              Viewing Organization: {session.organization_id}
            </h2>
            <p className="text-gray-600 mb-4">
              You are in read-only mode. You can view organization data but cannot make changes.
            </p>
            <div className="space-y-2">
              <p className="text-sm text-gray-500">
                <strong>Session ID:</strong> {session.session_id}
              </p>
              <p className="text-sm text-gray-500">
                <strong>Expires:</strong> {new Date(session.expires_at).toLocaleString()}
              </p>
              <p className="text-sm text-gray-500">
                <strong>Scope:</strong> {session.scope}
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-3xl font-bold text-gray-900">Support Console</h1>
      </div>

      <div className="bg-white shadow rounded-lg p-6 mb-6">
        <h2 className="text-lg font-medium text-gray-900 mb-4">Request Impersonation Access</h2>
        <p className="text-gray-600 mb-4">
          Enter an organization ID to request read-only access for support purposes.
        </p>
        <div className="flex space-x-2">
          <input
            type="text"
            placeholder="Organization ID or Name"
            className="flex-1 rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                const value = (e.target as HTMLInputElement).value;
                if (value) {
                  handleRequestAccess(value, value); // In real app, would look up org name
                }
              }
            }}
          />
          <button
            onClick={() => {
              const input = document.querySelector('input[placeholder="Organization ID or Name"]') as HTMLInputElement;
              if (input?.value) {
                handleRequestAccess(input.value, input.value);
              }
            }}
            className="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark"
          >
            Request Access
          </button>
        </div>
      </div>

      <div className="bg-white shadow rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">Active Sessions</h2>
        </div>
        {isLoading ? (
          <div className="p-6 text-center text-gray-500">Loading sessions...</div>
        ) : sessions && sessions.length > 0 ? (
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Organization ID
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Requested
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Expires
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {sessions.map((session) => (
                <tr key={session.session_id}>
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                    {session.organization_id}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {new Date(session.requested_at).toLocaleString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {new Date(session.expires_at).toLocaleString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <button
                      onClick={() => revokeMutation.mutate(session.session_id)}
                      disabled={revokeMutation.isPending}
                      className="text-red-600 hover:text-red-900 disabled:opacity-50"
                    >
                      Revoke
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div className="p-6 text-center text-gray-500">No active sessions</div>
        )}
      </div>

      <ImpersonationConsentModal
        isOpen={showConsentModal}
        onClose={() => {
          setShowConsentModal(false);
          setSelectedOrgId('');
          setSelectedOrgName('');
        }}
        onSubmit={handleSubmitConsent}
        organizationId={selectedOrgId}
        organizationName={selectedOrgName}
        isLoading={createMutation.isPending}
      />
    </div>
  );
}

