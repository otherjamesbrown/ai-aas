// Model Management Feature
// CLI command coverage: model registry, model cache, model deploy,
// model troubleshoot, model version, model library, deployment, inference

export { modelsApi } from './api/models';
export type {
  RegistryModel,
  CacheEntry,
  Deployment,
  DeploymentStatus,
  ModelVersion,
  LibraryEntry,
  InferenceModel,
} from './api/models';

// Page components are lazy loaded via routes
