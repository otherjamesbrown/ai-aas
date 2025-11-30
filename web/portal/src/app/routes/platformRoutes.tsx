import { createRoute } from '@tanstack/react-router';
import { lazy } from 'react';
import { rootRoute } from '../AppRouter';

// Lazy load platform operations pages
const PlatformStatusPage = lazy(() => import('../features/platform-operations/pages/PlatformStatusPage'));
const GitOpsPage = lazy(() => import('../features/platform-operations/pages/GitOpsPage'));

// Platform status route
export const platformStatusRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/platform/status',
  component: PlatformStatusPage,
});

// GitOps route
export const gitOpsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/platform/gitops',
  component: GitOpsPage,
});
