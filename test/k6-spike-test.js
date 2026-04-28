import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Spike test configuration - sudden traffic spikes
export const options = {
  stages: [
    { duration: '10s', target: 10 },   // Warm up
    { duration: '30s', target: 10 },   // Stay at 10 users
    { duration: '10s', target: 200 },  // Spike to 200 users
    { duration: '1m', target: 200 },   // Stay at 200 users
    { duration: '10s', target: 10 },   // Drop back to 10 users
    { duration: '30s', target: 10 },   // Recovery
    { duration: '10s', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000'], // 95% of requests should be below 1s during spike
    http_req_failed: ['rate<0.2'],     // Error rate should be less than 20% during spike
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

export default function () {
  // Focus on high-traffic endpoints during spike
  const scenario = Math.random();
  
  if (scenario < 0.5) {
    // 50% - Health checks (most common)
    const res = http.get(`${BASE_URL}/health`);
    check(res, {
      'health check successful': (r) => r.status === 200,
    });
  } else if (scenario < 0.8) {
    // 30% - Login attempts
    const loginPayload = JSON.stringify({
      email: `user${Math.floor(Math.random() * 100)}@example.com`,
      password: 'Test@123456',
    });
    
    const res = http.post(`${API_BASE}/auth/login`, loginPayload, {
      headers: { 'Content-Type': 'application/json' },
    });
    
    check(res, {
      'login response received': (r) => r.status === 200 || r.status === 401,
    });
  } else {
    // 20% - Public endpoints
    const res = http.get(`${API_BASE}/auth/google`);
    check(res, {
      'public endpoint accessible': (r) => r.status === 200 || r.status === 302,
    });
  }
  
  sleep(0.1); // Minimal sleep during spike test
}
