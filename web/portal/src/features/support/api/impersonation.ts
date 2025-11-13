import { httpClient } from '@/lib/http/client';

export interface ImpersonationSession {
  session_id: string; // UUID
  organization_id: string; // UUID
  support_user_id: string; // UUID
  requested_at: string; // ISO-8601 timestamp
  expires_at: string; // ISO-8601 timestamp
  scope: 'read-only';
  consent_token?: string;
}

export interface ImpersonationRequest {
  organization_id: string; // UUID
  reason: string; // 10-500 chars
  consent_token: string; // Signed proof of customer approval
  ttl_minutes: number; // 5-120 minutes
}

/**
 * Support impersonation API client
 */
export const impersonationApi = {
  /**
   * Create a new impersonation session
   */
  async createSession(request: ImpersonationRequest): Promise<ImpersonationSession> {
    const response = await httpClient.post<ImpersonationSession>(
      '/support/impersonations',
      request
    );
    return response.data;
  },

  /**
   * Get current active impersonation session
   */
  async getCurrentSession(): Promise<ImpersonationSession | null> {
    try {
      const response = await httpClient.get<ImpersonationSession>(
        '/support/impersonations/current'
      );
      return response.data;
    } catch (error: any) {
      if (error.response?.status === 404) {
        return null; // No active session
      }
      throw error;
    }
  },

  /**
   * Revoke impersonation session
   */
  async revokeSession(sessionId: string): Promise<void> {
    await httpClient.delete(`/support/impersonations/${sessionId}`);
  },

  /**
   * List all active impersonation sessions for the support user
   */
  async listSessions(): Promise<ImpersonationSession[]> {
    const response = await httpClient.get<ImpersonationSession[]>(
      '/support/impersonations'
    );
    return response.data;
  },
};

