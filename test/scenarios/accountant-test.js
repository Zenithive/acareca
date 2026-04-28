import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// =====================
// Custom Metrics
// =====================
const accountantReadErrors = new Rate('accountant_read_errors');
const accountantWriteErrors = new Rate('accountant_write_errors');
const accountantReadDuration = new Trend('accountant_read_duration');
const accountantWriteDuration = new Trend('accountant_write_duration');
const accountantOperations = new Counter('accountant_operations');
const reportGenerationDuration = new Trend('report_generation_duration');

// =====================
// Config
// =====================
export const options = {
  discardResponseBodies: false,
  scenarios: {
    accountant_list: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 20 },
        { duration: '30s', target: 10 },
        { duration: '30s', target: 0 },
      ],
      exec: 'listAccountants',
    },
    accountant_analytics: {
      executor: 'constant-vus',
      vus: 8,
      duration: '2m',
      exec: 'accountantAnalytics',
      startTime: '30s',
    },
    pl_reports: {
      executor: 'constant-vus',
      vus: 8,
      duration: '2m',
      exec: 'plReports',
      startTime: '45s',
    },
    bas_reports: {
      executor: 'constant-vus',
      vus: 8,
      duration: '2m',
      exec: 'basReports',
      startTime: '1m',
    },
    financial_year_operations: {
      executor: 'ramping-vus',
      startVUs: 5,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 15 },
        { duration: '30s', target: 5 },
      ],
      exec: 'financialYearOperations',
      startTime: '30s',
    },
    coa_operations: {
      executor: 'ramping-vus',
      startVUs: 5,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 15 },
        { duration: '30s', target: 5 },
      ],
      exec: 'coaOperations',
      startTime: '45s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<3000'],
    http_req_failed: ['rate<0.15'],
    accountant_read_errors: ['rate<0.1'],
    accountant_write_errors: ['rate<0.2'],
    accountant_read_duration: ['p(95)<2000'],
    report_generation_duration: ['p(95)<5000'],
    accountant_operations: ['count>100'],
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

  // Fetch accountant list to get valid IDs
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
      console.log(`Found ${accountantIds.length} accountant IDs`);
    } catch (e) {
      console.log('Could not parse accountants response');
    }
  }

  // Get financial year ID
  const fyRes = http.get(`${API_BASE}/fy`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  let financialYearId = null;
  if (fyRes.status === 200) {
    try {
      const fyBody = JSON.parse(fyRes.body);
      const fys = fyBody?.data?.items || fyBody?.data || [];
      if (fys.length > 0) {
        financialYearId = fys[0].id;
        console.log(`Found financial year ID: ${financialYearId}`);
      }
    } catch (e) {
      console.log('Could not parse financial year response');
    }
  }

  // Fallback financial year ID if not found
  if (!financialYearId) {
    financialYearId = 'cddd4fef-20e8-4b56-8989-6cd98a37e4a5';
  }

  // Get clinic IDs for BAS reports
  const clinicsRes = http.get(`${API_BASE}/clinic`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  let clinicIds = [];
  if (clinicsRes.status === 200) {
    try {
      const clinicsBody = JSON.parse(clinicsRes.body);
      const clinics = clinicsBody?.data?.items || clinicsBody?.data || [];
      clinicIds = clinics.map((c) => c.id).filter(Boolean);
      console.log(`Found ${clinicIds.length} clinic IDs`);
    } catch (e) {
      console.log('Could not parse clinics response');
    }
  }

  return { token, accountantIds, financialYearId, clinicIds };
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

  accountantOperations.add(1);
  if (type === 'read') {
    accountantReadDuration.add(duration);
  } else {
    accountantWriteDuration.add(duration);
  }

  return res;
}

// =====================
// Scenario 1: List Accountants
// =====================
export function listAccountants(data) {
  const headers = getAuthHeaders(data.token);

  group('List All Accountants', () => {
    const res = trackedRequest('get', `${API_BASE}/admin/accountant`, headers);
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
    accountantReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);

  if (data.accountantIds && data.accountantIds.length > 0) {
    group('Get Accountant by ID', () => {
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
        'response time < 300ms': (r) => r.timings.duration < 300,
      });
    });
  }

  sleep(1);
}

// =====================
// Scenario 2: Accountant Analytics
// =====================
export function accountantAnalytics(data) {
  const headers = getAuthHeaders(data.token);

  group('Accountant Overview Analytics', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/accountant/overview`,
      headers
    );
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
      'has analytics data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data !== undefined;
        } catch {
          return false;
        }
      },
    });
    accountantReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);

  group('User Growth Analytics', () => {
    const res = trackedRequest(
      'get',
      `${API_BASE}/admin/analytics/user-growth`,
      headers
    );
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
    });
  });

  sleep(1);

  group('Subscription Metrics', () => {
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

  sleep(1);

  group('Billing Dashboard', () => {
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

  sleep(2);
}

// =====================
// Scenario 3: P&L Reports
// =====================
export function plReports(data) {
  const headers = getAuthHeaders(data.token);
  const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31`;

  group('P&L Monthly Summary', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/summary${queryParams}`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    const ok = check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
    accountantReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);

  group('P&L By Account', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/by-account${queryParams}`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
  });

  sleep(1);

  group('P&L By Responsibility', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/by-responsibility${queryParams}`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
  });

  sleep(1);

  group('P&L FY Summary', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/fy-summary?financial_year_id=${data.financialYearId}`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 3000ms': (r) => r.timings.duration < 3000,
    });
  });

  sleep(1);

  group('P&L Full Report', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/report${queryParams}`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 3000ms': (r) => r.timings.duration < 3000,
    });
  });

  sleep(1);

  group('Export P&L Report', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/export${queryParams}&format=pdf`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 5000ms': (r) => r.timings.duration < 5000,
    });
  });

  sleep(2);
}

