import { ReactElement } from 'react';
import { render, RenderOptions } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TelemetryProvider } from '@/providers/TelemetryProvider';
import { AuthProvider } from '@/providers/AuthProvider';
import { FeatureFlagProviderWrapper } from '@/providers/FeatureFlagProviderWrapper';
import { ToastProvider } from '@/providers/ToastProvider';

// Create a test query client with default options
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

interface AllTheProvidersProps {
  children: React.ReactNode;
  queryClient?: QueryClient;
}

function AllTheProviders({
  children,
  queryClient = createTestQueryClient(),
}: AllTheProvidersProps) {
  return (
    <QueryClientProvider client={queryClient}>
      <TelemetryProvider
        serviceName="test"
        serviceVersion="0.1.0"
        enabled={false}
      >
        <AuthProvider>
          <FeatureFlagProviderWrapper>
            <ToastProvider>
              {children}
            </ToastProvider>
          </FeatureFlagProviderWrapper>
        </AuthProvider>
      </TelemetryProvider>
    </QueryClientProvider>
  );
}

interface CustomRenderOptions extends Omit<RenderOptions, 'wrapper'> {
  queryClient?: QueryClient;
}

function customRender(ui: ReactElement, options: CustomRenderOptions = {}) {
  const { queryClient, ...renderOptions } = options;

  return render(ui, {
    wrapper: (props) => (
      <AllTheProviders
        queryClient={queryClient}
        {...props}
      />
    ),
    ...renderOptions,
  });
}

// Re-export everything
export * from '@testing-library/react';
export { customRender as render };
export { createTestQueryClient };

