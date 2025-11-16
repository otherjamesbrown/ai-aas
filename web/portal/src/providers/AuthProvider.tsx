import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import axios from 'axios';
import { httpClient } from '@/lib/http/client';

interface User {
  id: string;
  email: string;
  roles: string[];
  scopes: string[];
  name?: string;
  organizationId?: string;
}

interface AuthContextValue {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (provider?: string) => Promise<void>;
  loginWithPassword: (email: string, password: string, orgId?: string) => Promise<void>;
  logout: () => void;
  refreshToken: () => Promise<boolean>;
  getAccessToken: () => string | null;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

interface AuthProviderProps {
  children: ReactNode;
  oauthClientId?: string;
  oauthIssuerUrl?: string;
  oauthRedirectUri?: string;
}

/**
 * Auth provider - manages OAuth2/OIDC authentication state
 * Supports silent refresh, MFA prompts, and session management
 */
export function AuthProvider({
  children,
  oauthClientId = import.meta.env.VITE_OAUTH_CLIENT_ID,
  oauthIssuerUrl = import.meta.env.VITE_OAUTH_ISSUER_URL || 'http://localhost:8080',
  oauthRedirectUri = import.meta.env.VITE_OAUTH_REDIRECT_URI || window.location.origin + '/auth/callback',
}: AuthProviderProps) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshTimer, setRefreshTimer] = useState<NodeJS.Timeout | null>(null);

  // Check for existing session on mount
  useEffect(() => {
    const initAuth = async () => {
      try {
        const storedToken = sessionStorage.getItem('auth_token');
        const storedUser = sessionStorage.getItem('auth_user');

        if (storedToken && storedUser) {
          try {
            const parsedUser = JSON.parse(storedUser);
            setUser(parsedUser);
            
            // Verify token is still valid
            await verifyToken(storedToken);
            
            // Schedule token refresh
            scheduleTokenRefresh();
          } catch (error) {
            console.error('Failed to restore session:', error);
            clearAuth();
          }
        }
      } catch (error) {
        console.error('Auth initialization error:', error);
      } finally {
        setIsLoading(false);
      }
    };

    initAuth();

    // Cleanup refresh timer on unmount
    return () => {
      if (refreshTimer) {
        clearTimeout(refreshTimer);
      }
    };
  }, []);

  const verifyToken = async (token: string): Promise<void> => {
    try {
      // Call userinfo endpoint to verify token
      interface UserInfoResponse {
        sub?: string;
        id?: string;
        email: string;
        name?: string;
        roles?: string[];
        scopes?: string[];
        organization_id?: string;
      }
      
      const response = await httpClient.get<UserInfoResponse>('/auth/userinfo', {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      
      // Update user info if needed
      if (response.data) {
        const userInfo: User = {
          id: response.data.sub || response.data.id || '',
          email: response.data.email,
          name: response.data.name,
          roles: response.data.roles || [],
          scopes: response.data.scopes || [],
          organizationId: response.data.organization_id,
        };
        setUser(userInfo);
        sessionStorage.setItem('auth_user', JSON.stringify(userInfo));
      }
    } catch (error) {
      // Token invalid, clear auth
      throw error;
    }
  };

  const refreshToken = async (): Promise<boolean> => {
    try {
      const refreshTokenValue = sessionStorage.getItem('refresh_token');
      if (!refreshTokenValue) {
        return false;
      }

      interface TokenResponse {
        access_token?: string;
        refresh_token?: string;
      }

      const response = await httpClient.post<TokenResponse>('/auth/token', {
        grant_type: 'refresh_token',
        refresh_token: refreshTokenValue,
      });

      if (response.data.access_token) {
        sessionStorage.setItem('auth_token', response.data.access_token);
        if (response.data.refresh_token) {
          sessionStorage.setItem('refresh_token', response.data.refresh_token);
        }
        return true;
      }

      return false;
    } catch (error) {
      console.error('Token refresh failed:', error);
      clearAuth();
      return false;
    }
  };

  const scheduleTokenRefresh = useCallback(() => {
    // Clear existing timer
    if (refreshTimer) {
      clearTimeout(refreshTimer);
    }

    // Refresh token 5 minutes before expiry (assuming 1 hour token lifetime)
    const refreshDelay = 55 * 60 * 1000; // 55 minutes

    const timer = setTimeout(async () => {
      const refreshed = await refreshToken();
      if (refreshed) {
        scheduleTokenRefresh();
      }
    }, refreshDelay);

    setRefreshTimer(timer);
  }, [refreshTimer]);

  const clearAuth = () => {
    setUser(null);
    sessionStorage.removeItem('auth_token');
    sessionStorage.removeItem('refresh_token');
    sessionStorage.removeItem('auth_user');
    if (refreshTimer) {
      clearTimeout(refreshTimer);
      setRefreshTimer(null);
    }
  };

  const login = async (provider: string = 'default') => {
    try {
      // Build OAuth2 authorization URL
      const authUrl = new URL(`${oauthIssuerUrl}/v1/auth/oidc/${provider}/login`);
      authUrl.searchParams.set('redirect_uri', oauthRedirectUri);
      authUrl.searchParams.set('client_id', oauthClientId || '');
      authUrl.searchParams.set('response_type', 'code');
      authUrl.searchParams.set('scope', 'openid profile email');
      authUrl.searchParams.set('state', generateState());

      // Store state for CSRF protection
      sessionStorage.setItem('oauth_state', authUrl.searchParams.get('state') || '');

      // Redirect to OAuth provider
      window.location.href = authUrl.toString();
    } catch (error) {
      console.error('Login initiation failed:', error);
      throw error;
    }
  };

  const loginWithPassword = async (email: string, password: string, orgId?: string): Promise<void> => {
    try {
      interface TokenResponse {
        access_token?: string;
        refresh_token?: string;
        token_type?: string;
        expires_in?: number;
      }

      interface UserInfoResponse {
        sub?: string;
        id?: string;
        email: string;
        name?: string;
        roles?: string[];
        scopes?: string[];
        organization_id?: string;
      }

      // Call password-based login endpoint
      const loginPayload: Record<string, string> = {
        email: email.trim(),
        password,
      };

      if (orgId) {
        loginPayload.org_id = orgId;
      }

      if (oauthClientId) {
        loginPayload.client_id = oauthClientId;
      }

      // Use user-org-service directly for login (same as API key operations)
      const userOrgServiceUrl = import.meta.env.VITE_USER_ORG_SERVICE_URL || 'http://localhost:8081';
      const loginUrl = `${userOrgServiceUrl}/v1/auth/login`;
      console.log('Attempting login to:', loginUrl);
      console.log('Login payload:', { email: loginPayload.email, hasPassword: !!loginPayload.password, orgId: loginPayload.org_id || 'none' });
      
      let response;
      let data: TokenResponse;
      try {
        console.log('About to call fetch...');
        const fetchResponse = await fetch(loginUrl, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(loginPayload),
          signal: AbortSignal.timeout(10000), // 10 second timeout
        });
        console.log('Fetch call completed successfully, status:', fetchResponse.status);

        // Parse response
        data = await fetchResponse.json() as TokenResponse;
        console.log('Response parsed successfully');

        // Create axios-like response object for compatibility
        response = {
          status: fetchResponse.status,
          statusText: fetchResponse.statusText,
          data,
          headers: fetchResponse.headers,
          config: {},
        };
      } catch (fetchError) {
        // Use console.log instead of console.error to ensure Playwright captures it
        console.log('Fetch call failed:', fetchError);
        console.log('Fetch error details:', JSON.stringify({
          message: fetchError instanceof Error ? fetchError.message : String(fetchError),
          name: fetchError instanceof Error ? fetchError.name : undefined,
        }, null, 2));
        throw fetchError; // Re-throw to be caught by outer catch
      }

      console.log('Login response received:', {
        status: response.status,
        hasAccessToken: !!response.data.access_token,
        hasRefreshToken: !!response.data.refresh_token,
        tokenType: response.data.token_type,
        expiresIn: response.data.expires_in,
        responseKeys: Object.keys(response.data),
      });

      if (response.data.access_token) {
        // Store token immediately - don't wait for userinfo
        sessionStorage.setItem('auth_token', response.data.access_token);
        console.log('AuthProvider: access token stored in sessionStorage, length:', response.data.access_token.length);
        if (response.data.refresh_token) {
          sessionStorage.setItem('refresh_token', response.data.refresh_token);
        }

        // Fetch user info (use user-org-service directly)
        // If this fails, we still have the token stored, so don't clear auth
        try {
          const userInfoResponse = await axios.get<UserInfoResponse>(`${userOrgServiceUrl}/v1/auth/userinfo`, {
            headers: {
              Authorization: `Bearer ${response.data.access_token}`,
              'Content-Type': 'application/json',
            },
          });

          const userInfo: User = {
            id: userInfoResponse.data.sub || userInfoResponse.data.id || '',
            email: userInfoResponse.data.email,
            name: userInfoResponse.data.name,
            roles: userInfoResponse.data.roles || [],
            scopes: userInfoResponse.data.scopes || [],
            organizationId: userInfoResponse.data.organization_id,
          };

          setUser(userInfo);
          sessionStorage.setItem('auth_user', JSON.stringify(userInfo));
          console.log('AuthProvider: user info fetched and stored');
        } catch (userInfoError) {
          // Log error but don't fail login - token is already stored
          console.warn('AuthProvider: failed to fetch user info, but token is stored:', userInfoError);
          // Create minimal user info from token if available
          // The token will still work for API calls
        }

        console.log('AuthProvider: login successful, token stored in sessionStorage');

        // Schedule token refresh
        scheduleTokenRefresh();
      } else {
        throw new Error('No access token received');
      }
    } catch (error) {
      console.error('Password login failed:', error);
      console.error('Error details:', {
        errorType: typeof error,
        errorName: error instanceof Error ? error.name : undefined,
        errorMessage: error instanceof Error ? error.message : String(error),
      });
      clearAuth();
      
      // Provide better error messages - handle all error types
      let errorMessage = 'Login failed. Please try again.';

      if (error instanceof Error) {
        // Network errors (connection refused, timeout, etc.)
        if (error.name === 'TypeError' && error.message.includes('Failed to fetch')) {
          errorMessage = 'Cannot connect to backend API. Please ensure the API router and user-org services are running.';
        } else if (error.name === 'AbortError' || error.message.includes('timeout')) {
          errorMessage = 'Request timed out. Please try again.';
        } else {
          errorMessage = error.message;
        }
      }
      // Fallback for unknown error types
      else if (typeof error === 'string') {
        errorMessage = error;
      }

      // Always log the full error for debugging
      console.error('Full login error details:', {
        error,
        errorName: error instanceof Error ? error.name : undefined,
        errorMessage: error instanceof Error ? error.message : undefined,
      });
      
      throw new Error(errorMessage);
    }
  };

  const handleCallback = useCallback(async (code: string, state: string) => {
    try {
      // Verify state
      const storedState = sessionStorage.getItem('oauth_state');
      if (state !== storedState) {
        throw new Error('Invalid state parameter');
      }
      sessionStorage.removeItem('oauth_state');

      interface TokenResponse {
        access_token?: string;
        refresh_token?: string;
      }

      interface UserInfoResponse {
        sub?: string;
        id?: string;
        email: string;
        name?: string;
        roles?: string[];
        scopes?: string[];
        organization_id?: string;
      }

      // Exchange code for tokens
      const response = await httpClient.post<TokenResponse>('/auth/token', {
        grant_type: 'authorization_code',
        code,
        redirect_uri: oauthRedirectUri,
        client_id: oauthClientId,
      });

      if (response.data.access_token) {
        sessionStorage.setItem('auth_token', response.data.access_token);
        if (response.data.refresh_token) {
          sessionStorage.setItem('refresh_token', response.data.refresh_token);
        }

        // Fetch user info
        const userInfoResponse = await httpClient.get<UserInfoResponse>('/auth/userinfo', {
          headers: {
            Authorization: `Bearer ${response.data.access_token}`,
          },
        });

        const userInfo: User = {
          id: userInfoResponse.data.sub || userInfoResponse.data.id || '',
          email: userInfoResponse.data.email,
          name: userInfoResponse.data.name,
          roles: userInfoResponse.data.roles || [],
          scopes: userInfoResponse.data.scopes || [],
          organizationId: userInfoResponse.data.organization_id,
        };

        setUser(userInfo);
        sessionStorage.setItem('auth_user', JSON.stringify(userInfo));

        // Schedule token refresh
        scheduleTokenRefresh();

        return true;
      }

      return false;
    } catch (error) {
      console.error('OAuth callback failed:', error);
      clearAuth();
      return false;
    }
  }, [oauthRedirectUri, oauthClientId, scheduleTokenRefresh]);

  const logout = () => {
    // Call logout endpoint
    const token = sessionStorage.getItem('auth_token');
    if (token) {
      httpClient.post('/auth/logout', {}, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }).catch(() => {
        // Ignore errors on logout
      });
    }

    clearAuth();
    
    // Redirect to login
    window.location.href = '/auth/login';
  };

  const getAccessToken = (): string | null => {
    return sessionStorage.getItem('auth_token');
  };

  // Handle OAuth callback if on callback page
  useEffect(() => {
    if (window.location.pathname === '/auth/callback') {
      const params = new URLSearchParams(window.location.search);
      const code = params.get('code');
      const state = params.get('state');

      if (code && state) {
        handleCallback(code, state).then((success) => {
          if (success) {
            // Redirect to home or stored redirect URL
            const redirect = sessionStorage.getItem('auth_redirect') || '/';
            sessionStorage.removeItem('auth_redirect');
            window.location.href = redirect;
          }
        });
      }
    }
  }, [handleCallback]);

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        login,
        loginWithPassword,
        logout,
        refreshToken,
        getAccessToken,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}

// Helper functions
function generateState(): string {
  const array = new Uint8Array(32);
  crypto.getRandomValues(array);
  return Array.from(array, (byte) => byte.toString(16).padStart(2, '0')).join('');
}
