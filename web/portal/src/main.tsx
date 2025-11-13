import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from '@tanstack/react-router';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { TelemetryProvider } from '@/providers/TelemetryProvider';
import { FeatureFlagProviderWrapper } from '@/providers/FeatureFlagProviderWrapper';
import { AuthProvider } from '@/providers/AuthProvider';
import { QueryProvider } from '@/lib/query';
import { ToastProvider } from '@/providers/ToastProvider';
import { router } from './app/AppRouter';
import './styles/global.css';

/**
 * Main application entry point
 * Wraps app with all necessary providers in correct order
 */
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <TelemetryProvider
        serviceName={import.meta.env.VITE_OTEL_SERVICE_NAME || 'web-portal'}
        serviceVersion={import.meta.env.VITE_OTEL_SERVICE_VERSION || '0.1.0'}
        endpoint={import.meta.env.VITE_OTEL_EXPORTER_OTLP_ENDPOINT}
      >
        <AuthProvider>
          <FeatureFlagProviderWrapper
            apiUrl={import.meta.env.VITE_FEATURE_FLAGS_API_URL}
          >
            <QueryProvider>
              <ToastProvider>
                <RouterProvider router={router} />
              </ToastProvider>
            </QueryProvider>
          </FeatureFlagProviderWrapper>
        </AuthProvider>
      </TelemetryProvider>
    </ErrorBoundary>
  </React.StrictMode>
);

