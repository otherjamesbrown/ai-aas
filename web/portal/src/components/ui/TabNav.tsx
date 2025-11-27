import { ReactNode } from 'react';

interface Tab {
  id: string;
  label: string;
  icon?: ReactNode;
  disabled?: boolean;
  badge?: string | number;
}

interface TabNavProps {
  tabs: Tab[];
  activeTab: string;
  onTabChange: (tabId: string) => void;
  variant?: 'underline' | 'pills';
  className?: string;
}

/**
 * TabNav - Horizontal tab navigation
 *
 * Variants:
 * - underline: Linode-style underline indicator (default)
 * - pills: Filled pill-style tabs
 */
export function TabNav({
  tabs,
  activeTab,
  onTabChange,
  variant = 'underline',
  className = '',
}: TabNavProps) {
  if (variant === 'pills') {
    return (
      <div className={`flex space-x-1 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg ${className}`}>
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => !tab.disabled && onTabChange(tab.id)}
            disabled={tab.disabled}
            className={`
              flex items-center px-4 py-2 text-sm font-medium rounded-md transition-colors
              ${
                activeTab === tab.id
                  ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
              }
              ${tab.disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
            `}
          >
            {tab.icon && <span className="mr-2">{tab.icon}</span>}
            {tab.label}
            {tab.badge !== undefined && (
              <span className="ml-2 px-2 py-0.5 text-xs bg-gray-200 dark:bg-gray-600 rounded-full">
                {tab.badge}
              </span>
            )}
          </button>
        ))}
      </div>
    );
  }

  // Underline variant (default, Linode-style)
  return (
    <div className={`border-b border-gray-200 dark:border-gray-700 ${className}`}>
      <nav className="flex space-x-8" aria-label="Tabs">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => !tab.disabled && onTabChange(tab.id)}
            disabled={tab.disabled}
            className={`
              flex items-center py-3 px-1 text-sm font-medium border-b-2 transition-colors
              ${
                activeTab === tab.id
                  ? 'border-primary text-primary'
                  : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
              }
              ${tab.disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
            `}
          >
            {tab.icon && <span className="mr-2">{tab.icon}</span>}
            {tab.label}
            {tab.badge !== undefined && (
              <span className="ml-2 px-2 py-0.5 text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                {tab.badge}
              </span>
            )}
          </button>
        ))}
      </nav>
    </div>
  );
}

/**
 * TabPanel - Container for tab content
 */
interface TabPanelProps {
  children: ReactNode;
  tabId: string;
  activeTab: string;
  className?: string;
}

export function TabPanel({ children, tabId, activeTab, className = '' }: TabPanelProps) {
  if (tabId !== activeTab) return null;

  return (
    <div className={`py-4 ${className}`} role="tabpanel">
      {children}
    </div>
  );
}
