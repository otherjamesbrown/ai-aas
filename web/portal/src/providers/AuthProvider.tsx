import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import { publicClient, httpClient } from '@/lib/http/client';
import { tokenManager } from '@/services/tokenManager';
import { oauthConfig } from '@/config/api';

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

interface TokenResponse {
  access_token: string;
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

/**
 * Auth provider - manages OAuth2/OIDC authentication state
 *
 * Uses centralized services:
 * - publicClient for unauthenticated auth requests
 * - httpClient for authenticated requests
 * - tokenManager for token lifecycle
 */
export function AuthProvider({
  children,
  oauthClientId = oauthConfig.clientId,
  oauthIssuerUrl = oauthConfig.issuerUrl,
  oauthRedirectUri = oauthConfig.redirectUri,
}: AuthProviderProps) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Check for existing session on mount
  useEffect(() => {
    const initAuth = async () => {
      try {
        const storedToken = tokenManager.getAccessToken();
        const storedUser = sessionStorage.getItem('auth_user');

        if (storedToken && storedUser) {
          try {
            const parsedUser = JSON.parse(storedUser);
            setUser(parsedUser);

            // Verify token is still valid
            await verifyToken();

            // tokenManager will handle refresh scheduling
            tokenManager.scheduleRefresh(3300); // 55 minutes default
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

    // Subscribe to token manager events
    const unsubscribe = tokenManager.on((event) => {
      if (event === 'logout') {
        setUser(null);
        window.location.href = '/auth/login';
      }
    });

    initAuth();

    return () => {
      unsubscribe();
      tokenManager.cancelRefresh();
    };
  }, []);

  const verifyToken = async (): Promise<void> => {
    try {
      const response = await httpClient.get<UserInfoResponse>('/auth/userinfo');

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
      throw error;
    }
  };

  const refreshToken = async (): Promise<boolean> => {
    return tokenManager.doRefresh();
  };

  const clearAuth = () => {
    setUser(null);
    tokenManager.clearTokens();
  };

  const login = async (provider: string = 'default') => {
    try {
      const authUrl = new URL(`${oauthIssuerUrl}/v1/auth/oidc/${provider}/login`);
      authUrl.searchParams.set('redirect_uri', oauthRedirectUri);
      authUrl.searchParams.set('client_id', oauthClientId || '');
      authUrl.searchParams.set('response_type', 'code');
      authUrl.searchParams.set('scope', 'openid profile email');
      authUrl.searchParams.set('state', generateState());

      sessionStorage.setItem('oauth_state', authUrl.searchParams.get('state') || '');
      window.location.href = authUrl.toString();
    } catch (error) {
      console.error('Login initiation failed:', error);
      throw error;
    }
  };

  const loginWithPassword = async (email: string, password: string, orgId?: string): Promise<void> => {
    try {
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

      // Use publicClient for login (no auth required)
      const response = await publicClient.post<TokenResponse>('/v1/auth/login', loginPayload);

      if (response.data.access_token) {
        // Store tokens via tokenManager
        tokenManager.setTokens(
          response.data.access_token,
          response.data.refresh_token,
          response.data.expires_in
        );

        // Fetch user info
        try {
          const userInfoResponse = await httpClient.get<UserInfoResponse>('/auth/userinfo');

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
        } catch (userInfoError) {
          // Token is stored, login successful even without user info
          console.warn('Failed to fetch user info:', userInfoError);
        }
      } else {
        throw new Error('No access token received');
      }
    } catch (error) {
      console.error('Password login failed:', error);
      clearAuth();

      let errorMessage = 'Login failed. Please try again.';

      if (error instanceof Error) {
        if (error.message.includes('Network Error') || error.message.includes('timeout')) {
          errorMessage = 'Cannot connect to backend API. Please check your connection.';
        } else if (error.message.includes('401') || error.message.includes('403')) {
          errorMessage = 'Invalid email or password.';
        } else {
          errorMessage = error.message;
        }
      }

      throw new Error(errorMessage);
    }
  };

  const handleCallback = useCallback(async (code: string, state: string) => {
    try {
      const storedState = sessionStorage.getItem('oauth_state');
      if (state !== storedState) {
        throw new Error('Invalid state parameter');
      }
      sessionStorage.removeItem('oauth_state');

      const response = await publicClient.post<TokenResponse>('/v1/auth/token', {
        grant_type: 'authorization_code',
        code,
        redirect_uri: oauthRedirectUri,
        client_id: oauthClientId,
      });

      if (response.data.access_token) {
        tokenManager.setTokens(
          response.data.access_token,
          response.data.refresh_token,
          response.data.expires_in
        );

        const userInfoResponse = await httpClient.get<UserInfoResponse>('/auth/userinfo');

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

        return true;
      }

      return false;
    } catch (error) {
      console.error('OAuth callback failed:', error);
      clearAuth();
      return false;
    }
  }, [oauthRedirectUri, oauthClientId]);

  const logout = () => {
    const token = tokenManager.getAccessToken();
    if (token) {
      httpClient.post('/auth/logout', {}).catch(() => {
        // Ignore errors on logout
      });
    }

    clearAuth();
    window.location.href = '/auth/login';
  };

  const getAccessToken = (): string | null => {
    return tokenManager.getAccessToken();
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

function generateState(): string {
  const array = new Uint8Array(32);
  crypto.getRandomValues(array);
  return Array.from(array, (byte) => byte.toString(16).padStart(2, '0')).join('');
}
