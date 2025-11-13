/**
 * Usage insights domain types
 */

export type OperationType = 'chat' | 'embeddings' | 'fine-tune';
export type ConfidenceLevel = 'estimated' | 'finalized';
export type TimeRange = 'last_24h' | 'last_7d' | 'last_30d' | 'custom';
export type DataSource = 'billing-api' | 'cache' | 'degraded';

export interface UsageSnapshot {
  window_start: string; // ISO-8601 timestamp
  window_end: string; // ISO-8601 timestamp
  model: string; // e.g., 'gpt-4o'
  operation: OperationType;
  requests: number;
  tokens: number;
  cost_cents: number;
  confidence: ConfidenceLevel;
}

export interface UsageTotals {
  requests: number;
  tokens: number;
  cost_cents: number;
}

export interface UsageReport {
  time_range: TimeRange;
  totals: UsageTotals;
  breakdowns: UsageSnapshot[];
  generated_at: string; // ISO-8601 timestamp
  source: DataSource;
}

export interface UsageFilters {
  time_range?: TimeRange;
  start_date?: string; // ISO-8601 timestamp (for custom range)
  end_date?: string; // ISO-8601 timestamp (for custom range)
  model?: string;
  operation?: OperationType;
}

export interface UsageApiResponse {
  report: UsageReport;
  last_sync_at?: string; // ISO-8601 timestamp
  sync_status?: 'success' | 'degraded' | 'error';
}

