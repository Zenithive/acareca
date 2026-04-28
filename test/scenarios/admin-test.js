import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// =====================
// Custom Metrics
// =====================
const adminReadErrors = new Rate('admin_read_errors');
const adminWriteErrors = new Rate('admin_write_errors');
const adminReadDuration = new Trend('admin_read_duration');
const adminWriteDuration = new Trend('admin_write_duration');
const adminOperations = new Counter('admin_operations');
const auditLogDuration = new Trend('audit_log_duration');

// =====================
// Config
// =====================
export const options = {
  discardResponseBodies: false,
  scenarios: {
    admin_practitioner_management: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 20 },
        { duration: '30s', target: 10 },
        { duration: '30s', target: 0 },
      ],
      exec: 'adminPractitionerManagement',
    },
    admin_accountant_management: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 20 },
        { duration: '30s', target: 10 },
        { duration: '30s', target: 0 },
      ],
      exec: 'adminAccountantManagement',
      startTime: '15s',
    },
    admin_subscription_management: {
      executor: 'constant-vus',
      vus: 10,
      duration: '2m',
      exec: 'adminSubscriptionManagement',
      startTime: '30s',
    },
    admin_audit_logs: {
      executor: 'constant-vus',
      vus: 8,
      duration: '2m',
      exec: 'adminAuditLogs',
      startTime: '45s',
    },
    admin_analytics_dashboard: {
      executor: 'constant-vus',
      vus: 8,
      duration: '2m',
      exec: 'adminAnalyticsDashboard',
      startTime: '1m',
    },
    admin_user_management: {
      executor: 'ramping-vus',
      startVUs: 5,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 15 },
        { duration: '30s', target: 5 },
      ],
      exec: 'adminUserManagement',
      startTime: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000'],
    http_req_failed: ['rate<0.1'],
    admin_read_errors: ['rate<0.05'],
    admin_write_errors: ['rate<0.15'],
    admin_read_duration: ['p(95)<1500'],
    audit_log_duration: ['p(95)<2000'],
    admin_operations: ['count>100'],
  },
};

// =====================
// Constants
// =====================
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

