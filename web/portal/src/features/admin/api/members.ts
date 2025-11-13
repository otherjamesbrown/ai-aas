import { httpClient } from '@/lib/http/client';
import type {
  MemberAccount,
  InviteMemberRequest,
  MemberRole,
} from '../types';

/**
 * Member management API client
 */
export const membersApi = {
  /**
   * List all members in the organization
   */
  async list(): Promise<MemberAccount[]> {
    const response = await httpClient.get<MemberAccount[]>('/organizations/me/members');
    return response.data;
  },

  /**
   * Get member by ID
   */
  async getById(memberId: string): Promise<MemberAccount> {
    const response = await httpClient.get<MemberAccount>(
      `/organizations/me/members/${memberId}`
    );
    return response.data;
  },

  /**
   * Invite a new member
   */
  async invite(request: InviteMemberRequest): Promise<MemberAccount> {
    const response = await httpClient.post<MemberAccount>(
      '/organizations/me/members/invite',
      request
    );
    return response.data;
  },

  /**
   * Resend invitation email
   */
  async resendInvite(memberId: string): Promise<void> {
    await httpClient.post(`/organizations/me/members/${memberId}/resend-invite`);
  },

  /**
   * Update member role
   */
  async updateRole(
    memberId: string,
    role: MemberRole,
    scopes?: string[]
  ): Promise<MemberAccount> {
    const response = await httpClient.patch<MemberAccount>(
      `/organizations/me/members/${memberId}/role`,
      { role, scopes }
    );
    return response.data;
  },

  /**
   * Remove member from organization
   */
  async remove(memberId: string): Promise<void> {
    await httpClient.delete(`/organizations/me/members/${memberId}`);
  },

  /**
   * Revoke pending invitation
   */
  async revokeInvite(memberId: string): Promise<void> {
    await httpClient.post(`/organizations/me/members/${memberId}/revoke-invite`);
  },
};

