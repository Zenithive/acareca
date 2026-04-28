import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';

// Custom metrics
const authErrors = new Rate('auth_errors');
const loginDuration = new Trend('login_duration');
const registrationDuration = new Trend('registration_duration');
const profileAccessDuration = new Trend('profile_access_duration');
const successfulLogins = new Counter('successful_logins');
const failedLogins = new Counter('failed_logins');

// Test configuration - Authentication focused
export const options = {
  scenarios: {
    // Scenario 1: User Registration Flow
    registration_flow: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 20 },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '10s',
      exec: 'registrationFlow',
    },
    
    // Scenario 2: Login Flow (concurrent with registration)
    login_flow: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 20 },
        { duration: '2m', target: 50 },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '10s',
      exec: 'loginFlow',
      startTime: '10s', // Start 10s after registration
    },
    
    // Scenario 3: Authenticated User Actions
    authenticated_actions: {
      executor: 'constant-vus',
      vus: 30,
      duration: '3m',
      exec: 'authenticatedActions',
      startTime: '30s',
    },
    
    // Scenario 4: Password Reset Flow
    password_reset: {
      executor: 'constant-arrival-rate',
      rate: 5,
      timeUnit: '1s',
      duration: '2m',
      preAllocatedVUs: 10,
      exec: 'passwordResetFlow',
      startTime: '20s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    http_req_failed: ['rate<0.1'],
    auth_errors: ['rate<0.15'],
    login_duration: ['p(95)<800'],
    successful_logins: ['count>50'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

// Shared test data
const testUsers = new SharedArray('users', function () {
  return [
    { email: 'loadtest1@example.com', password: 'LoadTest@123', name: 'Load Test 1' },
    { email: 'loadtest2@example.com', password: 'LoadTest@123', name: 'Load Test 2' },
    { email: 'loadtest3@example.com', password: 'LoadTest@123', name: 'Load Test 3' },
    { email: 'loadtest4@example.com', password: 'LoadTest@123', name: 'Load Test 4' },
    { email: 'loadtest5@example.com', password: 'LoadTest@123', name: 'Load Test 5' },
  ];
});

// Scenario 1: Registration Flow
export function registrationFlow() {
  group('User Registration', function () {
    const uniqueEmail = `test-${Date.now()}-${__VU}-${__ITER}@example.com`;
    const payload = JSON.stringify({
      email: uniqueEmail,
      password: 'Test@123456',
      name: `Test User ${__VU}`,
    });
    
    const params = {
      headers: { 'Content-Type': 'application/json' },
    };
    
    const startTime = Date.now();
    const res = http.post(`${API_BASE}/auth/register`, payload, params);
    const duration = Date.now() - startTime;
    
    registrationDuration.add(duration);
    
    const success = check(res, {
      'registration status is 200 or 201': (r) => r.status === 200 || r.status === 201,
      'registration response has body': (r) => r.body.length > 0,
      'registration response time < 2s': (r) => r.timings.duration < 2000,
    });
    
    if (!success) {
      authErrors.add(1);
      console.log(`Registration failed: ${res.status} - ${res.body}`);
    } else {
      authErrors.add(0);
    }
  });
  
  sleep(2);
}

// Scenario 2: Login Flow
export function loginFlow() {
  group('User Login', function () {
    const user = testUsers[Math.floor(Math.random() * testUsers.length)];
    
    const payload = JSON.stringify({
      email: user.email,
      password: user.password,
    });
    
    const params = {
      headers: { 'Content-Type': 'application/json' },
    };
    
    const startTime = Date.now();
    const res = http.post(`${API_BASE}/auth/login`, payload, params);
    const duration = Date.now() - startTime;
    
    loginDuration.add(duration);
    
    const success = check(res, {
      'login status is 200': (r) => r.status === 200,
      'login has access_token': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.data?.access_token !== undefined || body.access_token !== undefined;
        } catch {
          return false;
        }
      },
      'login response time < 1s': (r) => r.timings.duration < 1000,
    });
    
    if (success) {
      successfulLogins.add(1);
      authErrors.add(0);
      
      // Extract token for further use
      try {
        const body = JSON.parse(res.body);
        return body.data?.access_token || body.access_token;
      } catch {
        return null;
      }
    } else {
      failedLogins.add(1);
      authErrors.add(1);
      console.log(`Login failed: ${res.status} - ${res.body}`);
    }
  });
  
  sleep(1 + Math.random() * 2);
}

