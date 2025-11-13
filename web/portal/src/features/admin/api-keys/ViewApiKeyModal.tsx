import { useState } from 'react';

interface ViewApiKeyModalProps {
  isOpen: boolean;
  onClose: () => void;
  secret: string;
}

export function ViewApiKeyModal({ isOpen, onClose, secret }: ViewApiKeyModalProps) {
  const [copied, setCopied] = useState(false);

  if (!isOpen) return null;

  const handleCopy = async () => {
    await navigator.clipboard.writeText(secret);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto" aria-modal="true">
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="fixed inset-0 bg-gray-500 bg-opacity-75" onClick={onClose} />
        <div className="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
          <h2 className="text-xl font-bold mb-4">API Key Created</h2>
          <div className="mb-4">
            <p className="text-sm text-gray-600 mb-2">
              <strong>Important:</strong> This is the only time you'll see this secret. Copy it now
              and store it securely.
            </p>
            <div className="bg-gray-50 border border-gray-300 rounded-md p-3 font-mono text-sm break-all">
              {secret}
            </div>
          </div>
          <div className="flex justify-end space-x-3">
            <button
              onClick={handleCopy}
              className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
            >
              {copied ? 'Copied!' : 'Copy to Clipboard'}
            </button>
            <button
              onClick={onClose}
              className="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark"
            >
              I've Saved It
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

