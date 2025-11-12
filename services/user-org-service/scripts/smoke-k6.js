// k6 smoke test script for User-Org-Service
//
// Purpose:
//   Comprehensive smoke tests for deployment validation covering critical
//   endpoints: health checks, org management, user invites, and API key validation.
//
// Usage:
//   k6 run scripts/smoke-k6.js
//
// Environment Variables:
//   K6_VUS: Number of virtual users (default: 1)
//   K6_DURATION: Test duration (default: 30s)
//   API_URL: Service URL (default: http://localhost:8081)
//
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const responseTime = new Trend('response_time');

// Configuration
export const options = {
  stages: [
    { duration: '5s', target: 1 },   // Ramp up
    { duration: '20s', target: 1 }, // Stay at 1 VU
    { duration: '5s', target: 0 },   // Ramp down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<2000'], // 95% of requests should be below 2s
    'http_req_failed': ['rate<0.01'],     // Error rate should be less than 1%
    'errors': ['rate<0.01'],
  },
};

const API_URL = __ENV.API_URL || 'http://localhost:8081';
const BASE_URL = `${API_URL}`;

// Test data
let testOrgID = null;
let testUserID = null;
let testServiceAccountID = null;
let testAPIKeySecret = null;

export function setup() {
  // Setup: Create test org and user for authenticated tests
  console.log(`Running smoke tests against: ${API_URL}`);
  
  // Create test org
  const orgPayload = JSON.stringify({
    name: `smoke-test-org-${Date.now()}`,
    slug: `smoke-test-${Date.now()}`,
  });
  
  const orgRes = http.post(`${BASE_URL}/v1/orgs`, orgPayload, {
    headers: { 'Content-Type': 'application/json' },
  });
  
  if (orgRes.status === 201) {
    const orgData = JSON.parse(orgRes.body);
    testOrgID = orgData.orgId;
    console.log(`Created test org: ${testOrgID}`);
  } else {
    console.error(`Failed to create test org: ${orgRes.status} - ${orgRes.body}`);
  }
  
  return {
    orgID: testOrgID,
  };
}

export default function (data) {
  // Test 1: Health check
  const healthRes = http.get(`${BASE_URL}/healthz`);
  check(healthRes, {
    'health check status is 200': (r) => r.status === 200,
    'health check response time < 500ms': (r) => r.timings.duration < 500,
  }) || errorRate.add(1);
  responseTime.add(healthRes.timings.duration);
  
  sleep(0.5);
  
  // Test 2: Readiness check
  const readyRes = http.get(`${BASE_URL}/readyz`);
  check(readyRes, {
    'readiness check status is 200': (r) => r.status === 200,
  }) || errorRate.add(1);
  responseTime.add(readyRes.timings.duration);
  
  sleep(0.5);
  
  // Test 3: Create organization (if we have test data)
  if (data.orgID) {
    const orgPayload = JSON.stringify({
      name: `smoke-org-${Date.now()}-${Math.random()}`,
      slug: `smoke-${Date.now()}-${Math.random()}`,
    });
    
    const createOrgRes = http.post(`${BASE_URL}/v1/orgs`, orgPayload, {
      headers: { 'Content-Type': 'application/json' },
    });
    
    check(createOrgRes, {
      'create org status is 201': (r) => r.status === 201,
      'create org has orgId': (r) => {
        if (r.status === 201) {
          const body = JSON.parse(r.body);
          return body.orgId !== undefined;
        }
        return false;
      },
    }) || errorRate.add(1);
    responseTime.add(createOrgRes.timings.duration);
    
    sleep(0.5);
  }
  
  // Test 4: Get organization (if we have test data)
  if (data.orgID) {
    const getOrgRes = http.get(`${BASE_URL}/v1/orgs/${data.orgID}`);
    check(getOrgRes, {
      'get org status is 200': (r) => r.status === 200,
      'get org returns correct orgId': (r) => {
        if (r.status === 200) {
          const body = JSON.parse(r.body);
          return body.orgId === data.orgID;
        }
        return false;
      },
    }) || errorRate.add(1);
    responseTime.add(getOrgRes.timings.duration);
    
    sleep(0.5);
  }
  
  // Test 5: API key validation endpoint (public, no auth required)
  const validateKeyPayload = JSON.stringify({
    apiKeySecret: 'test-invalid-key',
  });
  
  const validateRes = http.post(`${BASE_URL}/v1/auth/validate-api-key`, validateKeyPayload, {
    headers: { 'Content-Type': 'application/json' },
  });
  
  check(validateRes, {
    'validate API key status is 200': (r) => r.status === 200,
    'validate API key returns valid=false for invalid key': (r) => {
      if (r.status === 200) {
        const body = JSON.parse(r.body);
        return body.valid === false;
      }
      return false;
    },
  }) || errorRate.add(1);
  responseTime.add(validateRes.timings.duration);
  
  sleep(0.5);
}

export function teardown(data) {
  // Cleanup: Optionally delete test data
  // Note: In production, test data should be cleaned up
  console.log(`Test completed. Test org ID: ${data.orgID}`);
}

