import { httpClient } from '@/lib/http/client';
import type {
  ApiKeyCredential,
  ApiKeyResponse,
  CreateApiKeyRequest,
} from '../types';

/**
 * API key lifecycle management API client
 */
export const apiKeysApi = {
  /**
   * List all API keys for the organization
   */
  async list(): Promise<ApiKeyCredential[]> {
    const response = await httpClient.get<ApiKeyCredential[]>(
      '/organizations/me/api-keys'
    );
    return response.data;
  },

  /**
   * Get API key by ID
   */
  async getById(keyId: string): Promise<ApiKeyCredential> {
    const response = await httpClient.get<ApiKeyCredential>(
      `/organizations/me/api-keys/${keyId}`
    );
    return response.data;
  },

  /**
   * Create a new API key
   * Returns the key with the full secret (only shown once)
   */
  async create(request: CreateApiKeyRequest): Promise<ApiKeyResponse> {
    const response = await httpClient.post<ApiKeyResponse>(
      '/organizations/me/api-keys',
      request
    );
    return response.data;
  },

  /**
   * Rotate an API key (generates new secret)
   */
  async rotate(keyId: string): Promise<ApiKeyResponse> {
    const response = await httpClient.post<ApiKeyResponse>(
      `/organizations/me/api-keys/${keyId}/rotate`
    );
    return response.data;
  },

  /**
   * Revoke an API key
   * Idempotent - repeated revokes warn without duplicating audit entries
   */
  async revoke(keyId: string): Promise<void> {
    await httpClient.post(`/organizations/me/api-keys/${keyId}/revoke`);
  },

  /**
   * Update API key display name
   */
  async updateName(keyId: string, displayName: string): Promise<ApiKeyCredential> {
    const response = await httpClient.patch<ApiKeyCredential>(
      `/organizations/me/api-keys/${keyId}`,
      { display_name: displayName }
    );
    return response.data;
  },
};

