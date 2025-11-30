import { createRoute } from '@tanstack/react-router';
import { lazy } from 'react';
import { rootRoute } from '../AppRouter';

// Lazy load platform operations pages
const PlatformStatusPage = lazy(() => import('../features/platform-operations/pages/PlatformStatusPage'));
const GitOpsPage = lazy(() => import('../features/platform-operations/pages/GitOpsPage'));
const ConfigPage = lazy(() => import('../features/platform-operations/pages/ConfigPage'));
const BootstrapPage = lazy(() => import('../features/platform-operations/pages/BootstrapPage'));
const RegistryPage = lazy(() => import('../features/platform-operations/pages/RegistryPage'));
const RoutingPolicyPage = lazy(() => import('../features/platform-operations/pages/RoutingPolicyPage'));
const SyncPage = lazy(() => import('../features/platform-operations/pages/SyncPage'));
const ExportPage = lazy(() => import('../features/platform-operations/pages/ExportPage'));

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

// Config route
export const configRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/platform/config',
  component: ConfigPage,
});

// Bootstrap route
export const bootstrapRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/platform/bootstrap',
  component: BootstrapPage,
});

// Registry route
export const registryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/platform/registry',
  component: RegistryPage,
});

// Routing policy route
export const routingPolicyRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/platform/routing',
  component: RoutingPolicyPage,
});

// Sync route
export const syncRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/platform/sync',
  component: SyncPage,
});

// Export route
export const exportRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/platform/export',
  component: ExportPage,
});
