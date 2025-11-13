import { useTelemetry } from '@/providers/TelemetryProvider';
import type { ImpersonationSession } from './impersonation';

/**
 * Forward impersonation audit events to telemetry
 */
export function useImpersonationAudit() {
  const telemetry = useTelemetry();

  const logImpersonationStart = async (session: ImpersonationSession) => {
    await telemetry.startSpan('impersonation.start', async (span) => {
      telemetry.addSpanAttribute('impersonation.session_id', session.session_id, span);
      telemetry.addSpanAttribute('impersonation.organization_id', session.organization_id, span);
      telemetry.addSpanAttribute('impersonation.support_user_id', session.support_user_id, span);
      telemetry.addSpanAttribute('impersonation.scope', session.scope, span);
      telemetry.addSpanAttribute('impersonation.expires_at', session.expires_at, span);
    });
  };

  const logImpersonationEnd = async (session: ImpersonationSession, reason: 'expired' | 'revoked' | 'manual') => {
    await telemetry.startSpan('impersonation.end', async (span) => {
      telemetry.addSpanAttribute('impersonation.session_id', session.session_id, span);
      telemetry.addSpanAttribute('impersonation.organization_id', session.organization_id, span);
      telemetry.addSpanAttribute('impersonation.reason', reason, span);
    });
  };

  const logImpersonationAction = async (
    session: ImpersonationSession,
    action: string,
    details?: Record<string, unknown>
  ) => {
    await telemetry.startSpan(`impersonation.action.${action}`, async (span) => {
      telemetry.addSpanAttribute('impersonation.session_id', session.session_id, span);
      telemetry.addSpanAttribute('impersonation.organization_id', session.organization_id, span);
      telemetry.addSpanAttribute('impersonation.action', action, span);
      
      if (details) {
        Object.entries(details).forEach(([key, value]) => {
          telemetry.addSpanAttribute(`impersonation.${key}`, String(value), span);
        });
      }
    });
  };

  return {
    logImpersonationStart,
    logImpersonationEnd,
    logImpersonationAction,
  };
}

