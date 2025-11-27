/**
 * Centralized API configuration
 *
 * This module provides a single source of truth for API URL configuration.
 * It handles dynamic URL detection for nip.io deployments and environment variables.
 *
 * IMPORTANT: This replaces the duplicated URL logic that was previously in:
 * - /src/lib/http/client.ts
 * - /src/providers/AuthProvider.tsx
 * - /src/components/ServiceHealthCheck.tsx
 * - /src/features/admin/api/apiKeys.ts
 */

export interface ApiConfig {
  /** Base URL without /api suffix (e.g., https://api.example.com) */
  baseUrl: string;
  /** Full API URL with /api suffix (e.g., https://api.example.com/api) */
  apiUrl: string;
  /** Whether running via nip.io wildcard DNS */
  isNipIo: boolean;
  /** Detected environment */
  environment: 'local' | 'development' | 'production';
}

/**
 * Compute API configuration based on current hostname.
 * This is called once at module load time.
 */
function computeApiConfig(): ApiConfig {
  const hostname = window.location.hostname;

  // Check if accessing via nip.io (e.g., portal.172.232.58.222.nip.io)
  const nipioMatch = hostname.match(/^portal\.(.+\.nip\.io)$/);

  if (nipioMatch) {
    const baseUrl = `https://api.${nipioMatch[1]}`;
    return {
      baseUrl,
      apiUrl: `${baseUrl}/api`,
      isNipIo: true,
      environment: 'development',
    };
  }

  // Use environment variable or default
  const envBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api';

  // Extract base URL (without /api suffix) for endpoints that don't use /api prefix
  const baseUrl = envBaseUrl.replace(/\/api\/?$/, '');

  // Determine environment
  let environment: 'local' | 'development' | 'production' = 'production';
  if (hostname === 'localhost' || hostname === '127.0.0.1') {
    environment = 'local';
  } else if (hostname.includes('.dev.') || hostname.includes('dev-')) {
    environment = 'development';
  }

  return {
    baseUrl,
    apiUrl: envBaseUrl,
    isNipIo: false,
    environment,
  };
}

/**
 * API configuration singleton.
 * Computed once at module load time and frozen to prevent modification.
 *
 * Usage:
 * ```typescript
 * import { apiConfig } from '@/config/api';
 *
 * // For authenticated API calls (httpClient uses this):
 * fetch(`${apiConfig.apiUrl}/users`);
 *
 * // For auth endpoints that don't use /api prefix:
 * fetch(`${apiConfig.baseUrl}/v1/auth/login`);
 * ```
 */
export const apiConfig: Readonly<ApiConfig> = Object.freeze(computeApiConfig());

/**
 * OAuth configuration from environment variables
 */
export const oauthConfig = {
  clientId: import.meta.env.VITE_OAUTH_CLIENT_ID || '',
  issuerUrl: import.meta.env.VITE_OAUTH_ISSUER_URL || 'http://localhost:8080',
  redirectUri: import.meta.env.VITE_OAUTH_REDIRECT_URI || `${window.location.origin}/auth/callback`,
} as const;
