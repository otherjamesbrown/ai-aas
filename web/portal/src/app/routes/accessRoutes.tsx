import { createRoute } from '@tanstack/react-router';
import { lazy } from 'react';
import { rootRoute } from '../AppRouter';

// Lazy load access control pages
const OrganizationsPage = lazy(() => import('../features/access-control/pages/OrganizationsPage'));
const UsersPage = lazy(() => import('../features/access-control/pages/UsersPage'));
const UserModelAccessPage = lazy(() => import('../features/access-control/pages/UserModelAccessPage'));
const CredentialsPage = lazy(() => import('../features/access-control/pages/CredentialsPage'));

// Organizations route
export const organizationsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/access/organizations',
  component: OrganizationsPage,
});

// Users route
export const usersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/access/users',
  component: UsersPage,
});

// User Model Access route
export const userModelAccessRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/access/user-model-access',
  component: UserModelAccessPage,
});

// Credentials route
export const credentialsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/access/credentials',
  component: CredentialsPage,
});
