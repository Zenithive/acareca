import http from 'k6/http';
import { check, sleep } from 'k6';

// Smoke test configuration - minimal load to verify basic functionality
export const options = {
  vus: 1, // 1 virtual user
  duration: '1m', // Run for 1 minute
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests should be below 500ms
    http_req_failed: ['rate<0.01'],   // Error rate should be less than 1%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

export function setup() {
  console.log('Running smoke test...');
  console.log(`Base URL: ${BASE_URL}`);
}

export default function () {
  // Test 1: Health check
  let res = http.get(`${BASE_URL}/health`);
  check(res, {
    'health check is 200': (r) => r.status === 200,
    'health check has correct body': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status === 'ok';
      } catch {
        return false;
      }
    },
  });
  sleep(1);
  
  // Test 2: Swagger documentation
  res = http.get(`${BASE_URL}/swagger/index.html`);
  check(res, {
    'swagger docs accessible': (r) => r.status === 200 || r.status === 404,
  });
  sleep(1);
  
  // Test 3: Google OAuth URL
  res = http.get(`${API_BASE}/auth/google`);
  check(res, {
    'google auth endpoint responds': (r) => r.status === 200 || r.status === 302,
  });
  sleep(1);
  
  // Test 4: Login endpoint (expect failure with invalid credentials)
  const loginPayload = JSON.stringify({
    email: 'smoke-test@example.com',
    password: 'SmokeTest@123',
  });
  
  res = http.post(`${API_BASE}/auth/login`, loginPayload, {
    headers: { 'Content-Type': 'application/json' },
  });
  
  check(res, {
    'login endpoint responds': (r) => r.status === 200 || r.status === 401 || r.status === 400,
    'login response has body': (r) => r.body.length > 0,
  });
  sleep(1);
  
  // Test 5: Register endpoint
  const registerPayload = JSON.stringify({
    email: `smoke-${Date.now()}@example.com`,
    password: 'SmokeTest@123',
    name: 'Smoke Test User',
  });
  
  res = http.post(`${API_BASE}/auth/register`, registerPayload, {
    headers: { 'Content-Type': 'application/json' },
  });
  
  check(res, {
    'register endpoint responds': (r) => r.status !== 0,
    'register response has body': (r) => r.body.length > 0,
  });
  sleep(2);
}

export function teardown(data) {
  console.log('Smoke test completed!');
}
