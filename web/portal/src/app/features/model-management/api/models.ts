import { httpClient } from '@/lib/http/client';

/**
 * Model types for Model Management API
 */
export interface RegistryModel {
  id: string;
  name: string;
  huggingface_id: string;
  display_name?: string;
  status: 'registered' | 'cached' | 'deployed';
  created_at: string;
  updated_at: string;
}

export interface CacheEntry {
  model_name: string;
  size_bytes: number;
  cached_at: string;
  last_accessed: string;
  status: 'cached' | 'downloading' | 'error';
}

export interface Deployment {
  id: string;
  model_name: string;
  environment: string;
  replicas: number;
  status: 'running' | 'pending' | 'failed' | 'stopped';
  endpoint?: string;
  created_at: string;
  updated_at: string;
}

export interface DeploymentStatus {
  deployment_id: string;
  model_name: string;
  pod_status: Array<{
    name: string;
    status: string;
    ready: boolean;
    restarts: number;
  }>;
  endpoint_status: {
    healthy: boolean;
    last_check: string;
  };
}

export interface ModelVersion {
  name: string;
  current_version: string;
  latest_version?: string;
  pinned: boolean;
}

export interface LibraryEntry {
  model_name: string;
  alias?: string;
  enabled: boolean;
  swap_target?: string;
  created_at: string;
}

export interface InferenceModel {
  id: string;
  name: string;
  provider: string;
  available: boolean;
}

/**
 * Model Management API client
 *
 * Provides UI access to CLI command equivalents:
 * - model registry: add, list, info, remove
 * - model cache: pull, list, delete, gc
 * - model deploy: create, delete, scale, status
 * - model troubleshoot: logs, events, test
 * - model version: check, update, pin
 * - model library: enable, disable, swap, list, history, alias
 * - deployment: status
 * - inference: get-models, send-request
 */
