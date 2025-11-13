import { httpClient } from '@/lib/http/client';
import type { UsageApiResponse, UsageFilters } from './types';

/**
 * Usage API client with degraded state handling
 */
export const usageApi = {
  /**
   * Get usage report with filters
   */
  async getReport(filters: UsageFilters = {}): Promise<UsageApiResponse> {
    try {
      const params = new URLSearchParams();
      
      if (filters.time_range) {
        params.append('time_range', filters.time_range);
      }
      if (filters.start_date) {
        params.append('start_date', filters.start_date);
      }
      if (filters.end_date) {
        params.append('end_date', filters.end_date);
      }
      if (filters.model) {
        params.append('model', filters.model);
      }
      if (filters.operation) {
        params.append('operation', filters.operation);
      }

      const response = await httpClient.get<UsageApiResponse>(
        `/organizations/me/usage?${params.toString()}`
      );

      return response.data;
    } catch (error: any) {
      // Handle degraded state - return cached/degraded data if available
      if (error.response?.status === 503 || error.response?.status === 504) {
        // Service unavailable - return degraded state
        return {
          report: {
            time_range: filters.time_range || 'last_7d',
            totals: { requests: 0, tokens: 0, cost_cents: 0 },
            breakdowns: [],
            generated_at: new Date().toISOString(),
            source: 'degraded',
          },
          sync_status: 'degraded',
        };
      }

      // Re-throw other errors
      throw error;
    }
  },

  /**
   * Get available models for filtering
   */
  async getAvailableModels(): Promise<string[]> {
    try {
      const response = await httpClient.get<string[]>(
        '/organizations/me/usage/models'
      );
      return response.data;
    } catch (error) {
      console.error('Failed to fetch available models:', error);
      return [];
    }
  },

  /**
   * Export usage data as CSV
   * Returns a blob URL for download
   */
  async exportCsv(filters: UsageFilters = {}): Promise<Blob> {
    const params = new URLSearchParams();
    
    if (filters.time_range) {
      params.append('time_range', filters.time_range);
    }
    if (filters.start_date) {
      params.append('start_date', filters.start_date);
    }
    if (filters.end_date) {
      params.append('end_date', filters.end_date);
    }
    if (filters.model) {
      params.append('model', filters.model);
    }
    if (filters.operation) {
      params.append('operation', filters.operation);
    }

    const response = await httpClient.get<Blob>(
      `/organizations/me/usage/export?${params.toString()}`,
      {
        responseType: 'blob',
      }
    );

    return response.data;
  },
};