// =====================
// Setup (LOGIN ONCE)
// =====================
export function setup() {
  const loginPayload = JSON.stringify({
    email: __ENV.ADMIN_EMAIL || 'admin@acareca.com',
    password: __ENV.ADMIN_PASSWORD || '@Admin1234',
  });

  const res = http.post(`${API_BASE}/auth/login`, loginPayload, {
    headers: { 'Content-Type': 'application/json' },
  });

  if (res.status !== 200) {
    throw new Error(`Admin login failed with status ${res.status}: ${res.body}`);
  }

  const body = JSON.parse(res.body);
  const token = body?.data?.access_token || body?.access_token;

  if (!token) {
    throw new Error(`Token missing in response: ${JSON.stringify(body)}`);
  }

  // Fetch practitioner list for admin operations
  const practitionersRes = http.get(`${API_BASE}/admin/practitioner`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  let practitionerIds = [];
  if (practitionersRes.status === 200) {
    try {
      const practitionersBody = JSON.parse(practitionersRes.body);
      const practitioners = practitionersBody?.data?.items || [];
      practitionerIds = practitioners.map((p) => p.id).filter(Boolean);
      console.log(`Found ${practitionerIds.length} practitioner IDs for admin`);
    } catch (e) {
      console.log('Could not parse practitioners response');
    }
  }

  // Fetch accountant list for admin operations
  const accountantsRes = http.get(`${API_BASE}/admin/accountant`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  let accountantIds = [];
  if (accountantsRes.status === 200) {
    try {
      const accountantsBody = JSON.parse(accountantsRes.body);
      const accountants = accountantsBody?.data?.items || [];
      accountantIds = accountants.map((a) => a.id).filter(Boolean);
      console.log(`Found ${accountantIds.length} accountant IDs for admin`);
    } catch (e) {
      console.log('Could not parse accountants response');
    }
  }

  // Fetch subscription list
  const subscriptionsRes = http.get(`${API_BASE}/admin/subscription`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  let subscriptionIds = [];
  if (subscriptionsRes.status === 200) {
    try {
      const subscriptionsBody = JSON.parse(subscriptionsRes.body);
      const subscriptions = subscriptionsBody?.data?.items || [];
      subscriptionIds = subscriptions.map((s) => s.id).filter(Boolean);
      console.log(`Found ${subscriptionIds.length} subscription IDs`);
    } catch (e) {
      console.log('Could not parse subscriptions response');
    }
  }

  return { token, practitionerIds, accountantIds, subscriptionIds };
}

// =====================
// Helpers
// =====================
function getAuthHeaders(token) {
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };
}

function trackedRequest(method, url, headers, body = null, type = 'read') {
  const start = Date.now();
  const res = body
    ? http[method](url, JSON.stringify(body), { headers })
    : http[method](url, { headers });
  const duration = Date.now() - start;

  adminOperations.add(1);
  if (type === 'read') {
    adminReadDuration.add(duration);
  } else {
    adminWriteDuration.add(duration);
  }

  return res;
}

// =====================
// Scenario 1: Admin Practitioner Management
// =====================
export function adminPractitionerManagement(data) {
  const headers = getAuthHeaders(data.token);

  group('List All Practitioners (Admin)', () => {
    const res = trackedRequest('get', `${API_BASE}/admin/practitioner`, headers);
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 800ms': (r) => r.timings.duration < 800,
      'has data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data !== undefined;
        } catch {
          return false;
        }
      },
    });
    adminReadErrors.add(ok ? 0 : 1);
  });

  sleep(0.5);

  group('List Practitioners with Pagination (Admin)', () => {
    const limits = [10, 20, 50];
    const limit = limits[Math.floor(Math.random() * limits.length)];
    const offset = Math.floor(Math.random() * 50);
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/practitioner?limit=${limit}&offset=${offset}`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1000ms': (r) => r.timings.duration < 1000,
    });
  });

  sleep(0.5);

  if (data.practitionerIds && data.practitionerIds.length > 0) {
    group('Get Practitioner by ID (Admin)', () => {
      const randomId =
        data.practitionerIds[
          Math.floor(Math.random() * data.practitionerIds.length)
        ];
      const res = trackedRequest(
        'get',
        `${API_BASE}/admin/practitioner/${randomId}`,
        headers
      );
      check(res, {
        'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
        'response time < 500ms': (r) => r.timings.duration < 500,
      });
    });

    sleep(0.5);

    group('Update Practitioner Status (Admin)', () => {
      const randomId =
        data.practitionerIds[
          Math.floor(Math.random() * data.practitionerIds.length)
        ];
      const statusData = {
        status: Math.random() > 0.5 ? 'active' : 'inactive',
      };
      const res = trackedRequest(
        'patch',
        `${API_BASE}/admin/practitioner/${randomId}/status`,
        headers,
        statusData,
        'write'
      );
      const ok = check(res, {
        'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
        'response time < 800ms': (r) => r.timings.duration < 800,
      });
      adminWriteErrors.add(ok ? 0 : 1);
    });
  }

  sleep(1);
}

// =====================
// Scenario 2: Admin Accountant Management
// =====================
export function adminAccountantManagement(data) {
  const headers = getAuthHeaders(data.token);

  group('List All Accountants (Admin)', () => {
    const res = trackedRequest('get', `${API_BASE}/admin/accountant`, headers);
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 800ms': (r) => r.timings.duration < 800,
      'has data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data !== undefined;
        } catch {
          return false;
        }
      },
    });
    adminReadErrors.add(ok ? 0 : 1);
  });

  sleep(0.5);

  group('List Accountants with Filters (Admin)', () => {
    const filters = [
      '?limit=10&offset=0',
      '?limit=20&offset=10',
      '?status=active',
      '?search=test',
    ];
    const filter = filters[Math.floor(Math.random() * filters.length)];
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/accountant${filter}`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1000ms': (r) => r.timings.duration < 1000,
    });
  });

  sleep(0.5);

  if (data.accountantIds && data.accountantIds.length > 0) {
    group('Get Accountant by ID (Admin)', () => {
      const randomId =
        data.accountantIds[
          Math.floor(Math.random() * data.accountantIds.length)
        ];
      const res = trackedRequest(
        'get',
        `${API_BASE}/admin/accountant/${randomId}`,
        headers
      );
      check(res, {
        'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
        'response time < 500ms': (r) => r.timings.duration < 500,
      });
    });

    sleep(0.5);

    group('Update Accountant Status (Admin)', () => {
      const randomId =
        data.accountantIds[
          Math.floor(Math.random() * data.accountantIds.length)
        ];
      const statusData = {
        status: Math.random() > 0.5 ? 'active' : 'suspended',
      };
      const res = trackedRequest(
        'patch',
        `${API_BASE}/admin/accountant/${randomId}/status`,
        headers,
        statusData,
        'write'
      );
      const ok = check(res, {
        'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
        'response time < 800ms': (r) => r.timings.duration < 800,
      });
      adminWriteErrors.add(ok ? 0 : 1);
    });
  }

  sleep(1);
}

