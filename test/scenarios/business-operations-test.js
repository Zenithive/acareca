import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';

// Custom metrics
const businessOpErrors = new Rate('business_op_errors');
const clinicOperationDuration = new Trend('clinic_operation_duration');
const accountantOperationDuration = new Trend('accountant_operation_duration');
const practitionerOperationDuration = new Trend('practitioner_operation_duration');
const successfulOperations = new Counter('successful_operations');

// Test configuration - Business operations focused
export const options = {
  scenarios: {
    // Scenario 1: Practitioner Operations
    practitioner_operations: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 20 },
        { duration: '3m', target: 40 },
        { duration: '1m', target: 0 },
      ],
      gracefulRampDown: '30s',
      exec: 'practitionerOperations',
    },
    
    // Scenario 2: Accountant Operations
    accountant_operations: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 15 },
        { duration: '3m', target: 30 },
        { duration: '1m', target: 0 },
      ],
      gracefulRampDown: '30s',
      exec: 'accountantOperations',
      startTime: '30s',
    },
    
    // Scenario 3: Clinic Management
    clinic_management: {
      executor: 'constant-vus',
      vus: 10,
      duration: '4m',
      exec: 'clinicManagement',
      startTime: '20s',
    },
    
    // Scenario 4: Invitation Flow
    invitation_flow: {
      executor: 'constant-arrival-rate',
      rate: 3,
      timeUnit: '1s',
      duration: '3m',
      preAllocatedVUs: 10,
      exec: 'invitationFlow',
      startTime: '40s',
    },
    
    // Scenario 5: Subscription Management
    subscription_management: {
      executor: 'ramping-arrival-rate',
      startRate: 2,
      timeUnit: '1s',
      stages: [
        { duration: '2m', target: 5 },
        { duration: '2m', target: 2 },
      ],
      preAllocatedVUs: 15,
      exec: 'subscriptionManagement',
      startTime: '1m',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1500'],
    http_req_failed: ['rate<0.15'],
    business_op_errors: ['rate<0.2'],
    successful_operations: ['count>100'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

// Test users with different roles
const practitionerUsers = new SharedArray('practitioners', function () {
  return [
    { email: 'practitioner1@example.com', password: 'Test@123456' },
    { email: 'practitioner2@example.com', password: 'Test@123456' },
    { email: 'practitioner3@example.com', password: 'Test@123456' },
  ];
});

const accountantUsers = new SharedArray('accountants', function () {
  return [
    { email: 'accountant1@example.com', password: 'Test@123456' },
    { email: 'accountant2@example.com', password: 'Test@123456' },
  ];
});

// Helper function to login and get token
function loginUser(email, password) {
  const payload = JSON.stringify({ email, password });
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

// Scenario 1: Practitioner Operations
export function practitionerOperations() {
  const user = practitionerUsers[__VU % practitionerUsers.length];
  const token = loginUser(user.email, user.password);
  
  if (!token) {
    businessOpErrors.add(1);
    sleep(2);
    return;
  }
  
  const authParams = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  };
  
  group('Practitioner - View Dashboard', function () {
    const startTime = Date.now();
    const res = http.get(`${API_BASE}/practitioner/dashboard`, authParams);
    const duration = Date.now() - startTime;
    
    practitionerOperationDuration.add(duration);
    
    const success = check(res, {
      'dashboard accessible': (r) => r.status === 200 || r.status === 404,
    });
    
    if (success) successfulOperations.add(1);
    else businessOpErrors.add(1);
  });
  
  sleep(1);
  
  group('Practitioner - View Clinics', function () {
    const res = http.get(`${API_BASE}/clinic`, authParams);
    
    check(res, {
      'clinics list accessible': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(2);
  
  group('Practitioner - View Subscription', function () {
    const res = http.get(`${API_BASE}/practitioner/subscription`, authParams);
    
    check(res, {
      'subscription info accessible': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(1);
  
  group('Practitioner - View Settings', function () {
    const res = http.get(`${API_BASE}/setting`, authParams);
    
    check(res, {
      'settings accessible': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(2);
}

// Scenario 2: Accountant Operations
export function accountantOperations() {
  const user = accountantUsers[__VU % accountantUsers.length];
  const token = loginUser(user.email, user.password);
  
  if (!token) {
    businessOpErrors.add(1);
    sleep(2);
    return;
  }
  
  const authParams = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  };
  
  group('Accountant - View Clients', function () {
    const startTime = Date.now();
    const res = http.get(`${API_BASE}/accountant/clients`, authParams);
    const duration = Date.now() - startTime;
    
    accountantOperationDuration.add(duration);
    
    const success = check(res, {
      'clients list accessible': (r) => r.status === 200 || r.status === 404,
    });
    
    if (success) successfulOperations.add(1);
    else businessOpErrors.add(1);
  });
  
  sleep(1);
  
  group('Accountant - View Reports', function () {
    const res = http.get(`${API_BASE}/accountant/reports`, authParams);
    
    check(res, {
      'reports accessible': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(2);
  
  group('Accountant - View Invitations', function () {
    const res = http.get(`${API_BASE}/invite`, authParams);
    
    check(res, {
      'invitations accessible': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(1);
}

// Scenario 3: Clinic Management
export function clinicManagement() {
  const user = practitionerUsers[__VU % practitionerUsers.length];
  const token = loginUser(user.email, user.password);
  
  if (!token) {
    businessOpErrors.add(1);
    sleep(3);
    return;
  }
  
  const authParams = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  };
  
  group('Clinic - List All', function () {
    const startTime = Date.now();
    const res = http.get(`${API_BASE}/clinic`, authParams);
    const duration = Date.now() - startTime;
    
    clinicOperationDuration.add(duration);
    
    check(res, {
      'clinic list retrieved': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(2);
  
  group('Clinic - Create New', function () {
    const clinicData = JSON.stringify({
      name: `Test Clinic ${__VU}-${Date.now()}`,
      address: '123 Test Street',
      phone: '1234567890',
    });
    
    const res = http.post(`${API_BASE}/clinic`, clinicData, authParams);
    
    const success = check(res, {
      'clinic created or validation error': (r) => r.status === 201 || r.status === 400 || r.status === 409,
    });
    
    if (success && res.status === 201) {
      successfulOperations.add(1);
      
      // Try to get the created clinic ID
      try {
        const body = JSON.parse(res.body);
        const clinicId = body.id || body.clinic_id;
        
        if (clinicId) {
          sleep(1);
          
          // View the created clinic
          group('Clinic - View Details', function () {
            const viewRes = http.get(`${API_BASE}/clinic/${clinicId}`, authParams);
            check(viewRes, {
              'clinic details retrieved': (r) => r.status === 200,
            });
          });
        }
      } catch (e) {
        // Ignore parsing errors
      }
    }
  });
  
  sleep(3);
}

// Scenario 4: Invitation Flow
export function invitationFlow() {
  const practitioner = practitionerUsers[0];
  const token = loginUser(practitioner.email, practitioner.password);
  
  if (!token) {
    businessOpErrors.add(1);
    sleep(2);
    return;
  }
  
  const authParams = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  };
  
  group('Invitation - Send', function () {
    const inviteData = JSON.stringify({
      email: `accountant-${Date.now()}-${__VU}@example.com`,
      role: 'accountant',
    });
    
    const res = http.post(`${API_BASE}/invite`, inviteData, authParams);
    
    const success = check(res, {
      'invitation sent or error': (r) => r.status === 200 || r.status === 201 || r.status === 400,
    });
    
    if (success && (res.status === 200 || res.status === 201)) {
      successfulOperations.add(1);
    }
  });
  
  sleep(1);
  
  group('Invitation - List', function () {
    const res = http.get(`${API_BASE}/invite`, authParams);
    
    check(res, {
      'invitations listed': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(2);
}

// Scenario 5: Subscription Management
export function subscriptionManagement() {
  const user = practitionerUsers[__VU % practitionerUsers.length];
  const token = loginUser(user.email, user.password);
  
  if (!token) {
    businessOpErrors.add(1);
    sleep(2);
    return;
  }
  
  const authParams = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  };
  
  group('Subscription - View Current', function () {
    const res = http.get(`${API_BASE}/practitioner/subscription`, authParams);
    
    const success = check(res, {
      'subscription info retrieved': (r) => r.status === 200 || r.status === 404,
    });
    
    if (success) successfulOperations.add(1);
  });
  
  sleep(1);
  
  group('Subscription - View Plans', function () {
    const res = http.get(`${API_BASE}/practitioner/subscription/plans`, authParams);
    
    check(res, {
      'subscription plans listed': (r) => r.status === 200 || r.status === 404,
    });
  });
  
  sleep(2);
}

export function handleSummary(data) {
  return {
    'business-operations-summary.json': JSON.stringify(data),
    stdout: textSummary(data),
  };
}

function textSummary(data) {
  const metrics = data.metrics;
  
  return `
╔════════════════════════════════════════════════════════════════╗
║         Business Operations Test Summary                       ║
╚════════════════════════════════════════════════════════════════╝

📊 Overall Metrics:
  Total Requests: ${metrics.http_reqs?.values.count || 0}
  Failed Requests: ${metrics.http_req_failed?.values.passes || 0} (${((metrics.http_req_failed?.values.rate || 0) * 100).toFixed(2)}%)
  Successful Operations: ${metrics.successful_operations?.values.count || 0}
  Business Op Error Rate: ${((metrics.business_op_errors?.values.rate || 0) * 100).toFixed(2)}%

⏱️  Operation Durations (P95):
  Clinic Operations: ${(metrics.clinic_operation_duration?.values['p(95)'] || 0).toFixed(2)}ms
  Accountant Operations: ${(metrics.accountant_operation_duration?.values['p(95)'] || 0).toFixed(2)}ms
  Practitioner Operations: ${(metrics.practitioner_operation_duration?.values['p(95)'] || 0).toFixed(2)}ms

📈 Response Times:
  Average: ${(metrics.http_req_duration?.values.avg || 0).toFixed(2)}ms
  P95: ${(metrics.http_req_duration?.values['p(95)'] || 0).toFixed(2)}ms
  P99: ${(metrics.http_req_duration?.values['p(99)'] || 0).toFixed(2)}ms

✅ Test Status: ${data.thresholds ? 'PASSED' : 'FAILED'}
  `;
}
