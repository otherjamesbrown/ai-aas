import { useEffect, useState } from 'react';
import { auditApi } from '../api/audit';
import type { AuditEvent } from '../types';

interface AuditEventBannerProps {
  onEventReceived?: (event: AuditEvent) => void;
}

/**
 * Audit event banner component that displays recent audit events
 * Shows toast notifications for new events
 */
export function AuditEventBanner({ onEventReceived }: AuditEventBannerProps) {
  const [latestEvent, setLatestEvent] = useState<AuditEvent | null>(null);
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    // Poll for recent audit events
    const pollInterval = setInterval(async () => {
      try {
        const events = await auditApi.getRecent(1);
        if (events.length > 0) {
          const event = events[0];
          // Only show if this is a new event
          if (!latestEvent || event.event_id !== latestEvent.event_id) {
            setLatestEvent(event);
            setIsVisible(true);
            onEventReceived?.(event);

            // Auto-hide after 5 seconds
            setTimeout(() => setIsVisible(false), 5000);
          }
        }
      } catch (error) {
        console.error('Failed to fetch audit events:', error);
      }
    }, 2000); // Poll every 2 seconds

    return () => clearInterval(pollInterval);
  }, [latestEvent, onEventReceived]);

  if (!isVisible || !latestEvent) return null;

  const isSuccess = latestEvent.result === 'success';
  const bgColor = isSuccess ? 'bg-green-50 border-green-200' : 'bg-red-50 border-red-200';
  const textColor = isSuccess ? 'text-green-800' : 'text-red-800';
  const iconColor = isSuccess ? 'text-green-400' : 'text-red-400';

  return (
    <div
      className={`fixed top-4 right-4 z-50 max-w-md rounded-lg border p-4 shadow-lg ${bgColor} transition-all duration-300`}
      role="alert"
      aria-live="polite"
    >
      <div className="flex items-start">
        <div className={`flex-shrink-0 ${iconColor}`}>
          {isSuccess ? (
            <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 20 20">
              <path
                fillRule="evenodd"
                d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                clipRule="evenodd"
              />
            </svg>
          ) : (
            <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 20 20">
              <path
                fillRule="evenodd"
                d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                clipRule="evenodd"
              />
            </svg>
          )}
        </div>
        <div className="ml-3 flex-1">
          <p className={`text-sm font-medium ${textColor}`}>
            {latestEvent.action} {isSuccess ? 'succeeded' : 'failed'}
          </p>
          <p className={`mt-1 text-sm ${textColor} opacity-75`}>
            {new Date(latestEvent.timestamp).toLocaleString()}
          </p>
        </div>
        <button
          onClick={() => setIsVisible(false)}
          className={`ml-4 flex-shrink-0 ${textColor} hover:opacity-75 focus:outline-none`}
          aria-label="Close notification"
        >
          <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 20 20">
            <path
              fillRule="evenodd"
              d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
              clipRule="evenodd"
            />
          </svg>
        </button>
      </div>
    </div>
  );
}

