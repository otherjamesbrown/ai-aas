import { usageApi } from '../api';
import type { UsageFilters } from '../types';

/**
 * Export usage data as CSV
 * Handles large datasets by streaming/chunking if needed
 */
export async function exportUsageCsv(filters: UsageFilters = {}): Promise<void> {
  try {
    // Fetch the blob from the API
    const blob = await usageApi.exportCsv(filters);

    // Create a download link
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;

    // Generate filename with timestamp
    const timestamp = new Date().toISOString().split('T')[0];
    const filename = `usage-export-${timestamp}.csv`;
    link.download = filename;

    // Trigger download
    document.body.appendChild(link);
    link.click();

    // Cleanup
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  } catch (error) {
    console.error('CSV export failed:', error);
    throw error;
  }
}

/**
 * Alternative: Client-side CSV generation for smaller datasets
 * Use this if the API doesn't support CSV export
 */
export function generateCsvFromData(data: Array<Record<string, unknown>>): string {
  if (data.length === 0) {
    return '';
  }

  // Get headers from first row
  const headers = Object.keys(data[0]);

  // Create CSV rows
  const csvRows = [
    headers.join(','), // Header row
    ...data.map((row) =>
      headers
        .map((header) => {
          const value = row[header];
          // Escape commas and quotes in values
          if (value === null || value === undefined) {
            return '';
          }
          const stringValue = String(value);
          if (stringValue.includes(',') || stringValue.includes('"') || stringValue.includes('\n')) {
            return `"${stringValue.replace(/"/g, '""')}"`;
          }
          return stringValue;
        })
        .join(',')
    ),
  ];

  return csvRows.join('\n');
}

