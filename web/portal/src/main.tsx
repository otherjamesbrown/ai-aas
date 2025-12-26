import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from '@tanstack/react-router';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { AppProviders } from '@/providers/AppProviders';
import { router } from './app/AppRouter';
import { initSentry } from '@/lib/sentry';
import './styles/global.css';

/**
 * Initialize Sentry before React renders
 * This ensures all errors are captured, including initialization errors
 */
initSentry();

/**
 * Main application entry point
 *
 * Provider hierarchy (max 4 levels per spec requirement):
 * 1. StrictMode
 * 2. ErrorBoundary
 * 3. AppProviders (combines Auth, Query, Toast)
 * 4. RouterProvider
 *
 * Note: TelemetryProvider and FeatureFlagProviderWrapper were evaluated
 * and removed as they are not critical for the admin portal MVP.
 * They can be re-added later if needed.
 */
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    </ErrorBoundary>
  </React.StrictMode>
);

