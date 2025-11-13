import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { impersonationApi } from '../api/impersonation';
import type { ImpersonationSession } from '../api/impersonation';

interface ImpersonationBannerProps {
  session: ImpersonationSession;
  onRevoke: () => void;
}

/**
 * Session banner with countdown and revoke button
 * Shows when support engineer is impersonating a user
 */
export function ImpersonationBanner({ session, onRevoke }: ImpersonationBannerProps) {
  const navigate = useNavigate();
  const [timeRemaining, setTimeRemaining] = useState<number>(0);
  const [isRevoking, setIsRevoking] = useState(false);

  useEffect(() => {
    const updateCountdown = () => {
      const expiresAt = new Date(session.expires_at).getTime();
      const now = Date.now();
      const remaining = Math.max(0, Math.floor((expiresAt - now) / 1000));
      setTimeRemaining(remaining);

      // Auto-redirect when session expires
      if (remaining === 0) {
        handleSessionExpired();
      }
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);

    return () => clearInterval(interval);
  }, [session.expires_at]);

  const handleSessionExpired = () => {
    navigate({ to: '/support' });
  };

  const handleRevoke = async () => {
    if (window.confirm('Are you sure you want to end this impersonation session?')) {
      setIsRevoking(true);
      try {
        await impersonationApi.revokeSession(session.session_id);
        onRevoke();
        navigate({ to: '/support' });
      } catch (error) {
        console.error('Failed to revoke session:', error);
        alert('Failed to revoke session. Please try again.');
      } finally {
        setIsRevoking(false);
      }
    }
  };

  const formatTimeRemaining = (seconds: number): string => {
    if (seconds < 60) {
      return `${seconds}s`;
    }
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    return `${minutes}m ${remainingSeconds}s`;
  };

  const getWarningLevel = (): 'info' | 'warning' | 'danger' => {
    if (timeRemaining < 60) return 'danger';
    if (timeRemaining < 300) return 'warning'; // 5 minutes
    return 'info';
  };

  const warningLevel = getWarningLevel();
  const bgColor =
    warningLevel === 'danger'
      ? 'bg-red-600'
      : warningLevel === 'warning'
      ? 'bg-yellow-600'
      : 'bg-blue-600';

  return (
    <div className={`${bgColor} text-white px-4 py-3 shadow-lg`} role="alert">
      <div className="max-w-7xl mx-auto flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
          <div>
            <p className="font-medium">
              Impersonation Mode Active
            </p>
            <p className="text-sm opacity-90">
              You are viewing organization data in read-only mode. Session expires in{' '}
              <strong>{formatTimeRemaining(timeRemaining)}</strong>
            </p>
          </div>
        </div>
        <button
          onClick={handleRevoke}
          disabled={isRevoking}
          className="px-4 py-2 bg-white bg-opacity-20 hover:bg-opacity-30 rounded-md text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed transition"
        >
          {isRevoking ? 'Ending...' : 'End Session'}
        </button>
      </div>
    </div>
  );
}

