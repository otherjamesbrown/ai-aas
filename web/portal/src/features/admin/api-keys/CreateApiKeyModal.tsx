import { useState } from 'react';
import type { CreateApiKeyRequest } from '../types';

interface CreateApiKeyModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreate: (request: CreateApiKeyRequest) => void;
  isLoading: boolean;
}

export function CreateApiKeyModal({
  isOpen,
  onClose,
  onCreate,
  isLoading,
}: CreateApiKeyModalProps) {
  const [displayName, setDisplayName] = useState('');
  const [scopes, setScopes] = useState<string[]>([]);
  const [scopeInput, setScopeInput] = useState('');

  if (!isOpen) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onCreate({
      display_name: displayName,
      scopes: scopes.length > 0 ? scopes : ['read'],
    });
    setDisplayName('');
    setScopes([]);
    setScopeInput('');
  };

  const addScope = () => {
    if (scopeInput.trim() && !scopes.includes(scopeInput.trim())) {
      setScopes([...scopes, scopeInput.trim()]);
      setScopeInput('');
    }
  };

  const removeScope = (scope: string) => {
    setScopes(scopes.filter((s) => s !== scope));
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto" aria-modal="true">
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="fixed inset-0 bg-gray-500 bg-opacity-75" onClick={onClose} />
        <div className="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
          <h2 className="text-xl font-bold mb-4">Create API Key</h2>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="display-name" className="block text-sm font-medium text-gray-700">
                Display Name
              </label>
              <input
                type="text"
                id="display-name"
                required
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
              />
            </div>
            <div>
              <label htmlFor="scopes" className="block text-sm font-medium text-gray-700">
                Scopes
              </label>
              <div className="mt-1 flex space-x-2">
                <input
                  type="text"
                  id="scopes"
                  value={scopeInput}
                  onChange={(e) => setScopeInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      addScope();
                    }
                  }}
                  placeholder="Enter scope and press Enter"
                  className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
                />
                <button
                  type="button"
                  onClick={addScope}
                  className="px-3 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                >
                  Add
                </button>
              </div>
              {scopes.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-2">
                  {scopes.map((scope) => (
                    <span
                      key={scope}
                      className="inline-flex items-center px-2 py-1 rounded-md text-xs font-medium bg-blue-100 text-blue-800"
                    >
                      {scope}
                      <button
                        type="button"
                        onClick={() => removeScope(scope)}
                        className="ml-2 text-blue-600 hover:text-blue-800"
                      >
                        ×
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>
            <div className="flex justify-end space-x-3">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isLoading}
                className="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-dark disabled:opacity-50"
              >
                {isLoading ? 'Creating...' : 'Create Key'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

