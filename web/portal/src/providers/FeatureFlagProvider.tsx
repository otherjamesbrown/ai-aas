import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import { httpClient } from '@/lib/http/client';

interface FeatureFlagContextValue {
  isEnabled: (flag: string) => boolean;
  flags: Record<string, boolean>;
  isLoading: boolean;
  refresh: () => Promise<void>;
}

const FeatureFlagContext = createContext<FeatureFlagContextValue | undefined>(undefined);

interface FeatureFlagProviderProps {
  children: ReactNode;
  apiUrl?: string;
  refreshInterval?: number; // milliseconds
  userId?: string;
  organizationId?: string;
  isAuthenticated?: boolean;
}

/**
 * Feature flag provider - manages feature flags from API
 * Supports per-organization and per-user flags
 */
export function FeatureFlagProvider({
  children,
  apiUrl: _apiUrl = import.meta.env.VITE_FEATURE_FLAGS_API_URL || 'http://localhost:8080/api',
  refreshInterval = 5 * 60 * 1000, // 5 minutes default
  userId,
  organizationId,
  isAuthenticated = false,
}: FeatureFlagProviderProps) {
  const [flags, setFlags] = useState<Record<string, boolean>>({});
  const [isLoading, setIsLoading] = useState(true);

  const fetchFlags = useCallback(async () => {
    if (!isAuthenticated || !userId) {
      // Use default flags for unauthenticated users
      setFlags({
        'budget-controls': false,
        'impersonation': false,
        'usage-insights': false,
      });
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);

      // Fetch feature flags from API
      // API should return flags based on user's organization and role
      const response = await httpClient.get<Record<string, boolean>>('/feature-flags', {
        params: {
          organization_id: organizationId,
          user_id: userId,
        },
      });

      if (response.data) {
        setFlags(response.data);
      } else {
        // Fallback to defaults if API returns empty
        setFlags(getDefaultFlags());
      }
    } catch (err) {
      console.error('Failed to fetch feature flags:', err);
      
      // Use default flags on error
      setFlags(getDefaultFlags());
    } finally {
      setIsLoading(false);
    }
  }, [isAuthenticated, userId, organizationId]);

  // Initial fetch
  useEffect(() => {
    fetchFlags();
  }, [fetchFlags]);

  // Set up periodic refresh
  useEffect(() => {
    if (!isAuthenticated) {
      return;
    }

    const interval = setInterval(() => {
      fetchFlags();
    }, refreshInterval);

    return () => clearInterval(interval);
  }, [isAuthenticated, refreshInterval, fetchFlags]);

  const isEnabled = useCallback(
    (flag: string): boolean => {
      // Check if flag exists in flags object
      if (flag in flags) {
        return flags[flag];
      }

      // Default to false for unknown flags
      return false;
    },
    [flags]
  );

  const refresh = useCallback(async () => {
    await fetchFlags();
  }, [fetchFlags]);

  return (
    <FeatureFlagContext.Provider
      value={{
        isEnabled,
        flags,
        isLoading,
        refresh,
      }}
    >
      {children}
    </FeatureFlagContext.Provider>
  );
}

export function useFeatureFlags() {
  const context = useContext(FeatureFlagContext);
  if (!context) {
    throw new Error('useFeatureFlags must be used within FeatureFlagProvider');
  }
  return context;
}

// Default flags (used as fallback)
function getDefaultFlags(): Record<string, boolean> {
  return {
    'budget-controls': true,
    'impersonation': true,
    'usage-insights': true,
    'api-key-management': true,
    'member-invites': true,
  };
}