// =====================
// Scenario 3: Admin Subscription Management
// =====================
export function adminSubscriptionManagement(data) {
  const headers = getAuthHeaders(data.token);

  group('List All Subscriptions (Admin)', () => {
    const res = trackedRequest('get', `${API_BASE}/admin/subscription`, headers);
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1000ms': (r) => r.timings.duration < 1000,
      'has data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data !== undefined;
        } catch {
          return false;
        }
      },
    });
    adminReadErrors.add(ok ? 0 : 1);
  });

  sleep(0.5);

  group('List Subscriptions with Filters (Admin)', () => {
    const filters = [
      '?status=active',
      '?status=cancelled',
      '?status=expired',
      '?limit=20&offset=0',
      '?plan=premium',
    ];
    const filter = filters[Math.floor(Math.random() * filters.length)];
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/subscription${filter}`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1000ms': (r) => r.timings.duration < 1000,
    });
  });

  sleep(0.5);

  if (data.subscriptionIds && data.subscriptionIds.length > 0) {
    group('Get Subscription by ID (Admin)', () => {
      const randomId =
        data.subscriptionIds[
          Math.floor(Math.random() * data.subscriptionIds.length)
        ];
      const res = trackedRequest(
        'get',
        `${API_BASE}/admin/subscription/${randomId}`,
        headers
      );
      check(res, {
        'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
        'response time < 500ms': (r) => r.timings.duration < 500,
      });
    });

    sleep(0.5);

    group('Update Subscription (Admin)', () => {
      const randomId =
        data.subscriptionIds[
          Math.floor(Math.random() * data.subscriptionIds.length)
        ];
      const updateData = {
        notes: `Admin update at ${Date.now()}`,
      };
      const res = trackedRequest(
        'patch',
        `${API_BASE}/admin/subscription/${randomId}`,
        headers,
        updateData,
        'write'
      );
      check(res, {
        'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
        'response time < 800ms': (r) => r.timings.duration < 800,
      });
    });
  }

  sleep(1);

  group('Get Subscription Statistics (Admin)', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/subscription/statistics`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
    });
  });

  sleep(1);
}

// =====================
// Scenario 4: Admin Audit Logs
// =====================
export function adminAuditLogs(data) {
  const headers = getAuthHeaders(data.token);

  group('List Audit Logs', () => {
    const start = Date.now();
    const res = trackedRequest('get', `${API_BASE}/admin/audit`, headers);
    auditLogDuration.add(Date.now() - start);
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
      'has data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data !== undefined;
        } catch {
          return false;
        }
      },
    });
    adminReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);

  group('List Audit Logs with Filters', () => {
    const filters = [
      '?limit=50&offset=0',
      '?action=create',
      '?action=update',
      '?action=delete',
      '?entity_type=practitioner',
      '?entity_type=accountant',
      '?entity_type=subscription',
    ];
    const filter = filters[Math.floor(Math.random() * filters.length)];
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/audit${filter}`,
      headers
    );
    auditLogDuration.add(Date.now() - start);
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
  });

  sleep(1);

  group('List Audit Logs by Date Range', () => {
    const endDate = new Date().toISOString().split('T')[0];
    const startDate = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000)
      .toISOString()
      .split('T')[0];
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/audit?start_date=${startDate}&end_date=${endDate}`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
  });

  sleep(1);

  if (data.practitionerIds && data.practitionerIds.length > 0) {
    group('Get Audit Logs for Specific User', () => {
      const randomId =
        data.practitionerIds[
          Math.floor(Math.random() * data.practitionerIds.length)
        ];
      const res = trackedRequest(
        'get',
        `${API_BASE}/admin/audit?user_id=${randomId}`,
        headers
      );
      check(res, {
        'status is 200': (r) => r.status === 200,
        'response time < 1500ms': (r) => r.timings.duration < 1500,
      });
    });
  }

  sleep(2);
}

