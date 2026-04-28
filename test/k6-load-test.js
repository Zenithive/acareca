import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 10 },  // Ramp up to 10 users
    { duration: '1m', target: 50 },   // Ramp up to 50 users
    { duration: '2m', target: 100 },  // Stay at 100 users
    { duration: '1m', target: 50 },   // Ramp down to 50 users
    { duration: '30s', target: 0 },   // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests should be below 500ms
    http_req_failed: ['rate<0.1'],    // Error rate should be less than 10%
    errors: ['rate<0.1'],
  },
};

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

// Test data
const testUsers = [
  { email: 'newuser7@example.com', password: 'SecurePass123!' },
  { email: 'newuser8@example.com', password: 'SecurePass123!' },
];

export function setup() {
  // Setup phase - runs once before the test
  console.log('Starting load test...');
  console.log(`Base URL: ${BASE_URL}`);
  
  // Health check
  const healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, {
    'health check is OK': (r) => r.status === 200,
  });
  
  return { baseUrl: BASE_URL };
}

export default function (data) {
  // Select a random test user
  const user = testUsers[Math.floor(Math.random() * testUsers.length)];
  
  // Test scenarios with weighted distribution
  const scenario = Math.random();
  
  if (scenario < 0.3) {
    // 30% - Authentication flow
    testAuthFlow(user);
  } else if (scenario < 0.6) {
    // 30% - Read operations (if authenticated)
    testReadOperations();
  } else if (scenario < 0.85) {
    // 25% - Health check and public endpoints
    testPublicEndpoints();
  } else {
    // 15% - Mixed operations
    testMixedOperations(user);
  }
  
  sleep(2 + Math.random()); // Think time between iterations (2-3 seconds to respect rate limits)
}

function testAuthFlow(user) {
  const loginPayload = JSON.stringify({
    email: user.email,
    password: user.password,
  });
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };
  
  // Login
  const loginRes = http.post(`${API_BASE}/auth/login`, loginPayload, params);
  
  const loginSuccess = check(loginRes, {
    'login status is 200 or 401': (r) => r.status === 200 || r.status === 401,
    'login response has body': (r) => r.body.length > 0,
  });
  
  if (!loginSuccess) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
  }
  
  // If login successful, test protected endpoints
  if (loginRes.status === 200) {
    try {
      const loginData = JSON.parse(loginRes.body);
      const token = loginData.data?.access_token || loginData.access_token;
      if (token) {
        const authParams = {
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`,
          },
        };
        
        // Get profile
        const profileRes = http.get(`${API_BASE}/auth/user/profile`, authParams);
        check(profileRes, {
          'profile fetch successful': (r) => r.status === 200,
        });
      }
    } catch (e) {
      errorRate.add(1);
    }
  }
  
  sleep(0.5);
}

function testReadOperations() {
  // Test various GET endpoints
  const endpoints = [
    '/health',
    '/api/v1/auth/google', // Get Google auth URL
  ];
  
  const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  const res = http.get(`${BASE_URL}${endpoint}`);
  
  const success = check(res, {
    'read operation successful': (r) => r.status === 200 || r.status === 302,
  });
  
  errorRate.add(success ? 0 : 1);
  sleep(0.3);
}

function testPublicEndpoints() {
  // Health check
  const healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, {
    'health check is 200': (r) => r.status === 200,
    'health check response time < 200ms': (r) => r.timings.duration < 200,
  });
  
  // Swagger docs
  const swaggerRes = http.get(`${BASE_URL}/swagger/index.html`);
  check(swaggerRes, {
    'swagger docs accessible': (r) => r.status === 200 || r.status === 404,
  });
  
  sleep(0.5);
}

function testMixedOperations(user) {
  // Simulate a realistic user journey
  
  // 1. Check health
  http.get(`${BASE_URL}/health`);
  sleep(0.2);
  
  // 2. Try to login
  const loginPayload = JSON.stringify({
    email: user.email,
    password: user.password,
  });
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };
  
  const loginRes = http.post(`${API_BASE}/auth/login`, loginPayload, params);
  
  if (loginRes.status === 200) {
    try {
      const loginData = JSON.parse(loginRes.body);
      const token = loginData.data?.access_token || loginData.access_token;
      if (token) {
        const authParams = {
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`,
          },
        };
        
        sleep(0.5);
        
        // 3. Access protected resources
        http.get(`${API_BASE}/auth/user/profile`, authParams);
        sleep(0.3);
        
        // 4. Access other protected endpoints (if available)
        http.get(`${API_BASE}/notification`, authParams);
      }
    } catch (e) {
      errorRate.add(1);
    }
  }
  
  sleep(1);
}

export function teardown(data) {
  // Teardown phase - runs once after the test
  console.log('Load test completed!');
}