export const modelsApi = {
  // ========== Model Registry ==========

  /**
   * List all models in the registry
   * Equivalent to: ai-aas-cli model registry list
   */
  async registryList(): Promise<RegistryModel[]> {
    const response = await httpClient.get<RegistryModel[]>('/v1/admin/models/registry');
    return response.data;
  },

  /**
   * Get model info
   * Equivalent to: ai-aas-cli model registry info <name>
   */
  async registryInfo(name: string): Promise<RegistryModel> {
    const response = await httpClient.get<RegistryModel>(`/v1/admin/models/registry/${name}`);
    return response.data;
  },

  /**
   * Add model to registry
   * Equivalent to: ai-aas-cli model registry add <huggingface_id> --name <name>
   */
  async registryAdd(huggingfaceId: string, name?: string, displayName?: string): Promise<RegistryModel> {
    const response = await httpClient.post<RegistryModel>('/v1/admin/models/registry', {
      huggingface_id: huggingfaceId,
      name,
      display_name: displayName,
    });
    return response.data;
  },

  /**
   * Remove model from registry
   * Equivalent to: ai-aas-cli model registry remove <name>
   */
  async registryRemove(name: string): Promise<void> {
    await httpClient.delete(`/v1/admin/models/registry/${name}`);
  },

  // ========== Model Cache ==========

  /**
   * List cached models
   * Equivalent to: ai-aas-cli model cache list
   */
  async cacheList(): Promise<CacheEntry[]> {
    const response = await httpClient.get<CacheEntry[]>('/v1/admin/models/cache');
    return response.data;
  },

  /**
   * Pull model to cache
   * Equivalent to: ai-aas-cli model cache pull <name>
   */
  async cachePull(name: string): Promise<{ job_id: string }> {
    const response = await httpClient.post<{ job_id: string }>(`/v1/admin/models/cache/${name}/pull`);
    return response.data;
  },

  /**
   * Delete model from cache
   * Equivalent to: ai-aas-cli model cache delete <name>
   */
  async cacheDelete(name: string): Promise<void> {
    await httpClient.delete(`/v1/admin/models/cache/${name}`);
  },

  /**
   * Garbage collect cache
   * Equivalent to: ai-aas-cli model cache gc
   */
  async cacheGc(): Promise<{ deleted_count: number; freed_bytes: number }> {
    const response = await httpClient.post<{ deleted_count: number; freed_bytes: number }>('/v1/admin/models/cache/gc');
    return response.data;
  },

  // ========== Model Deploy ==========

  /**
   * List deployments
   * Equivalent to: ai-aas-cli model deploy list
   */
  async deployList(environment?: string): Promise<Deployment[]> {
    const params = environment ? `?environment=${environment}` : '';
    const response = await httpClient.get<Deployment[]>(`/v1/admin/models/deployments${params}`);
    return response.data;
  },

  /**
   * Create deployment
   * Equivalent to: ai-aas-cli model deploy create <name> -e <env>
   */
  async deployCreate(name: string, environment: string, replicas?: number): Promise<Deployment> {
    const response = await httpClient.post<Deployment>('/v1/admin/models/deployments', {
      model_name: name,
      environment,
      replicas: replicas || 1,
    });
    return response.data;
  },

  /**
   * Delete deployment
   * Equivalent to: ai-aas-cli model deploy delete <name> -e <env>
   */
  async deployDelete(name: string, environment: string): Promise<void> {
    await httpClient.delete(`/v1/admin/models/deployments/${name}?environment=${environment}`);
  },

  /**
   * Scale deployment
   * Equivalent to: ai-aas-cli model deploy scale <name> --replicas <n>
   */
  async deployScale(name: string, environment: string, replicas: number): Promise<Deployment> {
    const response = await httpClient.patch<Deployment>(`/v1/admin/models/deployments/${name}`, {
      environment,
      replicas,
    });
    return response.data;
  },

  /**
   * Get deployment status
   * Equivalent to: ai-aas-cli model deploy status <name>
   */
  async deployStatus(name: string, environment: string): Promise<DeploymentStatus> {
    const response = await httpClient.get<DeploymentStatus>(`/v1/admin/models/deployments/${name}/status?environment=${environment}`);
    return response.data;
  },

  // ========== Model Troubleshoot ==========

  /**
   * Get deployment logs
   * Equivalent to: ai-aas-cli model troubleshoot logs <name>
   */
  async troubleshootLogs(name: string, lines?: number): Promise<{ logs: string }> {
    const params = lines ? `?lines=${lines}` : '';
    const response = await httpClient.get<{ logs: string }>(`/v1/admin/models/deployments/${name}/logs${params}`);
    return response.data;
  },

  /**
   * Get deployment events
   * Equivalent to: ai-aas-cli model troubleshoot events <name>
   */
  async troubleshootEvents(name: string): Promise<{ events: Array<{ type: string; reason: string; message: string; last_timestamp: string }> }> {
    const response = await httpClient.get<{ events: Array<{ type: string; reason: string; message: string; last_timestamp: string }> }>(`/v1/admin/models/deployments/${name}/events`);
    return response.data;
  },

  /**
   * Test inference
   * Equivalent to: ai-aas-cli model troubleshoot test-inference <name>
   */
  async troubleshootTestInference(name: string): Promise<{ success: boolean; latency_ms: number; response?: string; error?: string }> {
    const response = await httpClient.post<{ success: boolean; latency_ms: number; response?: string; error?: string }>(`/v1/admin/models/deployments/${name}/test-inference`);
    return response.data;
  },

  // ========== Model Version ==========

  /**
   * Check all model versions
   * Equivalent to: ai-aas-cli model version check
   */
  async versionCheck(): Promise<Array<{
    model_name: string;
    current_version: string;
    latest_version: string;
    pinned: boolean;
    update_available: boolean;
    source: string;
  }>> {
    const response = await httpClient.get<Array<{
      model_name: string;
      current_version: string;
      latest_version: string;
      pinned: boolean;
      update_available: boolean;
      source: string;
    }>>('/v1/admin/models/versions');
    return response.data;
  },

  /**
   * Update model version
   * Equivalent to: ai-aas-cli model version update <name>
   */
  async versionUpdate(name: string): Promise<RegistryModel> {
    const response = await httpClient.post<RegistryModel>(`/v1/admin/models/registry/${name}/version/update`);
    return response.data;
  },

  /**
   * Pin model version
   * Equivalent to: ai-aas-cli model version pin <name> <version>
   */
  async versionPin(name: string, version: string): Promise<RegistryModel> {
    const response = await httpClient.post<RegistryModel>(`/v1/admin/models/registry/${name}/version/pin`, {
      version,
    });
    return response.data;
  },

  /**
   * Unpin model version
   * Equivalent to: ai-aas-cli model version unpin <name>
   */
  async versionUnpin(name: string): Promise<RegistryModel> {
    const response = await httpClient.post<RegistryModel>(`/v1/admin/models/registry/${name}/version/unpin`);
    return response.data;
  },

  // ========== Model Library ==========

  /**
   * List library entries
   * Equivalent to: ai-aas-cli model library list
   */
  async libraryList(): Promise<LibraryEntry[]> {
    const response = await httpClient.get<LibraryEntry[]>('/v1/admin/models/library');
    return response.data;
  },

  /**
   * Enable model in library
   * Equivalent to: ai-aas-cli model library enable <name>
   */
  async libraryEnable(name: string): Promise<LibraryEntry> {
    const response = await httpClient.post<LibraryEntry>(`/v1/admin/models/library/${name}/enable`);
    return response.data;
  },

  /**
   * Disable model in library
   * Equivalent to: ai-aas-cli model library disable <name>
   */
  async libraryDisable(name: string): Promise<LibraryEntry> {
    const response = await httpClient.post<LibraryEntry>(`/v1/admin/models/library/${name}/disable`);
    return response.data;
  },

  /**
   * Swap model
   * Equivalent to: ai-aas-cli model library swap <name> --target <target>
   */
  async librarySwap(name: string, target: string): Promise<LibraryEntry> {
    const response = await httpClient.post<LibraryEntry>(`/v1/admin/models/library/${name}/swap`, {
      target,
    });
    return response.data;
  },

  /**
   * Set model alias
   * Equivalent to: ai-aas-cli model library alias <name> --alias <alias>
   */
  async libraryAlias(name: string, alias: string): Promise<LibraryEntry> {
    const response = await httpClient.post<LibraryEntry>(`/v1/admin/models/library/${name}/alias`, {
      alias,
    });
    return response.data;
  },

  /**
   * Get library history
   * Equivalent to: ai-aas-cli model library history <name>
   */
  async libraryHistory(name: string): Promise<Array<{ action: string; timestamp: string; details: string }>> {
    const response = await httpClient.get<Array<{ action: string; timestamp: string; details: string }>>(`/v1/admin/models/library/${name}/history`);
    return response.data;
  },

  // ========== Deployment Status ==========

  /**
   * Get multi-source deployment status
   * Equivalent to: ai-aas-cli deployment status
   */
  async deploymentStatus(): Promise<{
    deployments: Array<{
      name: string;
      environment: string;
      kubernetes_status: string;
      api_router_status: string;
      inference_status: string;
    }>;
  }> {
    const response = await httpClient.get<{
      deployments: Array<{
        name: string;
        environment: string;
        kubernetes_status: string;
        api_router_status: string;
        inference_status: string;
      }>;
    }>('/v1/admin/deployments/status');
    return response.data;
  },

  // ========== Inference ==========

  /**
   * Get available inference models
   * Equivalent to: ai-aas-cli inference get-models
   */
  async inferenceGetModels(): Promise<InferenceModel[]> {
    const response = await httpClient.get<InferenceModel[]>('/v1/inference/models');
    return response.data;
  },

  /**
   * Send inference request
   * Equivalent to: ai-aas-cli inference send-request
   */
  async inferenceSendRequest(modelId: string, prompt: string, options?: {
    max_tokens?: number;
    temperature?: number;
  }): Promise<{ response: string; tokens_used: number; latency_ms: number }> {
    const response = await httpClient.post<{ response: string; tokens_used: number; latency_ms: number }>('/v1/inference/completions', {
      model: modelId,
      prompt,
      ...options,
    });
    return response.data;
  },
};