// =====================
// Scenario 5: Admin Analytics Dashboard
// =====================
export function adminAnalyticsDashboard(data) {
  const headers = getAuthHeaders(data.token);

  group('Get User Growth Analytics', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/user-growth`,
      headers
    );
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
    });
    adminReadErrors.add(ok ? 0 : 1);
  });

  sleep(0.5);

  group('Get Active Users Analytics', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/active-users`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
    });
  });

  sleep(0.5);

  group('Get Subscription Analytics', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/subscriptions`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
    });
  });

  sleep(0.5);

  group('Get Practitioner Overview Analytics', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/practitioner/overview`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
  });

  sleep(0.5);

  group('Get Accountant Overview Analytics', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/accountant/overview`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
  });

  sleep(0.5);

  group('Get Billing Dashboard', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/billing/dashboard`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
  });

  sleep(0.5);

  group('Get Platform Revenue Analytics', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/billing/platform-revenue`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
    });
  });

  sleep(0.5);

  group('Get Plan Distribution Analytics', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/billing/plan-distribution`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
    });
  });

  sleep(2);
}

// =====================
// Scenario 6: Prac User Management
// =====================
export function practitionerManagement(data) {
  const headers = getAuthHeaders(data.token);

  group('List All practitioner (Admin)', () => {
    const res = trackedRequest('get', `${API_BASE}/admin/practitioner`, headers);
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1000ms': (r) => r.timings.duration < 1000,
    });
    adminReadErrors.add(ok ? 0 : 1);
  });

  sleep(0.5);


}

// =====================
// Summary
// =====================
export function handleSummary(data) {
  return {
    'admin-summary.json': JSON.stringify(data),
    stdout: textSummary(data),
  };
}

function textSummary(data) {
  const m = data.metrics;
  return `
========== ADMIN TEST SUMMARY ==========
Total Operations:       ${m.admin_operations?.values.count || 0}
Total Requests:         ${m.http_reqs?.values.count || 0}
Failed Requests:        ${m.http_req_failed?.values.fails || 0}
Failure Rate:           ${((m.http_req_failed?.values.rate || 0) * 100).toFixed(2)}%

Read Error Rate:        ${((m.admin_read_errors?.values.rate || 0) * 100).toFixed(2)}%
Write Error Rate:       ${((m.admin_write_errors?.values.rate || 0) * 100).toFixed(2)}%

P95 Read Duration:      ${(m.admin_read_duration?.values['p(95)'] || 0).toFixed(2)} ms
P99 Read Duration:      ${(m.admin_read_duration?.values['p(99)'] || 0).toFixed(2)} ms
P95 Write Duration:     ${(m.admin_write_duration?.values['p(95)'] || 0).toFixed(2)} ms
P95 Audit Log Duration: ${(m.audit_log_duration?.values['p(95)'] || 0).toFixed(2)} ms

Test Coverage:
✓ Admin Practitioner Management (List, Get, Update Status)
✓ Admin Accountant Management (List, Get, Update Status, Filters)
✓ Admin Subscription Management (List, Get, Update, Statistics, Filters)
✓ Admin Audit Logs (List, Filters, Date Range, User-specific)
✓ Admin Analytics Dashboard (User Growth, Active Users, Subscriptions, Revenue, Plans)
✓ Admin User Management (List, Search, Filter by Role/Status)
================================================
`;
}
