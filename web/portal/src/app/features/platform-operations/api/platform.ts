import { httpClient, publicClient } from '@/lib/http/client';

/**
 * Platform Operations types
 */
export interface PlatformStatus {
  overall: 'healthy' | 'degraded' | 'unhealthy';
  services: Record<string, {
    status: string;
    message?: string;
    response_ms?: number;
  }>;
  components: Record<string, {
    status: string;
    message?: string;
  }>;
}

export interface ConfigEntry {
  key: string;
  value: string;
  type: 'string' | 'number' | 'boolean' | 'json';
  description?: string;
  environment?: string;
  source: 'default' | 'env' | 'config' | 'runtime';
}

export interface Backup {
  id: string;
  name: string;
  type: 'full' | 'incremental';
  status: 'completed' | 'in_progress' | 'failed';
  size_bytes: number;
  created_at: string;
  expires_at?: string;
}

export interface ArgoApplication {
  name: string;
  namespace: string;
  project: string;
  sync_status: 'Synced' | 'OutOfSync' | 'Unknown';
  health_status: 'Healthy' | 'Degraded' | 'Progressing' | 'Missing' | 'Unknown';
  source: {
    repo_url: string;
    path: string;
    target_revision: string;
  };
  last_sync_time?: string;
}

/**
 * Platform Operations API client
 *
 * Provides UI access to CLI command equivalents:
 * - status: Overall platform status
 * - config: Configuration management
 * - backup: Backup operations
 * - gitops: ArgoCD application status
 */
