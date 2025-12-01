/**
 * Test Fixtures
 *
 * Provides test data and fixtures for E2E tests.
 * These fixtures represent realistic test data that can be used
 * to set up test scenarios.
 */

// Re-export test users from auth helpers
export { TEST_USERS } from '../helpers/auth';

/**
 * Test Organizations
 */
export const TEST_ORGS = {
  acme: {
    org_id: 'org-acme-001',
    name: 'ACME Corporation',
    slug: 'acme',
  },
  demo: {
    org_id: 'org-demo-001',
    name: 'Demo Organization',
    slug: 'demo',
  },
} as const;

/**
 * Test API Keys
 */
export const TEST_API_KEYS = {
  inference: {
    display_name: 'inference-key',
    scopes: ['inference:read', 'inference:write'],
    description: 'API key for inference operations',
  },
  readonly: {
    display_name: 'readonly-key',
    scopes: ['usage:read', 'keys:read'],
    description: 'Read-only API key',
  },
  admin: {
    display_name: 'admin-key',
    scopes: ['*'],
    description: 'Full access API key',
  },
} as const;

/**
 * Test Models
 */
export const TEST_MODELS = {
  gpt4: {
    model_id: 'gpt-4o',
    name: 'GPT-4o',
    provider: 'openai',
    version: '2024-01',
  },
  llama: {
    model_id: 'llama-3-70b',
    name: 'Llama 3 70B',
    provider: 'meta',
    version: '3.0',
  },
  claude: {
    model_id: 'claude-3-opus',
    name: 'Claude 3 Opus',
    provider: 'anthropic',
    version: '20240229',
  },
} as const;

/**
 * Test Inference Request
 */
export interface InferenceRequest {
  request_id: string;
  model: string;
  payload: string;
}

export function createInferenceRequest(
  model: string = 'gpt-4o',
  prompt: string = 'Hello, this is a test prompt'
): InferenceRequest {
  return {
    request_id: crypto.randomUUID(),
    model,
    payload: prompt,
  };
}

/**
 * Generate unique test name with timestamp
 */
export function uniqueName(prefix: string): string {
  return `${prefix}-${Date.now()}`;
}

/**
 * Test timeouts and delays
 */
export const TIMEOUTS = {
  /** Page load timeout */
  pageLoad: 10000,
  /** API request timeout */
  apiRequest: 15000,
  /** Modal animation delay */
  modalAnimation: 300,
  /** Network idle wait */
  networkIdle: 5000,
} as const;