// =====================
// Scenario 4: BAS Reports
// =====================
export function basReports(data) {
  const headers = getAuthHeaders(data.token);
  const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31`;

  group('BAS Report', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/bas/report${queryParams}`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    const ok = check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 3000ms': (r) => r.timings.duration < 3000,
    });
    accountantReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);

  group('BAS Preparation', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/bas/bas-preparation?financial_year_id=${data.financialYearId}&quarter=Q1`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 3000ms': (r) => r.timings.duration < 3000,
    });
  });

  sleep(1);

  group('Export BAS Report', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/bas/activity-statement/report/export${queryParams}&format=pdf`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 5000ms': (r) => r.timings.duration < 5000,
    });
  });

  sleep(1);

  group('Export BAS Preparation', () => {
    const start = Date.now();
    const res = trackedRequest(
      'get',
      `${API_BASE}/bas/bas-preparation/export?financial_year_id=${data.financialYearId}&quarter=Q1&format=excel`,
      headers
    );
    reportGenerationDuration.add(Date.now() - start);
    check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 5000ms': (r) => r.timings.duration < 5000,
    });
  });

  sleep(1);

  if (data.clinicIds && data.clinicIds.length > 0) {
    group('BAS Clinic Quarterly Summary', () => {
      const randomClinicId =
        data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
      const res = trackedRequest(
        'get',
        `${API_BASE}/bas/clinic/${randomClinicId}/summary?financial_year_id=${data.financialYearId}&quarter=Q1`,
        headers
      );
      check(res, {
        'status is 200, 400, or 404': (r) =>
          r.status === 200 || r.status === 400 || r.status === 404,
        'response time < 2000ms': (r) => r.timings.duration < 2000,
      });
    });

    sleep(1);

    group('BAS Clinic By Account', () => {
      const randomClinicId =
        data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
      const res = trackedRequest(
        'get',
        `${API_BASE}/bas/clinic/${randomClinicId}/by-account${queryParams}`,
        headers
      );
      check(res, {
        'status is 200, 400, or 404': (r) =>
          r.status === 200 || r.status === 400 || r.status === 404,
        'response time < 2000ms': (r) => r.timings.duration < 2000,
      });
    });

    sleep(1);

    group('BAS Clinic Monthly', () => {
      const randomClinicId =
        data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
      const res = trackedRequest(
        'get',
        `${API_BASE}/bas/clinic/${randomClinicId}/monthly?financial_year_id=${data.financialYearId}&month=2024-01`,
        headers
      );
      check(res, {
        'status is 200, 400, or 404': (r) =>
          r.status === 200 || r.status === 400 || r.status === 404,
        'response time < 2000ms': (r) => r.timings.duration < 2000,
      });
    });
  }

  sleep(2);
}

// =====================
// Scenario 5: Financial Year Operations
// =====================
export function financialYearOperations(data) {
  const headers = getAuthHeaders(data.token);

  group('List Financial Years', () => {
    const res = trackedRequest('get', `${API_BASE}/fy`, headers);
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
    accountantReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);

  if (data.financialYearId) {
    group('Get Financial Year by ID', () => {
      const res = trackedRequest(
        'get',
        `${API_BASE}/fy/${data.financialYearId}`,
        headers
      );
      check(res, {
        'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
        'response time < 300ms': (r) => r.timings.duration < 300,
      });
    });
  }

  sleep(1);
}

// =====================
// Scenario 6: Chart of Accounts Operations
// =====================
export function coaOperations(data) {
  const headers = getAuthHeaders(data.token);

  group('List Chart of Accounts', () => {
    const res = trackedRequest('get', `${API_BASE}/coa`, headers);
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
    accountantReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);

  group('List COA with Filters', () => {
    const filters = [
      '?type=asset',
      '?type=liability',
      '?type=income',
      '?type=expense',
      '?limit=20&offset=0',
    ];
    const filter = filters[Math.floor(Math.random() * filters.length)];
    const res = trackedRequest('get', `${API_BASE}/coa${filter}`, headers);
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1000ms': (r) => r.timings.duration < 1000,
    });
  });

  sleep(1);
}

// =====================
// Summary
// =====================
export function handleSummary(data) {
  return {
    'accountant-summary.json': JSON.stringify(data),
    stdout: textSummary(data),
  };
}

function textSummary(data) {
  const m = data.metrics;
  return `
========== ACCOUNTANT TEST SUMMARY ==========
Total Operations:       ${m.accountant_operations?.values.count || 0}
Total Requests:         ${m.http_reqs?.values.count || 0}
Failed Requests:        ${m.http_req_failed?.values.fails || 0}
Failure Rate:           ${((m.http_req_failed?.values.rate || 0) * 100).toFixed(2)}%

Read Error Rate:        ${((m.accountant_read_errors?.values.rate || 0) * 100).toFixed(2)}%
Write Error Rate:       ${((m.accountant_write_errors?.values.rate || 0) * 100).toFixed(2)}%

P95 Read Duration:      ${(m.accountant_read_duration?.values['p(95)'] || 0).toFixed(2)} ms
P99 Read Duration:      ${(m.accountant_read_duration?.values['p(99)'] || 0).toFixed(2)} ms
P95 Report Generation:  ${(m.report_generation_duration?.values['p(95)'] || 0).toFixed(2)} ms

Test Coverage:
✓ List Accountants
✓ Accountant Analytics (Overview, User Growth, Subscriptions, Billing)
✓ P&L Reports (Summary, By Account, By Responsibility, FY Summary, Export)
✓ BAS Reports (Report, Preparation, Export, Clinic-specific)
✓ Financial Year Operations
✓ Chart of Accounts Operations
================================================
`;
}
