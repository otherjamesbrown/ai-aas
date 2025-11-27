import axios, { AxiosInstance, AxiosRequestConfig, InternalAxiosRequestConfig } from 'axios';
import { apiConfig } from '@/config/api';

/**
 * HTTP client with authentication and CSRF protection interceptors
 */
export class HttpClient {
  private client: AxiosInstance;

  constructor(baseURL: string, options?: { withAuth?: boolean }) {
    this.client = axios.create({
      baseURL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.setupInterceptors(options?.withAuth ?? true);
  }

  private setupInterceptors(withAuth: boolean): void {
    // Request interceptor: Add correlation ID and optionally auth
    this.client.interceptors.request.use(
      (config: InternalAxiosRequestConfig) => {
        // Always add correlation ID for tracing
        config.headers['X-Correlation-ID'] = crypto.randomUUID();

        // Add CSRF token if present
        const csrfToken = document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]')?.content;
        if (csrfToken) {
          config.headers['X-CSRF-Token'] = csrfToken;
        }

        // Add auth token if this is an authenticated client
        if (withAuth) {
          const token = sessionStorage.getItem('auth_token');
          if (token) {
            config.headers.Authorization = `Bearer ${token}`;
          }
        }

        return config;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor: Handle 401 errors
    // NOTE: We don't do hard redirects here - let the app handle auth state changes
    // This prevents issues where non-critical 401s (like feature flags) redirect away from valid sessions
    if (withAuth) {
      this.client.interceptors.response.use(
        (response) => response,
        async (error) => {
          // Just propagate the error - AuthProvider and ProtectedRoute handle auth redirects
          return Promise.reject(error);
        }
      );
    }
  }

  get<T = unknown>(url: string, config?: AxiosRequestConfig) {
    return this.client.get<T>(url, config);
  }

  post<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return this.client.post<T>(url, data, config);
  }

  put<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return this.client.put<T>(url, data, config);
  }

  patch<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return this.client.patch<T>(url, data, config);
  }

  delete<T = unknown>(url: string, config?: AxiosRequestConfig) {
    return this.client.delete<T>(url, config);
  }
}

/**
 * Authenticated HTTP client for API calls.
 * Uses centralized API configuration.
 * Automatically adds Bearer token from sessionStorage.
 *
 * Usage:
 * ```typescript
 * import { httpClient } from '@/lib/http/client';
 * const response = await httpClient.get('/organizations/me');
 * ```
 */
export const httpClient = new HttpClient(apiConfig.apiUrl);

/**
 * Public HTTP client for unauthenticated requests.
 * Uses base URL (without /api suffix) for auth endpoints.
 * Does NOT add Bearer token or handle 401 redirects.
 *
 * Usage:
 * ```typescript
 * import { publicClient } from '@/lib/http/client';
 * const response = await publicClient.post('/v1/auth/login', { email, password });
 * ```
 */
export const publicClient = new HttpClient(apiConfig.baseUrl, { withAuth: false });
