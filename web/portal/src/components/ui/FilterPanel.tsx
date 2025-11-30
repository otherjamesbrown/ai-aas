import { ReactNode } from 'react';
import { Button } from './Button';

interface FilterOption {
  value: string;
  label: string;
}

interface FilterPanelProps {
  children: ReactNode;
  onReset?: () => void;
  className?: string;
}

/**
 * FilterPanel - Container for filter controls
 */
export function FilterPanel({ children, onReset, className = '' }: FilterPanelProps) {
  return (
    <div
      className={`flex flex-wrap items-center gap-4 p-4 bg-gray-50 dark:bg-gray-800/50 rounded-lg border border-gray-200 dark:border-gray-700 ${className}`}
    >
      {children}
      {onReset && (
        <Button variant="ghost" size="sm" onClick={onReset}>
          Reset Filters
        </Button>
      )}
    </div>
  );
}

interface FilterSelectProps {
  label: string;
  value: string;
  options: FilterOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}

/**
 * FilterSelect - Dropdown filter control
 */
export function FilterSelect({
  label,
  value,
  options,
  onChange,
  placeholder = 'All',
  className = '',
}: FilterSelectProps) {
  return (
    <div className={`flex flex-col ${className}`}>
      <label className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
        {label}
      </label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
      >
        <option value="">{placeholder}</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

interface FilterDateRangeProps {
  label: string;
  startDate: string;
  endDate: string;
  onStartDateChange: (date: string) => void;
  onEndDateChange: (date: string) => void;
  className?: string;
}

/**
 * FilterDateRange - Date range filter control
 */
export function FilterDateRange({
  label,
  startDate,
  endDate,
  onStartDateChange,
  onEndDateChange,
  className = '',
}: FilterDateRangeProps) {
  return (
    <div className={`flex flex-col ${className}`}>
      <label className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
        {label}
      </label>
      <div className="flex items-center gap-2">
        <input
          type="date"
          value={startDate}
          onChange={(e) => onStartDateChange(e.target.value)}
          className="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
        />
        <span className="text-gray-400">to</span>
        <input
          type="date"
          value={endDate}
          onChange={(e) => onEndDateChange(e.target.value)}
          className="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
        />
      </div>
    </div>
  );
}

interface FilterSearchProps {
  label?: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}

/**
 * FilterSearch - Search input filter control
 */
export function FilterSearch({
  label,
  value,
  onChange,
  placeholder = 'Search...',
  className = '',
}: FilterSearchProps) {
  return (
    <div className={`flex flex-col ${className}`}>
      {label && (
        <label className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
          {label}
        </label>
      )}
      <div className="relative">
        <svg
          className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className="pl-9 pr-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
        />
      </div>
    </div>
  );
}

interface FilterChipsProps {
  label: string;
  options: FilterOption[];
  selected: string[];
  onChange: (selected: string[]) => void;
  className?: string;
}

/**
 * FilterChips - Multi-select chip filter control
 */
export function FilterChips({
  label,
  options,
  selected,
  onChange,
  className = '',
}: FilterChipsProps) {
  const toggleOption = (value: string) => {
    if (selected.includes(value)) {
      onChange(selected.filter((v) => v !== value));
    } else {
      onChange([...selected, value]);
    }
  };

  return (
    <div className={`flex flex-col ${className}`}>
      <label className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
        {label}
      </label>
      <div className="flex flex-wrap gap-2">
        {options.map((option) => {
          const isSelected = selected.includes(option.value);
          return (
            <button
              key={option.value}
              onClick={() => toggleOption(option.value)}
              className={`px-3 py-1 text-sm rounded-full transition-colors ${
                isSelected
                  ? 'bg-primary text-white'
                  : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
              }`}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

interface QuickFilterProps {
  options: { value: string; label: string }[];
  selected: string;
  onChange: (value: string) => void;
  className?: string;
}

/**
 * QuickFilter - Single-select quick filter buttons
 */
export function QuickFilter({ options, selected, onChange, className = '' }: QuickFilterProps) {
  return (
    <div className={`flex items-center gap-1 ${className}`}>
      {options.map((option) => (
        <button
          key={option.value}
          onClick={() => onChange(option.value)}
          className={`px-3 py-1.5 text-sm font-medium rounded-md transition-colors ${
            selected === option.value
              ? 'bg-primary text-white'
              : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
          }`}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
