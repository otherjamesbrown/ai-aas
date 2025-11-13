import { useQuery } from '@tanstack/react-query';
import { usageApi } from '../api';
import type { UsageFilters } from '../types';

/**
 * Hook to fetch usage report with filters
 */
export function useUsageReport(filters: UsageFilters = {}) {
  return useQuery({
    queryKey: ['usage', 'report', filters],
    queryFn: () => usageApi.getReport(filters),
    staleTime: 2 * 60 * 1000, // 2 minutes
    gcTime: 5 * 60 * 1000, // 5 minutes
    retry: (failureCount, error: any) => {
      // Don't retry on degraded state (503/504)
      if (error?.response?.status === 503 || error?.response?.status === 504) {
        return false;
      }
      // Retry up to 2 times for other errors
      return failureCount < 2;
    },
  });
}

/**
 * Hook to fetch available models for filtering
 */
export function useAvailableModels() {
  return useQuery({
    queryKey: ['usage', 'models'],
    queryFn: () => usageApi.getAvailableModels(),
    staleTime: 10 * 60 * 1000, // 10 minutes
  });
}

