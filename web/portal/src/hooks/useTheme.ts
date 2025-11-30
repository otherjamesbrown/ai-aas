import { useState, useEffect, useCallback, useRef } from 'react';

type ThemeMode = 'system' | 'light' | 'dark';
type ResolvedTheme = 'light' | 'dark';

const THEME_STORAGE_KEY = 'theme-preference';
const DEBOUNCE_DELAY = 300;

/**
 * Get the system's preferred color scheme
 */
function getSystemPreference(): ResolvedTheme {
  if (typeof window === 'undefined') return 'light';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

/**
 * Get stored theme preference from localStorage
 */
function getStoredTheme(): ThemeMode {
  if (typeof window === 'undefined') return 'system';
  try {
    const stored = localStorage.getItem(THEME_STORAGE_KEY);
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      return stored;
    }
  } catch {
    // localStorage not available
  }
  return 'system';
}

/**
 * Store theme preference in localStorage
 */
function storeTheme(mode: ThemeMode): void {
  if (typeof window === 'undefined') return;
  try {
    localStorage.setItem(THEME_STORAGE_KEY, mode);
  } catch {
    // localStorage not available
  }
}

/**
 * Resolve theme mode to actual light/dark value
 */
function resolveTheme(mode: ThemeMode, systemPreference: ResolvedTheme): ResolvedTheme {
  if (mode === 'system') {
    return systemPreference;
  }
  return mode;
}

/**
 * Apply theme to document
 * Note: Using Tailwind's 'media' darkMode strategy means the browser handles
 * theme automatically via prefers-color-scheme. This function is provided
 * for cases where manual class-based toggling is needed.
 */
function applyTheme(resolvedTheme: ResolvedTheme): void {
  if (typeof document === 'undefined') return;

  // For class-based dark mode (darkMode: 'class' in tailwind config)
  // Currently using 'media' strategy so this is optional
  const root = document.documentElement;
  root.classList.remove('light', 'dark');
  root.classList.add(resolvedTheme);

  // Update meta theme-color for mobile browsers
  const metaThemeColor = document.querySelector('meta[name="theme-color"]');
  if (metaThemeColor) {
    metaThemeColor.setAttribute(
      'content',
      resolvedTheme === 'dark' ? '#1f2937' : '#ffffff'
    );
  }
}

/**
 * useTheme - React hook for theme management
 *
 * Provides system-aware theming with manual override support.
 * Implements debounce logic (300ms) to prevent rapid theme switching.
 *
 * Usage:
 * ```typescript
 * function ThemeToggle() {
 *   const { mode, resolvedTheme, setTheme, toggleTheme } = useTheme();
 *
 *   return (
 *     <button onClick={toggleTheme}>
 *       Current: {resolvedTheme} ({mode})
 *     </button>
 *   );
 * }
 * ```
 */
export function useTheme() {
  const [mode, setMode] = useState<ThemeMode>(getStoredTheme);
  const [systemPreference, setSystemPreference] = useState<ResolvedTheme>(getSystemPreference);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Resolved theme value
  const resolvedTheme = resolveTheme(mode, systemPreference);

  // Listen for system preference changes
  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    const handleChange = (e: MediaQueryListEvent) => {
      setSystemPreference(e.matches ? 'dark' : 'light');
    };

    // Use addEventListener for modern browsers
    mediaQuery.addEventListener('change', handleChange);

    return () => {
      mediaQuery.removeEventListener('change', handleChange);
    };
  }, []);

  // Apply theme when resolved theme changes
  useEffect(() => {
    applyTheme(resolvedTheme);
  }, [resolvedTheme]);

  /**
   * Set theme mode with debounce
   */
  const setTheme = useCallback((newMode: ThemeMode) => {
    // Cancel pending debounce
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }

    debounceRef.current = setTimeout(() => {
      setMode(newMode);
      storeTheme(newMode);
      debounceRef.current = null;
    }, DEBOUNCE_DELAY);
  }, []);

  /**
   * Toggle between light and dark modes
   * If currently on system, switches to opposite of current resolved theme
   */
  const toggleTheme = useCallback(() => {
    const newMode: ThemeMode = resolvedTheme === 'dark' ? 'light' : 'dark';
    setTheme(newMode);
  }, [resolvedTheme, setTheme]);

  /**
   * Reset to system preference
   */
  const resetToSystem = useCallback(() => {
    setTheme('system');
  }, [setTheme]);

  // Cleanup debounce on unmount
  useEffect(() => {
    return () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
      }
    };
  }, []);

  return {
    /** Current theme mode setting ('system' | 'light' | 'dark') */
    mode,
    /** Actual resolved theme ('light' | 'dark') */
    resolvedTheme,
    /** System's preferred color scheme */
    systemPreference,
    /** Set theme mode */
    setTheme,
    /** Toggle between light and dark */
    toggleTheme,
    /** Reset to system preference */
    resetToSystem,
    /** Whether theme is set to follow system */
    isSystem: mode === 'system',
  };
}

export type { ThemeMode, ResolvedTheme };
