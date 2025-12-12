import { createContext, useContext, useState, useCallback, ReactNode } from 'react';
import { Toast } from '@/components/Toast';
import { logger } from '@/lib/logger';

interface ToastContextValue {
  showToast: (toast: Omit<Toast, 'id'>) => void;
  dismissToast: (id: string) => void;
  toasts: Toast[];
}

const ToastContext = createContext<ToastContextValue | undefined>(undefined);

interface ToastProviderProps {
  children: ReactNode;
}

/**
 * Toast notification provider
 * Manages global toast notifications with auto-dismiss and action support
 */
export function ToastProvider({ children }: ToastProviderProps) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const showToast = useCallback((toast: Omit<Toast, 'id'>) => {
    logger.debug('Toast notification triggered', { component: 'ToastProvider', type: toast.type, title: toast.title });
    const id = crypto.randomUUID();
    const newToast: Toast = {
      ...toast,
      id,
      duration: toast.duration ?? 5000, // Default 5 seconds
    };

    logger.debug('Adding toast to state', { component: 'ToastProvider', toastId: id });
    setToasts((prev) => {
      const updated = [...prev, newToast];
      logger.debug('Toast state updated', { component: 'ToastProvider', totalToasts: updated.length });
      return updated;
    });
  }, []);

  const dismissToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ showToast, dismissToast, toasts }}>
      {children}
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </ToastContext.Provider>
  );
}

interface ToastContainerProps {
  toasts: Toast[];
  onDismiss: (id: string) => void;
}

function ToastContainer({ toasts, onDismiss }: ToastContainerProps) {
  if (toasts.length > 0) {
    logger.debug('Rendering toast container', { component: 'ToastContainer', toastCount: toasts.length });
  }

  if (toasts.length === 0) {
    return null;
  }

  return (
    <div
      className="fixed top-4 right-4 z-50 flex flex-col space-y-2 pointer-events-none"
      aria-live="polite"
      aria-label="Notifications"
      style={{ zIndex: 9999 }} // Ensure it's on top
    >
      {toasts.map((toast) => {
        return <Toast key={toast.id} toast={toast} onDismiss={onDismiss} />;
      })}
    </div>
  );
}

/**
 * Hook to access toast functionality
 */
export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error('useToast must be used within ToastProvider');
  }

  const showSuccess = useCallback(
    (title: string, message?: string, duration?: number) => {
      context.showToast({ type: 'success', title, message, duration });
    },
    [context]
  );

  const showError = useCallback(
    (title: string, message?: string, duration?: number) => {
      logger.debug('Error toast requested', { component: 'useToast', title, message });
      try {
        context.showToast({ type: 'error', title, message, duration: duration ?? 7000 }); // Errors stay longer
      } catch (error) {
        logger.error('Failed to show error toast', { component: 'useToast', title }, error instanceof Error ? error : undefined);
        throw error;
      }
    },
    [context]
  );

  const showWarning = useCallback(
    (title: string, message?: string, duration?: number) => {
      context.showToast({ type: 'warning', title, message, duration });
    },
    [context]
  );

  const showInfo = useCallback(
    (title: string, message?: string, duration?: number) => {
      context.showToast({ type: 'info', title, message, duration });
    },
    [context]
  );

  return {
    showToast: context.showToast,
    showSuccess,
    showError,
    showWarning,
    showInfo,
    dismissToast: context.dismissToast,
  };
}

