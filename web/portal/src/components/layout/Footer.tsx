/**
 * Footer - Page footer component
 *
 * Displays version, API reference link, and feedback link.
 * Based on Linode Cloud Manager design patterns.
 */
export function Footer() {
  const year = new Date().getFullYear();
  const version = import.meta.env.VITE_APP_VERSION || '1.0.0';

  return (
    <footer className="h-12 bg-white dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between px-6 text-sm">
      <div className="flex items-center space-x-4">
        <span className="text-gray-500 dark:text-gray-400">
          AI-AAS Platform v{version}
        </span>
        <span className="text-gray-300 dark:text-gray-600">|</span>
        <span className="text-gray-400 dark:text-gray-500">
          © {year} AI-AAS
        </span>
      </div>

      <div className="flex items-center space-x-4">
        <a
          href="/docs/api"
          className="text-gray-500 dark:text-gray-400 hover:text-primary dark:hover:text-primary transition-colors"
        >
          API Reference
        </a>
        <a
          href="https://github.com/otherjamesbrown/ai-aas/issues"
          target="_blank"
          rel="noopener noreferrer"
          className="text-gray-500 dark:text-gray-400 hover:text-primary dark:hover:text-primary transition-colors"
        >
          Feedback
        </a>
        <a
          href="/docs"
          className="text-gray-500 dark:text-gray-400 hover:text-primary dark:hover:text-primary transition-colors"
        >
          Documentation
        </a>
      </div>
    </footer>
  );
}
