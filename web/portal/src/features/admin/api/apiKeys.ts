import axios from 'axios';
import type {
  ApiKeyCredential,
  ApiKeyResponse,
  CreateApiKeyRequest,
} from '../types';

// Use API router for API key operations
// Extract base URL from VITE_API_BASE_URL (remove /api suffix if present)
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';
const userOrgServiceUrl = apiBaseUrl.replace(/\/api\/?$/, '');

/**
 * API key lifecycle management API client
 */
export const apiKeysApi = {
  /**
   * List all API keys for the organization
   */
  async list(): Promise<ApiKeyCredential[]> {
    const token = sessionStorage.getItem('auth_token');
    if (!token) {
      throw new Error('Authentication required. Please log in.');
    }
    const response = await axios.get<ApiKeyCredential[]>(
      `${userOrgServiceUrl}/organizations/me/api-keys`,
      {
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      }
    );
    return response.data;
  },

  /**
   * Get API key by ID
   */
  async getById(keyId: string): Promise<ApiKeyCredential> {
    const token = sessionStorage.getItem('auth_token');
    if (!token) {
      throw new Error('Authentication required. Please log in.');
    }
    const response = await axios.get<ApiKeyCredential>(
      `${userOrgServiceUrl}/organizations/me/api-keys/${keyId}`,
      {
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      }
    );
    return response.data;
  },

  /**
   * Create a new API key
   * Returns the key with the full secret (only shown once)
   */
  async create(request: CreateApiKeyRequest): Promise<ApiKeyResponse> {
    console.log('apiKeys.create: called with request:', request);
    const token = sessionStorage.getItem('auth_token');
    console.log('apiKeys.create: token from sessionStorage:', token ? 'present' : 'missing');
    if (!token) {
      throw new Error('Authentication required. Please log in.');
    }
    console.log('API Key create: URL:', `${userOrgServiceUrl}/organizations/me/api-keys`);
    console.log('API Key create: Token present:', !!token);
    console.log('API Key create: Request:', request);
    try {
      const response = await axios.post<ApiKeyResponse>(
        `${userOrgServiceUrl}/organizations/me/api-keys`,
        request,
        {
          headers: {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        }
      );
      console.log('API Key create: Success:', response.data);
      return response.data;
    } catch (error) {
      console.error('API Key create: Error:', error);
      throw error;
    }
  },

  /**
   * Rotate an API key (generates new secret)
   */
  async rotate(keyId: string): Promise<ApiKeyResponse> {
    const token = sessionStorage.getItem('auth_token');
    if (!token) {
      throw new Error('Authentication required. Please log in.');
    }
    const response = await axios.post<ApiKeyResponse>(
      `${userOrgServiceUrl}/organizations/me/api-keys/${keyId}/rotate`,
      {},
      {
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      }
    );
    return response.data;
  },

  /**
   * Revoke an API key
   * Idempotent - repeated revokes warn without duplicating audit entries
   */
  async revoke(keyId: string): Promise<void> {
    const token = sessionStorage.getItem('auth_token');
    if (!token) {
      throw new Error('Authentication required. Please log in.');
    }
    await axios.post(
      `${userOrgServiceUrl}/organizations/me/api-keys/${keyId}/revoke`,
      {},
      {
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      }
    );
  },

  /**
   * Update API key display name
   */
  async updateName(keyId: string, displayName: string): Promise<ApiKeyCredential> {
    const token = sessionStorage.getItem('auth_token');
    if (!token) {
      throw new Error('Authentication required. Please log in.');
    }
    const response = await axios.patch<ApiKeyCredential>(
      `${userOrgServiceUrl}/organizations/me/api-keys/${keyId}`,
      { display_name: displayName },
      {
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      }
    );
    return response.data;
  },
};

