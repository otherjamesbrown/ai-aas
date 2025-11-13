import { useEffect } from 'react';

interface AccessibilityAnnouncerProps {
  message: string;
  priority?: 'polite' | 'assertive';
}

/**
 * Component for announcing dynamic content changes to screen readers
 * Uses ARIA live regions for accessibility
 */
export function AccessibilityAnnouncer({ message, priority = 'polite' }: AccessibilityAnnouncerProps) {
  useEffect(() => {
    if (!message) return;

    const announcer = document.createElement('div');
    announcer.setAttribute('role', 'status');
    announcer.setAttribute('aria-live', priority);
    announcer.setAttribute('aria-atomic', 'true');
    announcer.className = 'sr-only';
    announcer.textContent = message;

    document.body.appendChild(announcer);

    // Remove after announcement
    const timeout = setTimeout(() => {
      document.body.removeChild(announcer);
    }, 1000);

    return () => {
      clearTimeout(timeout);
      if (document.body.contains(announcer)) {
        document.body.removeChild(announcer);
      }
    };
  }, [message, priority]);

  return null;
}

/**
 * Hook to announce messages to screen readers
 */
export function useA11yAnnounce() {
  return (message: string, priority: 'polite' | 'assertive' = 'polite') => {
    const announcer = document.createElement('div');
    announcer.setAttribute('role', 'status');
    announcer.setAttribute('aria-live', priority);
    announcer.setAttribute('aria-atomic', 'true');
    announcer.className = 'sr-only';
    announcer.textContent = message;
    document.body.appendChild(announcer);
    setTimeout(() => {
      if (document.body.contains(announcer)) {
        document.body.removeChild(announcer);
      }
    }, 1000);
  };
}

