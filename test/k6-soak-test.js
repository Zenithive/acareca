import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const responseTime = new Trend('response_time');

// Soak test configuration - sustained load over extended period
export const options = {
  stages: [
    { duration: '5m', target: 50 },    // Ramp up to 50 users
    { duration: '30m', target: 50 },   // Stay at 50 users for 30 minutes
    { duration: '5m', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<800'],  // 95% of requests should be below 800ms
    http_req_failed: ['rate<0.05'],    // Error rate should be less than 5%
    errors: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

// Simulated user sessions
const sessions = [];

export function setup() {
  console.log('Starting soak test - this will run for ~40 minutes');
  console.log(`Base URL: ${BASE_URL}`);
  return { startTime: Date.now() };
}

export default function (data) {
  // Simulate realistic user behavior over time
  const scenario = Math.random();
  
  if (scenario < 0.3) {
    // 30% - Complete user journey
    userJourney();
  } else if (scenario < 0.6) {
    // 30% - API operations
    apiOperations();
  } else if (scenario < 0.85) {
    // 25% - Read-heavy operations
    readOperations();
  } else {
    // 15% - Health monitoring
    healthCheck();
  }
  
  sleep(2 + Math.random() * 3); // 2-5 seconds think time
}

function userJourney() {
  // 1. Health check
  let res = http.get(`${BASE_URL}/health`);
  check(res, { 'health ok': (r) => r.status === 200 });
  sleep(1);
  
  // 2. Login
  const loginPayload = JSON.stringify({
    email: `soak-test-${Math.floor(Math.random() * 50)}@example.com`,
    password: 'Test@123456',
  });
  
  res = http.post(`${API_BASE}/auth/login`, loginPayload, {
    headers: { 'Content-Type': 'application/json' },
  });
  
  const loginSuccess = check(res, {
    'login successful or expected error': (r) => r.status === 200 || r.status === 401,
  });
  
  if (loginSuccess && res.status === 200) {
    try {
      const loginData = JSON.parse(res.body);
      const token = loginData.data?.access_token || loginData.access_token;
      if (token) {
        sleep(2);
        
        // 3. Access protected resource
        res = http.get(`${API_BASE}/auth/user/profile`, {
          headers: {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        });
        
        check(res, {
          'profile access successful': (r) => r.status === 200,
        });
        
        sleep(3);
        
        // 4. Additional operations
        res = http.get(`${API_BASE}/notification`, {
          headers: {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        });
        
        check(res, {
          'notification access': (r) => r.status === 200 || r.status === 401,
        });
      }
    } catch (e) {
      errorRate.add(1);
    }
  }
}

function apiOperations() {
  // Test various API endpoints
  const operations = [
    () => http.get(`${BASE_URL}/health`),
    () => http.get(`${API_BASE}/auth/google`),
    () => http.post(`${API_BASE}/auth/login`, JSON.stringify({
      email: 'test@example.com',
      password: 'wrong',
    }), { headers: { 'Content-Type': 'application/json' } }),
  ];
  
  const operation = operations[Math.floor(Math.random() * operations.length)];
  const res = operation();
  
  responseTime.add(res.timings.duration);
  check(res, {
    'operation completed': (r) => r.status !== 0,
  });
  
  sleep(1);
}

function readOperations() {
  // Read-heavy operations
  const endpoints = [
    `${BASE_URL}/health`,
    `${BASE_URL}/swagger/index.html`,
    `${API_BASE}/auth/google`,
  ];
  
  const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  const res = http.get(endpoint);
  
  check(res, {
    'read successful': (r) => r.status === 200 || r.status === 302 || r.status === 404,
    'no memory leak indicators': (r) => r.timings.duration < 2000,
  });
  
  sleep(0.5);
}

function healthCheck() {
  const res = http.get(`${BASE_URL}/health`);
  
  check(res, {
    'health check ok': (r) => r.status === 200,
    'response time stable': (r) => r.timings.duration < 500,
  });
  
  responseTime.add(res.timings.duration);
}

export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000 / 60;
  console.log(`Soak test completed after ${duration.toFixed(2)} minutes`);
}
