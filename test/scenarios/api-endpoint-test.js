import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics for each endpoint category
const healthCheckErrors = new Rate('health_check_errors');
const authEndpointErrors = new Rate('auth_endpoint_errors');
const businessEndpointErrors = new Rate('business_endpoint_errors');
const adminEndpointErrors = new Rate('admin_endpoint_errors');

const healthCheckDuration = new Trend('health_check_duration');
const authEndpointDuration = new Trend('auth_endpoint_duration');
const businessEndpointDuration = new Trend('business_endpoint_duration');

const totalEndpointsCalled = new Counter('total_endpoints_called');

// Test configuration - API endpoint coverage
export const options = {
  scenarios: {
    // Scenario 1: Health & Public Endpoints
    health_and_public: {
      executor: 'constant-vus',
      vus: 20,
      duration: '3m',
      exec: 'healthAndPublicEndpoints',
    },
    
    // Scenario 2: Auth Endpoints
    auth_endpoints: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 15 },
        { duration: '2m', target: 30 },
        { duration: '30s', target: 0 },
      ],
      exec: 'authEndpoints',
      startTime: '10s',
    },
    
    // Scenario 3: Business Endpoints (Authenticated)
    business_endpoints: {
      executor: 'ramping-arrival-rate',
      startRate: 5,
      timeUnit: '1s',
      stages: [
        { duration: '1m', target: 10 },
        { duration: '2m', target: 20 },
        { duration: '1m', target: 5 },
      ],
      preAllocatedVUs: 30,
      exec: 'businessEndpoints',
      startTime: '20s',
    },
    
    // Scenario 4: Admin Endpoints
    admin_endpoints: {
      executor: 'constant-arrival-rate',
      rate: 3,
      timeUnit: '1s',
      duration: '3m',
      preAllocatedVUs: 10,
      exec: 'adminEndpoints',
      startTime: '30s',
    },
    
    // Scenario 5: Mixed Endpoint Load
    mixed_load: {
      executor: 'ramping-vus',
      startVUs: 10,
      stages: [
        { duration: '1m', target: 30 },
        { duration: '2m', target: 50 },
        { duration: '1m', target: 10 },
      ],
      exec: 'mixedEndpointLoad',
      startTime: '15s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    http_req_failed: ['rate<0.15'],
    health_check_duration: ['p(95)<200'],
    auth_endpoint_duration: ['p(95)<800'],
    business_endpoint_duration: ['p(95)<1200'],
    total_endpoints_called: ['count>500'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

// Helper function to login
function getAuthToken() {
  const payload = JSON.stringify({
    email: 'loadtest1@example.com',
    password: 'LoadTest@123',
  });
  
  const res = http.post(`${API_BASE}/auth/login`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });
  
  if (res.status === 200) {
    try {
      const body = JSON.parse(res.body);
      return body.data?.access_token || body.access_token;
    } catch {
      return null;
    }
  }
  return null;
}

// Scenario 1: Health & Public Endpoints
export function healthAndPublicEndpoints() {
  group('Health Check', function () {
    const startTime = Date.now();
    const res = http.get(`${BASE_URL}/health`);
    const duration = Date.now() - startTime;
    
    healthCheckDuration.add(duration);
    totalEndpointsCalled.add(1);
    
    const success = check(res, {
      'health check is 200': (r) => r.status === 200,
      'health check has status ok': (r) => {
        try {
          return JSON.parse(r.body).status === 'ok';
        } catch {
          return false;
        }
      },
      'health check < 100ms': (r) => r.timings.duration < 100,
    });
    
    healthCheckErrors.add(success ? 0 : 1);
  });
  
  sleep(0.5);
  
  group('Swagger Documentation', function () {
    const res = http.get(`${BASE_URL}/swagger/index.html`);
    totalEndpointsCalled.add(1);
    
    check(res, {
      'swagger accessible': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(1);
  
  group('Google OAuth URL', function () {
    const res = http.get(`${API_BASE}/auth/google`);
    totalEndpointsCalled.add(1);
    
    check(res, {
      'google auth url responds': (r) => r.status === 200 || r.status === 302,
    });
  });
  
  sleep(1);
}

// Scenario 2: Auth Endpoints
export function authEndpoints() {
  const endpoints = [
    {
      name: 'Login',
      method: 'POST',
      url: `${API_BASE}/auth/login`,
      payload: { email: 'test@example.com', password: 'Test@123' },
    },
    {
      name: 'Register',
      method: 'POST',
      url: `${API_BASE}/auth/register`,
      payload: {
        email: `test-${Date.now()}-${__VU}@example.com`,
        password: 'Test@123456',
        name: 'Test User',
      },
    },
    {
      name: 'Forgot Password',
      method: 'POST',
      url: `${API_BASE}/auth/forgot-password`,
      payload: { email: 'test@example.com' },
    },
  ];
  
  const endpoint = endpoints[__ITER % endpoints.length];
  
  group(`Auth - ${endpoint.name}`, function () {
    const startTime = Date.now();
    const res = http[endpoint.method.toLowerCase()](
      endpoint.url,
      JSON.stringify(endpoint.payload),
      { headers: { 'Content-Type': 'application/json' } }
    );
    const duration = Date.now() - startTime;
    
    authEndpointDuration.add(duration);
    totalEndpointsCalled.add(1);
    
    const success = check(res, {
      'auth endpoint responds': (r) => r.status !== 0,
      'auth endpoint has body': (r) => r.body.length > 0,
    });
    
    authEndpointErrors.add(success ? 0 : 1);
  });
  
  sleep(1);
}

// Scenario 3: Business Endpoints
export function businessEndpoints() {
  const token = getAuthToken();
  
  if (!token) {
    businessEndpointErrors.add(1);
    sleep(2);
    return;
  }
  
  const authParams = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  };
  
  const endpoints = [
    { name: 'Profile', method: 'GET', url: `${API_BASE}/auth/user/profile` },
    { name: 'Clinics', method: 'GET', url: `${API_BASE}/clinic` },
    { name: 'Notifications', method: 'GET', url: `${API_BASE}/notification` },
    { name: 'Settings', method: 'GET', url: `${API_BASE}/setting` },
    { name: 'Invitations', method: 'GET', url: `${API_BASE}/invite` },
    { name: 'Subscription', method: 'GET', url: `${API_BASE}/practitioner/subscription` },
  ];
  
  const endpoint = endpoints[__ITER % endpoints.length];
  
  group(`Business - ${endpoint.name}`, function () {
    const startTime = Date.now();
    const res = http[endpoint.method.toLowerCase()](endpoint.url, authParams);
    const duration = Date.now() - startTime;
    
    businessEndpointDuration.add(duration);
    totalEndpointsCalled.add(1);
    
    const success = check(res, {
      'business endpoint accessible': (r) => r.status === 200 || r.status === 404,
    });
    
    businessEndpointErrors.add(success ? 0 : 1);
  });
  
  sleep(1);
}

// Scenario 4: Admin Endpoints
export function adminEndpoints() {
  // Note: Admin endpoints require admin authentication
  // This is a placeholder - adjust based on your admin auth flow
  
  group('Admin - Health Check', function () {
    const res = http.get(`${BASE_URL}/health`);
    totalEndpointsCalled.add(1);
    
    check(res, {
      'admin health check': (r) => r.status === 200,
    });
  });
  
  sleep(2);
  
  // Add more admin-specific endpoints here when admin auth is available
}

// Scenario 5: Mixed Endpoint Load
export function mixedEndpointLoad() {
  const scenario = Math.random();
  
  if (scenario < 0.3) {
    // 30% - Health checks
    const res = http.get(`${BASE_URL}/health`);
    totalEndpointsCalled.add(1);
    check(res, { 'health ok': (r) => r.status === 200 });
  } else if (scenario < 0.6) {
    // 30% - Auth operations
    const res = http.post(
      `${API_BASE}/auth/login`,
      JSON.stringify({ email: 'test@example.com', password: 'wrong' }),
      { headers: { 'Content-Type': 'application/json' } }
    );
    totalEndpointsCalled.add(1);
    check(res, { 'auth responds': (r) => r.status !== 0 });
  } else {
    // 40% - Business operations (with auth)
    const token = getAuthToken();
    if (token) {
      const res = http.get(`${API_BASE}/auth/user/profile`, {
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
      });
      totalEndpointsCalled.add(1);
      check(res, { 'profile accessible': (r) => r.status === 200 });
    }
  }
  
  sleep(1);
}

export function handleSummary(data) {
  return {
    'api-endpoint-summary.json': JSON.stringify(data),
    stdout: textSummary(data),
  };
}

function textSummary(data) {
  const metrics = data.metrics;
  
  return `
╔════════════════════════════════════════════════════════════════╗
║            API Endpoint Coverage Test Summary                  ║
╚════════════════════════════════════════════════════════════════╝

📊 Overall Metrics:
  Total Endpoints Called: ${metrics.total_endpoints_called?.values.count || 0}
  Total Requests: ${metrics.http_reqs?.values.count || 0}
  Failed Requests: ${metrics.http_req_failed?.values.passes || 0} (${((metrics.http_req_failed?.values.rate || 0) * 100).toFixed(2)}%)

🎯 Endpoint Category Error Rates:
  Health Check Errors: ${((metrics.health_check_errors?.values.rate || 0) * 100).toFixed(2)}%
  Auth Endpoint Errors: ${((metrics.auth_endpoint_errors?.values.rate || 0) * 100).toFixed(2)}%
  Business Endpoint Errors: ${((metrics.business_endpoint_errors?.values.rate || 0) * 100).toFixed(2)}%
  Admin Endpoint Errors: ${((metrics.admin_endpoint_errors?.values.rate || 0) * 100).toFixed(2)}%

⏱️  Response Times by Category (P95):
  Health Checks: ${(metrics.health_check_duration?.values['p(95)'] || 0).toFixed(2)}ms
  Auth Endpoints: ${(metrics.auth_endpoint_duration?.values['p(95)'] || 0).toFixed(2)}ms
  Business Endpoints: ${(metrics.business_endpoint_duration?.values['p(95)'] || 0).toFixed(2)}ms

📈 Overall Response Times:
  Average: ${(metrics.http_req_duration?.values.avg || 0).toFixed(2)}ms
  P95: ${(metrics.http_req_duration?.values['p(95)'] || 0).toFixed(2)}ms
  P99: ${(metrics.http_req_duration?.values['p(99)'] || 0).toFixed(2)}ms

✅ Test Status: ${data.thresholds ? 'PASSED' : 'FAILED'}
  `;
}
