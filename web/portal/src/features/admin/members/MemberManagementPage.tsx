import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { membersApi } from '../api/members';
import { ConfirmDestructiveModal } from '@/components/ConfirmDestructiveModal';
import { Button, DataTable, StatusBadge, IconButton, Modal, Input, Select } from '@/components/ui';
import type { Column } from '@/components/ui';
import type { MemberAccount, MemberRole } from '../types';

const ROLE_OPTIONS = [
  { value: 'owner', label: 'Owner' },
  { value: 'admin', label: 'Admin' },
  { value: 'manager', label: 'Manager' },
  { value: 'analyst', label: 'Analyst' },
];

/**
 * Member management page - rebuilt with unified components
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

  const columns: Column<MemberAccount>[] = [
    {
      key: 'email',
      header: 'Email',
      sortable: true,
      render: (item) => (
        <span className="font-medium text-gray-900 dark:text-white">{item.email}</span>
      ),
    },
    {
      key: 'role',
      header: 'Role',
      render: (item) => (
        <span className="px-2 py-1 text-xs font-semibold rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300">
          {item.role}
        </span>
      ),
    },
    {
      key: 'invite_status',
      header: 'Status',
      render: (item) => (
        <StatusBadge
          status={
            item.invite_status === 'accepted' ? 'success' :
            item.invite_status === 'pending' ? 'warning' : 'error'
          }
        >
          {item.invite_status}
        </StatusBadge>
      ),
    },
    {
      key: 'last_active_at',
      header: 'Last Active',
      sortable: true,
      render: (item) => (
        <span className="text-gray-500 dark:text-gray-400">
          {item.last_active_at ? new Date(item.last_active_at).toLocaleDateString() : 'Never'}
        </span>
      ),
    },
  ];

  const renderActions = (member: MemberAccount) => (
    <div className="flex items-center space-x-1">
      {member.invite_status === 'pending' && (
        <>
          <IconButton
            icon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
            }
            label="Resend invite"
            variant="ghost"
            size="sm"
            onClick={() => resendMutation.mutate(member.member_id)}
            disabled={resendMutation.isPending}
          />
          <IconButton
            icon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
              </svg>
            }
            label="Revoke invite"
            variant="ghost"
            size="sm"
            className="text-yellow-600 dark:text-yellow-400"
            onClick={() => revokeMutation.mutate(member.member_id)}
            disabled={revokeMutation.isPending}
          />
        </>
      )}
      <IconButton
        icon={
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
          </svg>
        }
        label="Change role"
        variant="ghost"
        size="sm"
        onClick={() => {
          setSelectedMember(member);
          setShowRoleModal(true);
        }}
      />
      {member.role !== 'owner' && (
        <IconButton
          icon={
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          }
          label="Remove member"
          variant="ghost"
          size="sm"
          className="text-red-600 hover:text-red-700 dark:text-red-400"
          onClick={() => {
            setSelectedMember(member);
            setShowRemoveModal(true);
          }}
        />
      )}
    </div>
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Members</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Manage team members and their roles
          </p>
        </div>
        <Button
          onClick={() => setShowInviteModal(true)}
          leftIcon={
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z" />
            </svg>
          }
        >
          Invite Member
        </Button>
      </div>

      {/* Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
        <DataTable
          columns={columns}
          data={members || []}
          keyExtractor={(item) => item.member_id}
          loading={isLoading}
          emptyMessage="No members found. Invite your first team member."
          actions={renderActions}
        />
      </div>

      {/* Invite Modal */}
      <InviteMemberModal
        isOpen={showInviteModal}
        onClose={() => setShowInviteModal(false)}
        onInvite={(email, role, scopes) => inviteMutation.mutate({ email, role, scopes })}
        isLoading={inviteMutation.isPending}
      />

      {/* Role & Remove Modals */}
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

// Invite Member Modal
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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onInvite(email, role, undefined);
    setEmail('');
    setRole('manager');
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Invite Member"
      description="Send an invitation to join your organization"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isLoading}>
            Cancel
          </Button>
          <Button type="submit" form="invite-form" loading={isLoading}>
            Send Invite
          </Button>
        </>
      }
    >
      <form id="invite-form" onSubmit={handleSubmit} className="space-y-4">
        <Input
          label="Email"
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="colleague@example.com"
        />
        <Select
          label="Role"
          value={role}
          onChange={(e) => setRole(e.target.value as MemberRole)}
          options={ROLE_OPTIONS}
        />
      </form>
    </Modal>
  );
}

// Update Role Modal
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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onUpdate(role, undefined);
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Update Role"
      description={`Change role for ${member.email}`}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isLoading}>
            Cancel
          </Button>
          <Button type="submit" form="role-form" loading={isLoading}>
            Update Role
          </Button>
        </>
      }
    >
      <form id="role-form" onSubmit={handleSubmit} className="space-y-4">
        <Select
          label="Role"
          value={role}
          onChange={(e) => setRole(e.target.value as MemberRole)}
          options={ROLE_OPTIONS}
        />
      </form>
    </Modal>
  );
}