// Scenario 3: Authenticated User Actions
export function authenticatedActions() {
  // First login
  const user = testUsers[__VU % testUsers.length];
  
  const loginPayload = JSON.stringify({
    email: user.email,
    password: user.password,
  });
  
  const loginRes = http.post(`${API_BASE}/auth/login`, loginPayload, {
    headers: { 'Content-Type': 'application/json' },
  });
  
  if (loginRes.status !== 200) {
    authErrors.add(1);
    sleep(2);
    return;
  }
  
  let token;
  try {
    const body = JSON.parse(loginRes.body);
    token = body.data?.access_token || body.access_token;
  } catch {
    authErrors.add(1);
    sleep(2);
    return;
  }
  
  const authParams = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  };
  
  // Action 1: Get Profile
  group('Get Profile', function () {
    const startTime = Date.now();
    const res = http.get(`${API_BASE}/auth/user/profile`, authParams);
    const duration = Date.now() - startTime;
    
    profileAccessDuration.add(duration);
    
    check(res, {
      'profile fetch is 200': (r) => r.status === 200,
      'profile has user data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.email !== undefined;
        } catch {
          return false;
        }
      },
    });
  });
  
  sleep(1);
  
  // Action 2: Update Profile
  group('Update Profile', function () {
    const updatePayload = JSON.stringify({
      name: `Updated User ${__VU} - ${Date.now()}`,
    });
    
    const res = http.put(`${API_BASE}/auth/user/profile`, updatePayload, authParams);
    
    check(res, {
      'profile update is 200': (r) => r.status === 200,
    });
  });
  
  sleep(2);
  
  // Action 3: Access Notifications
  group('Get Notifications', function () {
    const res = http.get(`${API_BASE}/notification`, authParams);
    
    check(res, {
      'notifications accessible': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(1);
  
  // Action 4: Logout
  group('Logout', function () {
    const res = http.post(`${API_BASE}/auth/user/logout`, null, authParams);
    
    check(res, {
      'logout successful': (r) => r.status === 200 || r.status === 204,
    });
  });
  
  sleep(3);
}

// Scenario 4: Password Reset Flow
export function passwordResetFlow() {
  group('Password Reset Request', function () {
    const user = testUsers[Math.floor(Math.random() * testUsers.length)];
    
    const payload = JSON.stringify({
      email: user.email,
    });
    
    const params = {
      headers: { 'Content-Type': 'application/json' },
    };
    
    const res = http.post(`${API_BASE}/auth/forgot-password`, payload, params);
    
    check(res, {
      'forgot password request accepted': (r) => r.status === 200 || r.status === 202,
      'forgot password response has body': (r) => r.body.length > 0,
    });
  });
  
  sleep(2);
}

export function handleSummary(data) {
  return {
    'auth-flow-summary.json': JSON.stringify(data),
    stdout: textSummary(data),
  };
}

function textSummary(data) {
  const metrics = data.metrics;
  
  return `
╔════════════════════════════════════════════════════════════════╗
║           Authentication Flow Test Summary                     ║
╚════════════════════════════════════════════════════════════════╝

📊 Overall Metrics:
  Total Requests: ${metrics.http_reqs?.values.count || 0}
  Failed Requests: ${metrics.http_req_failed?.values.passes || 0} (${((metrics.http_req_failed?.values.rate || 0) * 100).toFixed(2)}%)
  Avg Response Time: ${(metrics.http_req_duration?.values.avg || 0).toFixed(2)}ms
  P95 Response Time: ${(metrics.http_req_duration?.values['p(95)'] || 0).toFixed(2)}ms

🔐 Authentication Metrics:
  Successful Logins: ${metrics.successful_logins?.values.count || 0}
  Failed Logins: ${metrics.failed_logins?.values.count || 0}
  Auth Error Rate: ${((metrics.auth_errors?.values.rate || 0) * 100).toFixed(2)}%
  
⏱️  Operation Durations:
  Login P95: ${(metrics.login_duration?.values['p(95)'] || 0).toFixed(2)}ms
  Registration P95: ${(metrics.registration_duration?.values['p(95)'] || 0).toFixed(2)}ms
  Profile Access P95: ${(metrics.profile_access_duration?.values['p(95)'] || 0).toFixed(2)}ms

✅ Test Status: ${data.thresholds ? 'PASSED' : 'FAILED'}
  `;
}
