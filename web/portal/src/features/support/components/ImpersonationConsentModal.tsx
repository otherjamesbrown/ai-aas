import { useState } from 'react';
import type { ImpersonationRequest } from '../api/impersonation';

interface ImpersonationConsentModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (request: ImpersonationRequest) => void | Promise<void>;
  organizationId: string;
  organizationName: string;
  isLoading?: boolean;
}

/**
 * Consent modal for capturing impersonation justification and consent token
 */
export function ImpersonationConsentModal({
  isOpen,
  onClose,
  onSubmit,
  organizationId,
  organizationName,
  isLoading = false,
}: ImpersonationConsentModalProps) {
  const [reason, setReason] = useState('');
  const [consentToken, setConsentToken] = useState('');
  const [ttlMinutes, setTtlMinutes] = useState(60);
  const [errors, setErrors] = useState<{ reason?: string; consentToken?: string }>({});

  if (!isOpen) return null;

  const validate = (): boolean => {
    const newErrors: { reason?: string; consentToken?: string } = {};

    if (!reason.trim() || reason.trim().length < 10) {
      newErrors.reason = 'Reason must be at least 10 characters';
    }
    if (reason.trim().length > 500) {
      newErrors.reason = 'Reason must be less than 500 characters';
    }
    if (!consentToken.trim()) {
      newErrors.consentToken = 'Consent token is required';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    const request: ImpersonationRequest = {
      organization_id: organizationId,
      reason: reason.trim(),
      consent_token: consentToken.trim(),
      ttl_minutes: ttlMinutes,
    };

    await onSubmit(request);
    
    // Reset form on success
    setReason('');
    setConsentToken('');
    setTtlMinutes(60);
    setErrors({});
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto" aria-modal="true">
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="fixed inset-0 bg-gray-500 bg-opacity-75" onClick={onClose} />
        <div className="relative bg-white rounded-lg shadow-xl max-w-lg w-full p-6">
          <h2 className="text-xl font-bold mb-4">Request Impersonation Access</h2>
          
          <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mb-4">
            <p className="text-sm text-yellow-800">
              <strong>Organization:</strong> {organizationName}
            </p>
            <p className="text-sm text-yellow-800 mt-2">
              You are requesting read-only access to view this organization's data. All actions
              will be logged and the session will expire automatically.
            </p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="reason" className="block text-sm font-medium text-gray-700 mb-2">
                Justification <span className="text-red-500">*</span>
              </label>
              <textarea
                id="reason"
                rows={4}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Explain why you need access to this organization's data..."
                className={`block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm ${
                  errors.reason ? 'border-red-300' : ''
                }`}
                required
                minLength={10}
                maxLength={500}
              />
              {errors.reason && (
                <p className="mt-1 text-sm text-red-600">{errors.reason}</p>
              )}
              <p className="mt-1 text-xs text-gray-500">
                {reason.length}/500 characters
              </p>
            </div>

            <div>
              <label htmlFor="consent-token" className="block text-sm font-medium text-gray-700 mb-2">
                Consent Token <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                id="consent-token"
                value={consentToken}
                onChange={(e) => setConsentToken(e.target.value)}
                placeholder="Enter consent token provided by customer"
                className={`block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm ${
                  errors.consentToken ? 'border-red-300' : ''
                }`}
                required
              />
              {errors.consentToken && (
                <p className="mt-1 text-sm text-red-600">{errors.consentToken}</p>
              )}
              <p className="mt-1 text-xs text-gray-500">
                Token provided by customer to authorize this access
              </p>
            </div>

            <div>
              <label htmlFor="ttl-minutes" className="block text-sm font-medium text-gray-700 mb-2">
                Session Duration (minutes)
              </label>
              <input
                type="number"
                id="ttl-minutes"
                value={ttlMinutes}
                onChange={(e) => setTtlMinutes(Math.max(5, Math.min(120, parseInt(e.target.value) || 60)))}
                min={5}
                max={120}
                className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
              <p className="mt-1 text-xs text-gray-500">
                Session will expire after {ttlMinutes} minutes (5-120 minutes)
              </p>
            </div>

            <div className="flex justify-end space-x-3 pt-4">
              <button
                type="button"
                onClick={onClose}
                disabled={isLoading}
                className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isLoading}
                className="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark disabled:opacity-50"
              >
                {isLoading ? 'Requesting Access...' : 'Request Access'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

