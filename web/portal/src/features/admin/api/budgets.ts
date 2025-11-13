import { httpClient } from '@/lib/http/client';
import type {
  BudgetPolicy,
  UpdateBudgetRequest,
} from '../types';

/**
 * Budget policy API client
 */
export const budgetsApi = {
  /**
   * Get current organization budget policy
   */
  async getPolicy(): Promise<BudgetPolicy> {
    const response = await httpClient.get<BudgetPolicy>(
      '/organizations/me/budget'
    );
    return response.data;
  },

  /**
   * Update budget policy
   */
  async updatePolicy(updates: UpdateBudgetRequest): Promise<BudgetPolicy> {
    const response = await httpClient.patch<BudgetPolicy>(
      '/organizations/me/budget',
      updates
    );
    return response.data;
  },

  /**
   * Get budget history/audit log
   */
  async getHistory(): Promise<Array<BudgetPolicy & { changed_at: string; changed_by: string }>> {
    const response = await httpClient.get<Array<BudgetPolicy & { changed_at: string; changed_by: string }>>(
      '/organizations/me/budget/history'
    );
    return response.data;
  },
};

