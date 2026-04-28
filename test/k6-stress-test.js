import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Stress test configuration - gradually increase load beyond normal capacity
export const options = {
  stages: [
    { duration: '1m', target: 50 },    // Ramp up to 50 users
    { duration: '2m', target: 100 },   // Ramp up to 100 users
    { duration: '2m', target: 200 },   // Ramp up to 200 users
    { duration: '2m', target: 300 },   // Ramp up to 300 users
    { duration: '2m', target: 400 },   // Ramp up to 400 users (stress point)
    { duration: '2m', target: 400 },   // Stay at 400 users
    { duration: '2m', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'], // 95% of requests should be below 2s
    http_req_failed: ['rate<0.3'],     // Error rate should be less than 30%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

export default function () {
  // Realistic mix of operations under stress
  const scenario = Math.random();
  
  if (scenario < 0.4) {
    // 40% - Health checks
    const res = http.get(`${BASE_URL}/health`);
    check(res, {
      'health check successful': (r) => r.status === 200,
      'response time acceptable': (r) => r.timings.duration < 3000,
    });
  } else if (scenario < 0.7) {
    // 30% - Authentication
    const loginPayload = JSON.stringify({
      email: `stress-test-${Math.floor(Math.random() * 1000)}@example.com`,
      password: 'Test@123456',
    });
    
    const res = http.post(`${API_BASE}/auth/login`, loginPayload, {
      headers: { 'Content-Type': 'application/json' },
    });
    
    check(res, {
      'login response received': (r) => r.status !== 0,
      'no timeout': (r) => r.timings.duration < 5000,
    });
  } else if (scenario < 0.9) {
    // 20% - Registration attempts
    const registerPayload = JSON.stringify({
      email: `stress-${Date.now()}-${Math.random()}@example.com`,
      password: 'Test@123456',
      name: 'Stress Test User',
    });
    
    const res = http.post(`${API_BASE}/auth/register`, registerPayload, {
      headers: { 'Content-Type': 'application/json' },
    });
    
    check(res, {
      'register response received': (r) => r.status !== 0,
    });
  } else {
    // 10% - Swagger docs
    const res = http.get(`${BASE_URL}/swagger/index.html`);
    check(res, {
      'swagger accessible': (r) => r.status === 200 || r.status === 404,
    });
  }
  
  sleep(Math.random() * 2); // Variable think time
}

export function handleSummary(data) {
  return {
    'stress-test-summary.json': JSON.stringify(data),
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
  };
}

function textSummary(data, options) {
  // Simple text summary
  return `
Stress Test Summary
===================
Total Requests: ${data.metrics.http_reqs.values.count}
Failed Requests: ${data.metrics.http_req_failed.values.passes}
Avg Response Time: ${data.metrics.http_req_duration.values.avg.toFixed(2)}ms
P95 Response Time: ${data.metrics['http_req_duration{p(95)}'] ? data.metrics['http_req_duration{p(95)}'].values.toFixed(2) : 'N/A'}ms
  `;
}
