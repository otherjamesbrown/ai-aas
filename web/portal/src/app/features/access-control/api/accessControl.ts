import { httpClient } from '@/lib/http/client';

/**
 * Access Control types
 */
export interface Organization {
  id: string;
  name: string;
  slug: string;
  status: 'active' | 'suspended' | 'trial';
  created_at: string;
  updated_at: string;
}

export interface User {
  id: string;
  email: string;
  name?: string;
  organization_id: string;
  roles: string[];
  status: 'active' | 'pending' | 'disabled';
  created_at: string;
  last_login_at?: string;
}

export interface CreateUserRequest {
  email: string;
  name?: string;
  organization_id: string;
  roles: string[];
}

export interface UpdateUserRequest {
  name?: string;
  roles?: string[];
  status?: 'active' | 'disabled';
}

export interface CreateOrgRequest {
  name: string;
  slug?: string;
}

export interface UpdateOrgRequest {
  name?: string;
  status?: 'active' | 'suspended';
}

export interface Credentials {
  type: string;
  name: string;
  configured: boolean;
  last_tested?: string;
  test_result?: 'success' | 'failure';
}

/**
 * Access Control API client
 *
 * Provides UI access to CLI command equivalents:
 * - org: list, create, update, delete
 * - user: list, create, update, delete
 * - apikey: list, create, delete (existing)
 * - credentials: set, list, test
 */
export const accessControlApi = {
  // ========== Organizations ==========

  /**
   * List all organizations
   * Equivalent to: ai-aas-cli org list
   */
  async orgList(): Promise<Organization[]> {
    const response = await httpClient.get<Organization[]>('/v1/admin/organizations');
    return response.data;
  },

  /**
   * Get organization by ID
   */
  async orgGet(id: string): Promise<Organization> {
    const response = await httpClient.get<Organization>(`/v1/admin/organizations/${id}`);
    return response.data;
  },

  /**
   * Create organization
   * Equivalent to: ai-aas-cli org create <name>
   */
  async orgCreate(request: CreateOrgRequest): Promise<Organization> {
    const response = await httpClient.post<Organization>('/v1/admin/organizations', request);
    return response.data;
  },

  /**
   * Update organization
   * Equivalent to: ai-aas-cli org update <id>
   */
  async orgUpdate(id: string, request: UpdateOrgRequest): Promise<Organization> {
    const response = await httpClient.patch<Organization>(`/v1/admin/organizations/${id}`, request);
    return response.data;
  },

  /**
   * Delete organization
   * Equivalent to: ai-aas-cli org delete <id>
   */
  async orgDelete(id: string): Promise<void> {
    await httpClient.delete(`/v1/admin/organizations/${id}`);
  },

  // ========== Users ==========

  /**
   * List all users
   * Equivalent to: ai-aas-cli user list
   */
  async userList(organizationId?: string): Promise<User[]> {
    const params = organizationId ? `?organization_id=${organizationId}` : '';
    const response = await httpClient.get<User[]>(`/v1/admin/users${params}`);
    return response.data;
  },

  /**
   * Get user by ID
   */
  async userGet(id: string): Promise<User> {
    const response = await httpClient.get<User>(`/v1/admin/users/${id}`);
    return response.data;
  },

  /**
   * Create user
   * Equivalent to: ai-aas-cli user create <email> --org <org_id>
   */
  async userCreate(request: CreateUserRequest): Promise<User> {
    const response = await httpClient.post<User>('/v1/admin/users', request);
    return response.data;
  },

  /**
   * Update user
   * Equivalent to: ai-aas-cli user update <id>
   */
  async userUpdate(id: string, request: UpdateUserRequest): Promise<User> {
    const response = await httpClient.patch<User>(`/v1/admin/users/${id}`, request);
    return response.data;
  },

  /**
   * Delete user
   * Equivalent to: ai-aas-cli user delete <id>
   */
  async userDelete(id: string): Promise<void> {
    await httpClient.delete(`/v1/admin/users/${id}`);
  },

  // ========== Credentials ==========

  /**
   * List credentials
   * Equivalent to: ai-aas-cli credentials list
   */
  async credentialsList(): Promise<Credentials[]> {
    const response = await httpClient.get<Credentials[]>('/v1/admin/credentials');
    return response.data;
  },

  /**
   * Set credential
   * Equivalent to: ai-aas-cli credentials set <type> --name <name> --value <value>
   */
  async credentialsSet(type: string, name: string, value: string): Promise<Credentials> {
    const response = await httpClient.post<Credentials>('/v1/admin/credentials', {
      type,
      name,
      value,
    });
    return response.data;
  },

  /**
   * Test credential
   * Equivalent to: ai-aas-cli credentials test <type>
   */
  async credentialsTest(type: string): Promise<{ success: boolean; message: string }> {
    const response = await httpClient.post<{ success: boolean; message: string }>(`/v1/admin/credentials/${type}/test`);
    return response.data;
  },

  /**
   * Delete credential
   * Equivalent to: ai-aas-cli credentials delete <type>
   */
  async credentialsDelete(type: string): Promise<void> {
    await httpClient.delete(`/v1/admin/credentials/${type}`);
  },

  // ========== User Model Access ==========

  /**
   * Show user model access
   * Equivalent to: ai-aas-cli user model-access show <user>
   */
  async userModelAccessShow(userId: string): Promise<{
    user_id: string;
    user_email: string;
    access_mode: 'allowlist' | 'denylist' | 'all';
    models: Array<{
      model_name: string;
      granted: boolean;
      granted_at?: string;
      granted_by?: string;
    }>;
  }> {
    const response = await httpClient.get<{
      user_id: string;
      user_email: string;
      access_mode: 'allowlist' | 'denylist' | 'all';
      models: Array<{
        model_name: string;
        granted: boolean;
        granted_at?: string;
        granted_by?: string;
      }>;
    }>(`/v1/admin/users/${userId}/model-access`);
    return response.data;
  },

  /**
   * Set user model access mode
   * Equivalent to: ai-aas-cli user model-access set-mode <user> <mode>
   */
  async userModelAccessSetMode(userId: string, mode: 'allowlist' | 'denylist' | 'all'): Promise<void> {
    await httpClient.post(`/v1/admin/users/${userId}/model-access/mode`, { mode });
  },

  /**
   * Grant model access to user
   * Equivalent to: ai-aas-cli user model-access grant <user> <model>
   */
  async userModelAccessGrant(userId: string, modelName: string): Promise<void> {
    await httpClient.post(`/v1/admin/users/${userId}/model-access/grant`, { model_name: modelName });
  },

  /**
   * Revoke model access from user
   * Equivalent to: ai-aas-cli user model-access revoke <user> <model>
   */
  async userModelAccessRevoke(userId: string, modelName: string): Promise<void> {
    await httpClient.post(`/v1/admin/users/${userId}/model-access/revoke`, { model_name: modelName });
  },

  /**
   * Grant all models to user
   * Equivalent to: ai-aas-cli user model-access grant-all <user>
   */
  async userModelAccessGrantAll(userId: string): Promise<void> {
    await httpClient.post(`/v1/admin/users/${userId}/model-access/grant-all`);
  },

  /**
   * List available models for access control
   */
  async listAvailableModels(): Promise<string[]> {
    const response = await httpClient.get<string[]>('/v1/admin/models/available');
    return response.data;
  },
};
