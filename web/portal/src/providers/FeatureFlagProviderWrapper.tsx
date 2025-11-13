import { ReactNode } from 'react';
import { useAuth } from './AuthProvider';
import { FeatureFlagProvider } from './FeatureFlagProvider';

interface FeatureFlagProviderWrapperProps {
  children: ReactNode;
  apiUrl?: string;
  refreshInterval?: number;
}

/**
 * Wrapper component that connects FeatureFlagProvider to AuthProvider
 * This avoids circular dependencies
 */
export function FeatureFlagProviderWrapper({
  children,
  apiUrl,
  refreshInterval,
}: FeatureFlagProviderWrapperProps) {
  const { user, isAuthenticated } = useAuth();

  return (
    <FeatureFlagProvider
      apiUrl={apiUrl}
      refreshInterval={refreshInterval}
      userId={user?.id}
      organizationId={user?.organizationId}
      isAuthenticated={isAuthenticated}
    >
      {children}
    </FeatureFlagProvider>
  );
}

