import { createRoute } from '@tanstack/react-router';
import { lazy } from 'react';
import { rootRoute } from '../AppRouter';
import { ProtectedRoute } from './ProtectedRoute';

// Lazy load support page
const SupportConsolePage = lazy(() => import('@/features/support/pages/SupportConsolePage'));

// Support console route
// Note: In production, this should check for support role/permission
export const supportConsoleRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/support',
  component: () => (
    <ProtectedRoute>
      <SupportConsolePage />
    </ProtectedRoute>
  ),
});

