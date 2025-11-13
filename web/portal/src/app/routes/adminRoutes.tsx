import { createRoute } from '@tanstack/react-router';
import { lazy } from 'react';
import { rootRoute } from '../AppRouter';

// Lazy load admin pages
const OrganizationSettingsPage = lazy(() => import('@/features/admin/org/OrganizationSettingsPage'));
const MemberManagementPage = lazy(() => import('@/features/admin/members/MemberManagementPage'));
const BudgetControlsPage = lazy(() => import('@/features/admin/budgets/BudgetControlsPage'));
const ApiKeysPage = lazy(() => import('@/features/admin/api-keys/ApiKeysPage'));

// Admin index route - redirects to organization settings
export const adminIndexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin',
  component: () => {
    // Redirect to organization settings
    window.location.href = '/admin/organization';
    return null;
  },
});

// Organization settings route
export const organizationSettingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin/organization',
  component: OrganizationSettingsPage,
});

// Member management route
export const memberManagementRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin/members',
  component: MemberManagementPage,
});

// Budget controls route
export const budgetControlsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin/budgets',
  component: BudgetControlsPage,
});

// API keys route
export const apiKeysRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin/api-keys',
  component: ApiKeysPage,
});

