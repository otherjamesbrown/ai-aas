import { httpClient } from '@/lib/http/client';
import type {
  OrganizationProfile,
  UpdateOrganizationRequest,
} from '../types';

/**
 * Organization profile API client
 */
export const organizationApi = {
  /**
   * Get current organization profile
   */
  async getProfile(): Promise<OrganizationProfile> {
    const response = await httpClient.get<OrganizationProfile>('/organizations/me');
    return response.data;
  },

  /**
   * Update organization profile
   */
  async updateProfile(
    updates: UpdateOrganizationRequest
  ): Promise<OrganizationProfile> {
    const response = await httpClient.patch<OrganizationProfile>(
      '/organizations/me',
      updates
    );
    return response.data;
  },

  /**
   * Get organization by ID (admin only)
   */
  async getById(orgId: string): Promise<OrganizationProfile> {
    const response = await httpClient.get<OrganizationProfile>(
      `/organizations/${orgId}`
    );
    return response.data;
  },
};

