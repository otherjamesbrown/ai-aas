import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { membersApi } from '../api/members';
import { ConfirmDestructiveModal } from '@/components/ConfirmDestructiveModal';
// InviteMemberModal is defined inline below
import type { MemberAccount, MemberRole } from '../types';

/**
 * Member management page with invite/resend/remove/role flows
 */
export default function MemberManagementPage() {
  const queryClient = useQueryClient();
  const [showInviteModal, setShowInviteModal] = useState(false);
  const [showRemoveModal, setShowRemoveModal] = useState(false);
  const [selectedMember, setSelectedMember] = useState<MemberAccount | null>(null);
  const [showRoleModal, setShowRoleModal] = useState(false);

  const { data: members, isLoading } = useQuery({
    queryKey: ['members'],
    queryFn: () => membersApi.list(),
  });

  const inviteMutation = useMutation({
    mutationFn: membersApi.invite,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['members'] });
      setShowInviteModal(false);
    },
  });

  const removeMutation = useMutation({
    mutationFn: (memberId: string) => membersApi.remove(memberId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['members'] });
      setShowRemoveModal(false);
      setSelectedMember(null);
    },
  });

  const resendMutation = useMutation({
    mutationFn: (memberId: string) => membersApi.resendInvite(memberId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['members'] });
    },
  });

  const updateRoleMutation = useMutation({
    mutationFn: ({ memberId, role, scopes }: { memberId: string; role: MemberRole; scopes?: string[] }) =>
      membersApi.updateRole(memberId, role, scopes),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['members'] });
      setShowRoleModal(false);
      setSelectedMember(null);
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (memberId: string) => membersApi.revokeInvite(memberId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['members'] });
    },
  });

  if (isLoading) {
    return <div className="flex items-center justify-center min-h-screen">Loading...</div>;
  }

  const handleRemove = (member: MemberAccount) => {
    setSelectedMember(member);
    setShowRemoveModal(true);
  };

  const handleUpdateRole = (member: MemberAccount) => {
    setSelectedMember(member);
    setShowRoleModal(true);
  };

  return (
    <div className="max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-3xl font-bold text-gray-900">Members</h1>
        <button
          onClick={() => setShowInviteModal(true)}
          className="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
        >
          Invite Member
        </button>
      </div>

      <div className="bg-white shadow rounded-lg overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Email
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Role
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Last Active
              </th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {members?.map((member) => (
              <tr key={member.member_id}>
                <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                  {member.email}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  <span className="px-2 py-1 text-xs font-semibold rounded-full bg-blue-100 text-blue-800">
                    {member.role}
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  <span
                    className={`px-2 py-1 text-xs font-semibold rounded-full ${
                      member.invite_status === 'accepted'
                        ? 'bg-green-100 text-green-800'
                        : member.invite_status === 'pending'
                        ? 'bg-yellow-100 text-yellow-800'
                        : 'bg-red-100 text-red-800'
                    }`}
                  >
                    {member.invite_status}
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {member.last_active_at
                    ? new Date(member.last_active_at).toLocaleDateString()
                    : 'Never'}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                  <div className="flex justify-end space-x-2">
                    {member.invite_status === 'pending' && (
                      <>
                        <button
                          onClick={() => resendMutation.mutate(member.member_id)}
                          disabled={resendMutation.isPending}
                          className="text-primary hover:text-primary-dark disabled:opacity-50"
                        >
                          Resend
                        </button>
                        <button
                          onClick={() => revokeMutation.mutate(member.member_id)}
                          disabled={revokeMutation.isPending}
                          className="text-yellow-600 hover:text-yellow-900 disabled:opacity-50"
                        >
                          Revoke
                        </button>
                      </>
                    )}
                    <button
                      onClick={() => handleUpdateRole(member)}
                      className="text-primary hover:text-primary-dark"
                    >
                      Change Role
                    </button>
                    {member.role !== 'owner' && (
                      <button
                        onClick={() => handleRemove(member)}
                        className="text-red-600 hover:text-red-900"
                      >
                        Remove
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <InviteMemberModal
        isOpen={showInviteModal}
        onClose={() => setShowInviteModal(false)}
        onInvite={(email, role, scopes) =>
          inviteMutation.mutate({ email, role, scopes })
        }
        isLoading={inviteMutation.isPending}
      />

      {selectedMember && (
        <>
          <ConfirmDestructiveModal
            isOpen={showRemoveModal}
            onClose={() => {
              setShowRemoveModal(false);
              setSelectedMember(null);
            }}
            onConfirm={() => selectedMember && removeMutation.mutate(selectedMember.member_id)}
            title="Remove Member"
            message={`Are you sure you want to remove ${selectedMember.email} from the organization? This action cannot be undone.`}
            confirmText="Remove Member"
            confirmationText="REMOVE"
            isLoading={removeMutation.isPending}
          />

          <UpdateRoleModal
            isOpen={showRoleModal}
            onClose={() => {
              setShowRoleModal(false);
              setSelectedMember(null);
            }}
            member={selectedMember}
            onUpdate={(role, scopes) =>
              selectedMember &&
              updateRoleMutation.mutate({
                memberId: selectedMember.member_id,
                role,
                scopes,
              })
            }
            isLoading={updateRoleMutation.isPending}
          />
        </>
      )}
    </div>
  );
}

// Invite Member Modal Component
function InviteMemberModal({
  isOpen,
  onClose,
  onInvite,
  isLoading,
}: {
  isOpen: boolean;
  onClose: () => void;
  onInvite: (email: string, role: MemberRole, scopes?: string[]) => void;
  isLoading: boolean;
}) {
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<MemberRole>('manager');
  const [scopes, setScopes] = useState<string[]>([]);

  if (!isOpen) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onInvite(email, role, scopes.length > 0 ? scopes : undefined);
    setEmail('');
    setRole('manager');
    setScopes([]);
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto" aria-modal="true">
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="fixed inset-0 bg-gray-500 bg-opacity-75" onClick={onClose} />
        <div className="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
          <h2 className="text-xl font-bold mb-4">Invite Member</h2>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="invite-email" className="block text-sm font-medium text-gray-700">
                Email
              </label>
              <input
                type="email"
                id="invite-email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
            </div>
            <div>
              <label htmlFor="invite-role" className="block text-sm font-medium text-gray-700">
                Role
              </label>
              <select
                id="invite-role"
                value={role}
                onChange={(e) => setRole(e.target.value as MemberRole)}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              >
                <option value="owner">Owner</option>
                <option value="admin">Admin</option>
                <option value="manager">Manager</option>
                <option value="analyst">Analyst</option>
              </select>
            </div>
            <div className="flex justify-end space-x-3">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isLoading}
                className="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark disabled:opacity-50"
              >
                {isLoading ? 'Sending...' : 'Send Invite'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

// Update Role Modal Component
function UpdateRoleModal({
  isOpen,
  onClose,
  member,
  onUpdate,
  isLoading,
}: {
  isOpen: boolean;
  onClose: () => void;
  member: MemberAccount;
  onUpdate: (role: MemberRole, scopes?: string[]) => void;
  isLoading: boolean;
}) {
  const [role, setRole] = useState<MemberRole>(member.role);
  const [scopes] = useState<string[]>(member.scopes);

  if (!isOpen) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onUpdate(role, scopes.length > 0 ? scopes : undefined);
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto" aria-modal="true">
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="fixed inset-0 bg-gray-500 bg-opacity-75" onClick={onClose} />
        <div className="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
          <h2 className="text-xl font-bold mb-4">Update Role for {member.email}</h2>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="update-role" className="block text-sm font-medium text-gray-700">
                Role
              </label>
              <select
                id="update-role"
                value={role}
                onChange={(e) => setRole(e.target.value as MemberRole)}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              >
                <option value="owner">Owner</option>
                <option value="admin">Admin</option>
                <option value="manager">Manager</option>
                <option value="analyst">Analyst</option>
              </select>
            </div>
            <div className="flex justify-end space-x-3">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isLoading}
                className="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark disabled:opacity-50"
              >
                {isLoading ? 'Updating...' : 'Update Role'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

