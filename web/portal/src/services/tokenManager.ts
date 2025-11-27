import { publicClient } from '@/lib/http/client';

interface TokenResponse {
  access_token: string;
  refresh_token?: string;
  expires_in?: number;
  token_type?: string;
}

type TokenEventType = 'refresh' | 'logout' | 'error';
type TokenEventListener = (event: TokenEventType, data?: unknown) => void;

/**
 * TokenManager handles token lifecycle management.
 *
 * Responsibilities:
 * - Schedule automatic token refresh before expiry
 * - Handle refresh failures and logout
 * - Provide clean API without closure issues
 *
 * Usage:
 * ```typescript
 * import { tokenManager } from '@/services/tokenManager';
 *
 * // After successful login
 * tokenManager.setTokens(accessToken, refreshToken, expiresIn);
 *
 * // Listen for events
 * tokenManager.on((event) => {
 *   if (event === 'logout') {
 *     // Handle logout
 *   }
 * });
 * ```
 */
class TokenManager {
  private refreshTimeout: ReturnType<typeof setTimeout> | null = null;
  private listeners: Set<TokenEventListener> = new Set();

  /**
   * Subscribe to token events
   */
  on(listener: TokenEventListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private emit(event: TokenEventType, data?: unknown): void {
    this.listeners.forEach(listener => listener(event, data));
  }

  /**
   * Store tokens and schedule refresh
   */
  setTokens(accessToken: string, refreshToken?: string, expiresIn?: number): void {
    sessionStorage.setItem('auth_token', accessToken);
    if (refreshToken) {
      sessionStorage.setItem('refresh_token', refreshToken);
    }

    // Schedule refresh if we know expiry time
    if (expiresIn) {
      this.scheduleRefresh(expiresIn);
    } else {
      // Default: refresh in 55 minutes (assuming 1 hour token lifetime)
      this.scheduleRefresh(3600);
    }
  }

  /**
   * Get current access token
   */
  getAccessToken(): string | null {
    return sessionStorage.getItem('auth_token');
  }

  /**
   * Get current refresh token
   */
  getRefreshToken(): string | null {
    return sessionStorage.getItem('refresh_token');
  }

  /**
   * Schedule token refresh before expiry
   */
  scheduleRefresh(expiresIn: number): void {
    this.cancelRefresh();

    // Refresh 60 seconds before expiry, minimum 10 seconds
    const refreshIn = Math.max(10, expiresIn - 60) * 1000;

    this.refreshTimeout = setTimeout(() => {
      this.doRefresh();
    }, refreshIn);
  }

  /**
   * Cancel scheduled refresh
   */
  cancelRefresh(): void {
    if (this.refreshTimeout) {
      clearTimeout(this.refreshTimeout);
      this.refreshTimeout = null;
    }
  }

  /**
   * Perform token refresh
   */
  async doRefresh(): Promise<boolean> {
    const refreshToken = this.getRefreshToken();
    if (!refreshToken) {
      this.emit('logout');
      return false;
    }

    try {
      const response = await publicClient.post<TokenResponse>('/v1/auth/token', {
        grant_type: 'refresh_token',
        refresh_token: refreshToken,
      });

      const { access_token, refresh_token, expires_in } = response.data;

      sessionStorage.setItem('auth_token', access_token);
      if (refresh_token) {
        sessionStorage.setItem('refresh_token', refresh_token);
      }

      // Schedule next refresh
      this.scheduleRefresh(expires_in || 3600);
      this.emit('refresh');

      return true;
    } catch (error) {
      console.error('Token refresh failed:', error);
      this.emit('error', error);
      this.clearTokens();
      this.emit('logout');
      return false;
    }
  }

  /**
   * Clear all tokens and cancel refresh
   */
  clearTokens(): void {
    this.cancelRefresh();
    sessionStorage.removeItem('auth_token');
    sessionStorage.removeItem('refresh_token');
    sessionStorage.removeItem('auth_user');
  }

  /**
   * Check if we have a valid token
   */
  hasToken(): boolean {
    return !!this.getAccessToken();
  }
}

/**
 * Singleton token manager instance
 */
export const tokenManager = new TokenManager();
