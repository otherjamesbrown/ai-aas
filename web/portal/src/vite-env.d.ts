/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_OAUTH_CLIENT_ID?: string;
  readonly VITE_OAUTH_ISSUER_URL?: string;
  readonly VITE_OAUTH_REDIRECT_URI?: string;
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_FEATURE_FLAGS_API_URL?: string;
  readonly VITE_OTEL_SERVICE_NAME?: string;
  readonly VITE_OTEL_SERVICE_VERSION?: string;
  readonly VITE_OTEL_EXPORTER_OTLP_ENDPOINT?: string;
  readonly MODE: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

