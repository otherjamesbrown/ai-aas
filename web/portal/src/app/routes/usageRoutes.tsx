import { createRoute } from '@tanstack/react-router';
import { lazy } from 'react';
import { rootRoute } from '../AppRouter';
import { ProtectedRoute } from './ProtectedRoute';

// Lazy load usage page
const UsageDashboard = lazy(() => import('@/features/usage/components/UsageDashboard'));

// Usage dashboard route
export const usageDashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/usage',
  component: () => (
    <ProtectedRoute permissions="usage.read">
      <UsageDashboard />
    </ProtectedRoute>
  ),
});

