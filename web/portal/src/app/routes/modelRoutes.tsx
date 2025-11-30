import { createRoute } from '@tanstack/react-router';
import { lazy } from 'react';
import { rootRoute } from '../AppRouter';

// Lazy load model management pages
const ModelRegistryPage = lazy(() => import('../features/model-management/pages/ModelRegistryPage'));
const ModelCachePage = lazy(() => import('../features/model-management/pages/ModelCachePage'));
const ModelDeploymentsPage = lazy(() => import('../features/model-management/pages/ModelDeploymentsPage'));
const InferencePage = lazy(() => import('../features/model-management/pages/InferencePage'));

// Model registry route
export const modelRegistryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/models/registry',
  component: ModelRegistryPage,
});

// Model cache route
export const modelCacheRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/models/cache',
  component: ModelCachePage,
});

// Model deployments route
export const modelDeploymentsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/models/deployments',
  component: ModelDeploymentsPage,
});

// Inference playground route
export const inferenceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/models/inference',
  component: InferencePage,
});
