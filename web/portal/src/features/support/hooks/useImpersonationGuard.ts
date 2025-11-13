import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { impersonationApi, type ImpersonationSession } from '../api/impersonation';

interface UseImpersonationGuardResult {
  isImpersonating: boolean;
  session: ImpersonationSession | null;
  isLoading: boolean;
  isReadOnly: boolean;
}

/**
 * Hook to check if current session is an impersonation session
 * Enforces read-only mode when impersonating
 */
export function useImpersonationGuard(): UseImpersonationGuardResult {
  const [session, setSession] = useState<ImpersonationSession | null>(null);

  const { data: currentSession, isLoading } = useQuery({
    queryKey: ['impersonation', 'current'],
    queryFn: () => impersonationApi.getCurrentSession(),
    refetchInterval: 30 * 1000, // Check every 30 seconds
    retry: false,
  });

  useEffect(() => {
    if (currentSession) {
      setSession(currentSession);
      
      // Check if session has expired
      const expiresAt = new Date(currentSession.expires_at).getTime();
      if (Date.now() >= expiresAt) {
        setSession(null);
      }
    } else {
      setSession(null);
    }
  }, [currentSession]);

  const isImpersonating = !!session;
  const isReadOnly = isImpersonating && session?.scope === 'read-only';

  return {
    isImpersonating,
    session,
    isLoading,
    isReadOnly,
  };
}

/**
 * Hook to check if an action should be blocked in read-only mode
 */
export function useReadOnlyGuard(_action: string): {
  isBlocked: boolean;
  message: string;
} {
  const { isReadOnly, session } = useImpersonationGuard();

  if (!isReadOnly) {
    return { isBlocked: false, message: '' };
  }

  return {
    isBlocked: true,
    message: `This action is blocked in read-only impersonation mode. Session expires at ${session ? new Date(session.expires_at).toLocaleString() : 'unknown'}.`,
  };
}

