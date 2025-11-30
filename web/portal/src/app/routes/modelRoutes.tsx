import { createRoute } from '@tanstack/react-router';
import { lazy } from 'react';
import { rootRoute } from '../AppRouter';

// Lazy load model management pages
const ModelRegistryPage = lazy(() => import('../features/model-management/pages/ModelRegistryPage'));
const ModelCachePage = lazy(() => import('../features/model-management/pages/ModelCachePage'));
const ModelDeploymentsPage = lazy(() => import('../features/model-management/pages/ModelDeploymentsPage'));
const ModelTroubleshootPage = lazy(() => import('../features/model-management/pages/ModelTroubleshootPage'));
const ModelVersionPage = lazy(() => import('../features/model-management/pages/ModelVersionPage'));
const ModelLibraryPage = lazy(() => import('../features/model-management/pages/ModelLibraryPage'));
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

// Model troubleshoot route
export const modelTroubleshootRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/models/troubleshoot',
  component: ModelTroubleshootPage,
});

// Model version route
export const modelVersionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/models/versions',
  component: ModelVersionPage,
});

// Model library route
export const modelLibraryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/models/library',
  component: ModelLibraryPage,
});

// Inference playground route
export const inferenceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/models/inference',
  component: InferencePage,
});
