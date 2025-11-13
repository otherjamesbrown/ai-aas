import { httpClient } from '@/lib/http/client';
import type { AuditEvent } from '../types';

/**
 * Audit log API client
 */
export const auditApi = {
  /**
   * Get recent audit events for the organization
   */
  async getRecent(limit = 50): Promise<AuditEvent[]> {
    const response = await httpClient.get<AuditEvent[]>(
      `/organizations/me/audit?limit=${limit}`
    );
    return response.data;
  },

  /**
   * Get audit event by ID
   */
  async getById(eventId: string): Promise<AuditEvent> {
    const response = await httpClient.get<AuditEvent>(
      `/organizations/me/audit/${eventId}`
    );
    return response.data;
  },

  /**
   * Search audit events with filters
   */
  async search(filters: {
    action?: string;
    actor_id?: string;
    result?: 'success' | 'failure';
    since?: string; // ISO-8601 timestamp
    until?: string; // ISO-8601 timestamp
    limit?: number;
  }): Promise<AuditEvent[]> {
    const params = new URLSearchParams();
    if (filters.action) params.append('action', filters.action);
    if (filters.actor_id) params.append('actor_id', filters.actor_id);
    if (filters.result) params.append('result', filters.result);
    if (filters.since) params.append('since', filters.since);
    if (filters.until) params.append('until', filters.until);
    if (filters.limit) params.append('limit', String(filters.limit));

    const response = await httpClient.get<AuditEvent[]>(
      `/organizations/me/audit/search?${params.toString()}`
    );
    return response.data;
  },
};

