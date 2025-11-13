import { createRootRoute, createRoute, createRouter, Outlet } from '@tanstack/react-router';
import { lazy, Suspense } from 'react';
import Layout from './Layout';
import { LoadingSpinner } from '@/components/LoadingSpinner';
import {
  adminIndexRoute,
  organizationSettingsRoute,
  memberManagementRoute,
  budgetControlsRoute,
  apiKeysRoute,
} from './routes/adminRoutes';
import { usageDashboardRoute } from './routes/usageRoutes';
import { supportConsoleRoute } from './routes/supportRoutes';

// Lazy load route components
const HomePage = lazy(() => import('./pages/HomePage'));
const LoginPage = lazy(() => import('./pages/LoginPage'));
const AccessDeniedPage = lazy(() => import('./pages/AccessDeniedPage'));

// Root route
export const rootRoute = createRootRoute({
  component: () => (
    <Layout>
      <Suspense fallback={<LoadingSpinner size="lg" className="min-h-screen" aria-label="Loading application" />}>
        <Outlet />
      </Suspense>
    </Layout>
  ),
});

// Index route
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomePage,
});

// Auth routes
const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/auth/login',
  component: LoginPage,
  validateSearch: (search: Record<string, unknown>): { redirect?: string } => {
    return {
      redirect: search.redirect ? (search.redirect as string) : undefined,
    };
  },
});

// Usage route is now imported from usageRoutes.tsx

// Access denied route
const accessDeniedRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/access-denied',
  component: AccessDeniedPage,
});

// 404 route
const notFoundRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '*',
  component: () => (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <h1 className="text-4xl font-bold text-gray-900 mb-4">404</h1>
        <p className="text-gray-600 mb-4">Page not found</p>
        <a href="/" className="text-primary hover:underline">
          Return to home
        </a>
      </div>
    </div>
  ),
});

// Route tree
const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  adminIndexRoute,
  organizationSettingsRoute,
  memberManagementRoute,
  budgetControlsRoute,
  apiKeysRoute,
  usageDashboardRoute,
  supportConsoleRoute,
  accessDeniedRoute,
  notFoundRoute,
]);

// Create router
export const router = createRouter({ routeTree });

// Declare router types
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

export default router;

