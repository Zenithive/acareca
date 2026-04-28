import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// =====================
// Custom Metrics
// =====================
const practitionerReadErrors = new Rate('practitioner_read_errors');
const practitionerWriteErrors = new Rate('practitioner_write_errors');
const practitionerReadDuration = new Trend('practitioner_read_duration');
const practitionerWriteDuration = new Trend('practitioner_write_duration');
const practitionerOperations = new Counter('practitioner_operations');

// =====================
// Config
// =====================
export const options = {
  discardResponseBodies: false,
  scenarios: {
    practitioner_list: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 20 },
        { duration: '30s', target: 10 },
        { duration: '30s', target: 0 },
      ],
      exec: 'listPractitioners',
    },
    practitioner_get: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 15 },
        { duration: '1m', target: 30 },
        { duration: '30s', target: 15 },
        { duration: '30s', target: 0 },
      ],
      exec: 'getPractitioner',
      startTime: '15s',
    },
    practitioner_search: {
      executor: 'constant-vus',
      vus: 10,
      duration: '2m',
      exec: 'searchPractitioners',
      startTime: '30s',
    },
    practitioner_pagination: {
      executor: 'ramping-vus',
      startVUs: 5,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 15 },
        { duration: '30s', target: 5 },
      ],
      exec: 'paginatePractitioners',
      startTime: '45s',
    },
    practitioner_bulk_read: {
      executor: 'constant-arrival-rate',
      rate: 3,
      timeUnit: '1s',
      duration: '2m',
      preAllocatedVUs: 15,
      exec: 'bulkReadPractitioners',
      startTime: '1m',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1500'],
    http_req_failed: ['rate<0.1'],
    practitioner_read_errors: ['rate<0.05'],
    practitioner_write_errors: ['rate<0.1'],
    practitioner_read_duration: ['p(95)<800'],
    practitioner_operations: ['count>100'],
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
    email: __ENV.TEST_EMAIL || 'mihir@yopmail.com',
    password: __ENV.TEST_PASSWORD || '@Demo1234',
  });

  const res = http.post(`${API_BASE}/auth/login`, loginPayload, {
    headers: { 'Content-Type': 'application/json' },
  });

  if (res.status !== 200) {
    throw new Error(`Login failed with status ${res.status}: ${res.body}`);
  }

  const body = JSON.parse(res.body);
  const token = body?.data?.access_token || body?.access_token;

  if (!token) {
    throw new Error(`Token missing in response: ${JSON.stringify(body)}`);
  }

  // Fetch practitioner list to get valid IDs
  const practitionersRes = http.get(`${API_BASE}/practitioner`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  let practitionerIds = [];
  if (practitionersRes.status === 200) {
    const practitionersBody = JSON.parse(practitionersRes.body);
    const practitioners = practitionersBody?.data?.items || [];
    practitionerIds = practitioners.map((p) => p.id).filter(Boolean);
    console.log(`Found ${practitionerIds.length} practitioner IDs`);
  }

  return { token, practitionerIds };
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

function trackedRequest(method, url, headers, body = null) {
  const start = Date.now();
  const res = body
    ? http[method](url, JSON.stringify(body), { headers })
    : http[method](url, { headers });
  const duration = Date.now() - start;

  practitionerOperations.add(1);
  practitionerReadDuration.add(duration);

  return res;
}

// =====================
// Scenario 1: List Practitioners
// =====================
export function listPractitioners(data) {
  const headers = getAuthHeaders(data.token);

  group('List All Practitioners', () => {
    const res = trackedRequest('get', `${API_BASE}/practitioner`, headers);
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 500ms': (r) => r.timings.duration < 500,
      'has data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data !== undefined;
        } catch {
          return false;
        }
      },
    });
    practitionerReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);
}

// =====================
// Scenario 2: Get Practitioner by ID
// =====================
export function getPractitioner(data) {
  const headers = getAuthHeaders(data.token);

  if (!data.practitionerIds || data.practitionerIds.length === 0) {
    console.log('No practitioner IDs available, skipping get test');
    sleep(1);
    return;
  }

  group('Get Practitioner by ID', () => {
    const randomId =
      data.practitionerIds[
        Math.floor(Math.random() * data.practitionerIds.length)
      ];
    const res = trackedRequest(
      'get',
      `${API_BASE}/practitioner/${randomId}`,
      headers
    );
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 300ms': (r) => r.timings.duration < 300,
      'has practitioner data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data?.id === randomId;
        } catch {
          return false;
        }
      },
    });
    practitionerReadErrors.add(ok ? 0 : 1);
  });

  sleep(0.5);
}

