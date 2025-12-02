// Platform Operations Feature
// CLI command coverage: status, config, backup, gitops

export { platformApi } from './api/platform';
export type {
  PlatformStatus,
  ConfigEntry,
  Backup,
  ArgoApplication,
} from './api/platform';

// Page components are lazy loaded via routes