export const platformApi = {
  // ========== Status ==========

  /**
   * Get platform status
   * Equivalent to: ai-aas-cli status
   */
  async getStatus(): Promise<PlatformStatus> {
    const response = await publicClient.get<PlatformStatus>('/v1/platform/health');
    return response.data;
  },

  /**
   * Get health check
   * Equivalent to: ai-aas-cli status --health
   */
  async getHealth(): Promise<{ status: string; message?: string }> {
    const response = await publicClient.get<{ status: string; message?: string }>('/v1/status/healthz');
    return response.data;
  },

  /**
   * Get readiness
   * Equivalent to: ai-aas-cli status --ready
   */
  async getReadiness(): Promise<{
    status: string;
    components?: Record<string, string | { status: string; message?: string }>;
  }> {
    const response = await publicClient.get<{
      status: string;
      components?: Record<string, string | { status: string; message?: string }>;
    }>('/v1/status/readyz');
    return response.data;
  },

  // ========== Config ==========

  /**
   * Show all configuration
   * Equivalent to: ai-aas-cli config show
   */
  async configShow(): Promise<Array<{
    key: string;
    value: string;
    description?: string;
    editable: boolean;
    type: 'string' | 'number' | 'boolean' | 'json';
  }>> {
    const response = await httpClient.get<Array<{
      key: string;
      value: string;
      description?: string;
      editable: boolean;
      type: 'string' | 'number' | 'boolean' | 'json';
    }>>('/v1/admin/config');
    return response.data;
  },

  /**
   * Test configuration validity
   * Equivalent to: ai-aas-cli config test
   */
  async configTest(): Promise<{
    valid: boolean;
    message: string;
    errors?: string[];
  }> {
    const response = await httpClient.post<{
      valid: boolean;
      message: string;
      errors?: string[];
    }>('/v1/admin/config/test');
    return response.data;
  },

  /**
   * List configuration entries
   * Equivalent to: ai-aas-cli config list
   */
  async configList(environment?: string): Promise<ConfigEntry[]> {
    const params = environment ? `?environment=${environment}` : '';
    const response = await httpClient.get<ConfigEntry[]>(`/v1/admin/config${params}`);
    return response.data;
  },

  /**
   * Get configuration value
   * Equivalent to: ai-aas-cli config get <key>
   */
  async configGet(key: string): Promise<ConfigEntry> {
    const response = await httpClient.get<ConfigEntry>(`/v1/admin/config/${key}`);
    return response.data;
  },

  /**
   * Set configuration value
   * Equivalent to: ai-aas-cli config set <key> <value>
   */
  async configSet(key: string, value: string, type?: string): Promise<ConfigEntry> {
    const response = await httpClient.post<ConfigEntry>('/v1/admin/config', {
      key,
      value,
      type,
    });
    return response.data;
  },

  /**
   * Delete configuration
   * Equivalent to: ai-aas-cli config delete <key>
   */
  async configDelete(key: string): Promise<void> {
    await httpClient.delete(`/v1/admin/config/${key}`);
  },

  // ========== Backup ==========

  /**
   * List backups
   * Equivalent to: ai-aas-cli backup list
   */
  async backupList(): Promise<Backup[]> {
    const response = await httpClient.get<Backup[]>('/v1/admin/backups');
    return response.data;
  },

  /**
   * Create backup
   * Equivalent to: ai-aas-cli backup create
   */
  async backupCreate(name?: string, type: 'full' | 'incremental' = 'full'): Promise<Backup> {
    const response = await httpClient.post<Backup>('/v1/admin/backups', {
      name,
      type,
    });
    return response.data;
  },

  /**
   * Restore from backup
   * Equivalent to: ai-aas-cli backup restore <id>
   */
  async backupRestore(id: string): Promise<{ job_id: string; status: string }> {
    const response = await httpClient.post<{ job_id: string; status: string }>(`/v1/admin/backups/${id}/restore`);
    return response.data;
  },

  /**
   * Delete backup
   * Equivalent to: ai-aas-cli backup delete <id>
   */
  async backupDelete(id: string): Promise<void> {
    await httpClient.delete(`/v1/admin/backups/${id}`);
  },

  // ========== GitOps ==========

  /**
   * List ArgoCD applications
   * Equivalent to: ai-aas-cli gitops list
   */
  async gitopsList(): Promise<ArgoApplication[]> {
    const response = await httpClient.get<ArgoApplication[]>('/v1/admin/gitops/applications');
    return response.data;
  },

  /**
   * Get ArgoCD application details
   * Equivalent to: ai-aas-cli gitops get <name>
   */
  async gitopsGet(name: string): Promise<ArgoApplication> {
    const response = await httpClient.get<ArgoApplication>(`/v1/admin/gitops/applications/${name}`);
    return response.data;
  },

  /**
   * Sync ArgoCD application
   * Equivalent to: ai-aas-cli gitops sync <name>
   */
  async gitopsSync(name: string): Promise<{ status: string; message: string }> {
    const response = await httpClient.post<{ status: string; message: string }>(`/v1/admin/gitops/applications/${name}/sync`);
    return response.data;
  },

  /**
   * Refresh ArgoCD application
   * Equivalent to: ai-aas-cli gitops refresh <name>
   */
  async gitopsRefresh(name: string): Promise<{ status: string }> {
    const response = await httpClient.post<{ status: string }>(`/v1/admin/gitops/applications/${name}/refresh`);
    return response.data;
  },

  // ========== Bootstrap ==========

  /**
   * Bootstrap the platform
   * Equivalent to: ai-aas-cli bootstrap
   */
  async bootstrap(data: {
    admin_email: string;
    admin_password: string;
    org_name: string;
    hf_token?: string;
    skip_model_pull?: boolean;
  }): Promise<{ api_key: string; message: string }> {
    const response = await httpClient.post<{ api_key: string; message: string }>('/v1/admin/bootstrap', data);
    return response.data;
  },

  // ========== Registry ==========

  /**
   * List registered services
   * Equivalent to: ai-aas-cli registry list
   */
  async registryList(): Promise<Array<{
    name: string;
    type: string;
    endpoint: string;
    status: 'enabled' | 'disabled';
    health: 'healthy' | 'unhealthy' | 'unknown';
    registered_at: string;
  }>> {
    const response = await httpClient.get<Array<{
      name: string;
      type: string;
      endpoint: string;
      status: 'enabled' | 'disabled';
      health: 'healthy' | 'unhealthy' | 'unknown';
      registered_at: string;
    }>>('/v1/admin/registry');
    return response.data;
  },

  /**
   * Register a service
   * Equivalent to: ai-aas-cli registry register
   */
  async registryRegister(data: {
    name: string;
    type: string;
    endpoint: string;
  }): Promise<{ message: string }> {
    const response = await httpClient.post<{ message: string }>('/v1/admin/registry', data);
    return response.data;
  },

  /**
   * Deregister a service
   * Equivalent to: ai-aas-cli registry deregister
   */
  async registryDeregister(name: string): Promise<void> {
    await httpClient.delete(`/v1/admin/registry/${name}`);
  },

  /**
   * Enable a service
   * Equivalent to: ai-aas-cli registry enable
   */
  async registryEnable(name: string): Promise<{ message: string }> {
    const response = await httpClient.post<{ message: string }>(`/v1/admin/registry/${name}/enable`);
    return response.data;
  },

  /**
   * Disable a service
   * Equivalent to: ai-aas-cli registry disable
   */
  async registryDisable(name: string): Promise<{ message: string }> {
    const response = await httpClient.post<{ message: string }>(`/v1/admin/registry/${name}/disable`);
    return response.data;
  },

  // ========== Routing Policy ==========

  /**
   * List routing policies
   * Equivalent to: ai-aas-cli routing policy list
   */
  async routingPolicyList(): Promise<Array<{
    id: string;
    name: string;
    priority: number;
    conditions: Record<string, string>;
    target: string;
    enabled: boolean;
    created_at: string;
  }>> {
    const response = await httpClient.get<Array<{
      id: string;
      name: string;
      priority: number;
      conditions: Record<string, string>;
      target: string;
      enabled: boolean;
      created_at: string;
    }>>('/v1/admin/routing/policies');
    return response.data;
  },

  /**
   * Create routing policy
   * Equivalent to: ai-aas-cli routing policy create
   */
  async routingPolicyCreate(data: {
    name: string;
    priority: number;
    conditions: Record<string, string>;
    target: string;
  }): Promise<{ id: string; message: string }> {
    const response = await httpClient.post<{ id: string; message: string }>('/v1/admin/routing/policies', data);
    return response.data;
  },

  /**
   * Delete routing policy
   * Equivalent to: ai-aas-cli routing policy delete
   */
  async routingPolicyDelete(id: string): Promise<void> {
    await httpClient.delete(`/v1/admin/routing/policies/${id}`);
  },

  // ========== Sync ==========

  /**
   * Trigger sync
   * Equivalent to: ai-aas-cli sync trigger
   */
  async syncTrigger(target?: string): Promise<{ job_id: string; message: string }> {
    const response = await httpClient.post<{ job_id: string; message: string }>('/v1/admin/sync/trigger', { target });
    return response.data;
  },

  /**
   * Get sync status
   * Equivalent to: ai-aas-cli sync status
   */
  async syncStatus(): Promise<{
    last_sync: string;
    status: 'idle' | 'syncing' | 'failed';
    targets: Array<{
      name: string;
      last_sync: string;
      status: string;
      message?: string;
    }>;
  }> {
    const response = await httpClient.get<{
      last_sync: string;
      status: 'idle' | 'syncing' | 'failed';
      targets: Array<{
        name: string;
        last_sync: string;
        status: string;
        message?: string;
      }>;
    }>('/v1/admin/sync/status');
    return response.data;
  },

  // ========== Export ==========

  /**
   * Export usage data
   * Equivalent to: ai-aas-cli export usage
   */
  async exportUsage(params?: {
    start_date?: string;
    end_date?: string;
    format?: 'json' | 'csv';
  }): Promise<Blob> {
    const queryParams = new URLSearchParams();
    if (params?.start_date) queryParams.append('start_date', params.start_date);
    if (params?.end_date) queryParams.append('end_date', params.end_date);
    if (params?.format) queryParams.append('format', params.format);

    const response = await httpClient.get<Blob>(`/v1/admin/export/usage?${queryParams}`, {
      responseType: 'blob',
    });
    return response.data;
  },

  /**
   * Export memberships data
   * Equivalent to: ai-aas-cli export memberships
   */
  async exportMemberships(params?: {
    org_id?: string;
    format?: 'json' | 'csv';
  }): Promise<Blob> {
    const queryParams = new URLSearchParams();
    if (params?.org_id) queryParams.append('org_id', params.org_id);
    if (params?.format) queryParams.append('format', params.format);

    const response = await httpClient.get<Blob>(`/v1/admin/export/memberships?${queryParams}`, {
      responseType: 'blob',
    });
    return response.data;
  },
};