// =====================
// Scenario 3: Search Practitioners
// =====================
export function searchPractitioners(data) {
  const headers = getAuthHeaders(data.token);

  group('Search Practitioners', () => {
    const searchTerms = ['john', 'jane', 'test', 'demo', 'smith', 'clinic'];
    const term = searchTerms[Math.floor(Math.random() * searchTerms.length)];
    const res = trackedRequest(
      'get',
      `${API_BASE}/practitioner?search=${term}`,
      headers
    );
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1000ms': (r) => r.timings.duration < 1000,
      'has results': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data !== undefined;
        } catch {
          return false;
        }
      },
    });
    practitionerReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);
}

// =====================
// Scenario 4: Paginate Practitioners
// =====================
export function paginatePractitioners(data) {
  const headers = getAuthHeaders(data.token);

  group('Paginate Practitioners', () => {
    const limits = [5, 10, 20, 50];
    const limit = limits[Math.floor(Math.random() * limits.length)];
    const offset = Math.floor(Math.random() * 100);

    const res = trackedRequest(
      'get',
      `${API_BASE}/practitioner?limit=${limit}&offset=${offset}`,
      headers
    );
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 800ms': (r) => r.timings.duration < 800,
      'has pagination data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data?.items !== undefined;
        } catch {
          return false;
        }
      },
    });
    practitionerReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);
}

// =====================
// Scenario 5: Bulk Read Practitioners
// =====================
export function bulkReadPractitioners(data) {
  const headers = getAuthHeaders(data.token);

  group('Bulk Read Practitioners', () => {
    // Read multiple pages in sequence
    for (let i = 0; i < 3; i++) {
      const res = trackedRequest(
        'get',
        `${API_BASE}/practitioner?limit=20&offset=${i * 20}`,
        headers
      );
      check(res, {
        'status is 200': (r) => r.status === 200,
        'response time < 1000ms': (r) => r.timings.duration < 1000,
      });
      sleep(0.2);
    }
  });

  if (data.practitionerIds && data.practitionerIds.length > 0) {
    group('Bulk Get Individual Practitioners', () => {
      // Get multiple individual practitioners
      for (let i = 0; i < 5; i++) {
        const randomId =
          data.practitionerIds[
            Math.floor(Math.random() * data.practitionerIds.length)
          ];
        const res = trackedRequest(
          'get',
          `${API_BASE}/practitioner/${randomId}`,
          headers
        );
        check(res, {
          'status is 200': (r) => r.status === 200,
          'response time < 500ms': (r) => r.timings.duration < 500,
        });
        sleep(0.1);
      }
    });
  }

  sleep(1);
}

// =====================
// Summary
// =====================
export function handleSummary(data) {
  return {
    'practitioner-summary.json': JSON.stringify(data),
    stdout: textSummary(data),
  };
}

function textSummary(data) {
  const m = data.metrics;
  return `
========== PRACTITIONER TEST SUMMARY ==========
Total Operations:     ${m.practitioner_operations?.values.count || 0}
Total Requests:       ${m.http_reqs?.values.count || 0}
Failed Requests:      ${m.http_req_failed?.values.fails || 0}
Failure Rate:         ${((m.http_req_failed?.values.rate || 0) * 100).toFixed(2)}%

Read Error Rate:      ${((m.practitioner_read_errors?.values.rate || 0) * 100).toFixed(2)}%
P95 Read Duration:    ${(m.practitioner_read_duration?.values['p(95)'] || 0).toFixed(2)} ms
P99 Read Duration:    ${(m.practitioner_read_duration?.values['p(99)'] || 0).toFixed(2)} ms

Test Coverage:
✓ List Practitioners (with pagination)
✓ Get Practitioner by ID
✓ Search Practitioners
✓ Paginate Practitioners
✓ Bulk Read Operations
================================================
`;
}
