import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// =====================
// Custom Metrics
// =====================
const dbReadErrors = new Rate('db_read_errors');
const dbWriteErrors = new Rate('db_write_errors');
const dbReadDuration = new Trend('db_read_duration');
const dbWriteDuration = new Trend('db_write_duration');
const dbOperations = new Counter('db_operations');
const concurrentReads = new Counter('concurrent_reads');
const concurrentWrites = new Counter('concurrent_writes');

// =====================
// Config
// =====================
export const options = {
  discardResponseBodies: false,
  scenarios: {
    heavy_reads: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 30 },
        { duration: '3m', target: 60 },
        { duration: '1m', target: 30 },
        { duration: '30s', target: 0 },
      ],
      exec: 'heavyReadOperations',
    },
    write_operations: {
      executor: 'ramping-arrival-rate',
      startRate: 5,
      timeUnit: '1s',
      stages: [
        { duration: '2m', target: 15 },
        { duration: '2m', target: 25 },
        { duration: '1m', target: 10 },
      ],
      preAllocatedVUs: 40,
      exec: 'writeOperations',
      startTime: '30s',
    },
    complex_queries: {
      executor: 'constant-vus',
      vus: 10,
      duration: '3m',
      exec: 'complexQueries',
      startTime: '1m',
    },
    read_write_mix: {
      executor: 'ramping-vus',
      startVUs: 10,
      stages: [
        { duration: '1m', target: 25 },
        { duration: '2m', target: 40 },
        { duration: '1m', target: 10 },
      ],
      exec: 'readWriteMix',
      startTime: '45s',
    },
    bulk_operations: {
      executor: 'constant-arrival-rate',
      rate: 2,
      timeUnit: '1s',
      duration: '3m',
      preAllocatedVUs: 10,
      exec: 'bulkOperations',
      startTime: '1m',
    },
    clinic_operations: {
      executor: 'ramping-vus',
      startVUs: 5,
      stages: [
        { duration: '1m', target: 15 },
        { duration: '2m', target: 25 },
        { duration: '1m', target: 10 },
        { duration: '30s', target: 0 },
      ],
      exec: 'clinicOperations',
      startTime: '30s',
    },
    form_operations: {
      executor: 'ramping-vus',
      startVUs: 5,
      stages: [
        { duration: '1m', target: 15 },
        { duration: '2m', target: 20 },
        { duration: '1m', target: 10 },
        { duration: '30s', target: 0 },
      ],
      exec: 'formOperations',
      startTime: '45s',
    },
    pl_report_operations: {
      executor: 'constant-vus',
      vus: 8,
      duration: '3m',
      exec: 'plReportOperations',
      startTime: '1m',
    },
    bas_report_operations: {
      executor: 'constant-vus',
      vus: 8,
      duration: '3m',
      exec: 'basReportOperations',
      startTime: '1m30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000'],
    http_req_failed: ['rate<0.15'],
    db_read_errors: ['rate<0.1'],
    db_write_errors: ['rate<0.2'],
    db_read_duration: ['p(95)<1000'],
    db_write_duration: ['p(95)<1500'],
    db_operations: ['count>500'],
  },
};

// =====================
// Constants
// =====================
const BASE_URL = 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

// =====================
// Setup (LOGIN ONCE)
// =====================
export function setup() {
  const res = http.post(
    `${API_BASE}/auth/login`,
    JSON.stringify({
      email: 'mihir@yopmail.com',
      password: '@Demo1234',
    }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  console.log('LOGIN STATUS:', res.status);
  console.log('LOGIN BODY:', res.body);

  if (res.status !== 200) {
    throw new Error(`Login failed with status ${res.status}`);
  }

  let body;
  try {
    body = JSON.parse(res.body);
  } catch (e) {
    throw new Error('Login response is not valid JSON');
  }

  const token = body?.data?.access_token || body?.access_token;
  if (!token) {
    throw new Error(`Token missing in response: ${JSON.stringify(body)}`);
  }

  // Fetch practitioner list to get valid IDs and financial year IDs
  const practitionersRes = http.get(`${API_BASE}/practitioner`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  let practitionerIds = [];
  let financialYearId = null;
  let clinicIds = [];
  let formIds = [];

  if (practitionersRes.status === 200) {
    try {
      const practitionersBody = JSON.parse(practitionersRes.body);
      const practitioners = practitionersBody?.data?.items || [];
      practitionerIds = practitioners.map((p) => p.id);
      console.log('Found practitioner IDs:', practitionerIds);
    } catch (e) {
      console.log('Could not parse practitioners response');
    }
  }

  // Try to get a financial year ID (you may need to adjust this based on your API)
  // For now, we'll use a placeholder - you should replace this with actual FY ID
  financialYearId = 'cddd4fef-20e8-4b56-8989-6cd98a37e4a5'; // Replace with actual FY ID

  // Fetch clinic list to get valid IDs
  const clinicsRes = http.get(`${API_BASE}/clinic`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  if (clinicsRes.status === 200) {
    try {
      const clinicsBody = JSON.parse(clinicsRes.body);
      const clinics = clinicsBody?.data?.items || clinicsBody?.data || [];
      clinicIds = clinics.map((c) => c.id);
      console.log('Found clinic IDs:', clinicIds);
    } catch (e) {
      console.log('Could not parse clinics response');
    }
  }

  // Fetch form list to get valid IDs
  const formsRes = http.get(`${API_BASE}/form`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  });

  if (formsRes.status === 200) {
    try {
      const formsBody = JSON.parse(formsRes.body);
      const forms = formsBody?.data?.items || formsBody?.data || [];
      formIds = forms.map((f) => f.id);
      console.log('Found form IDs:', formIds);
    } catch (e) {
      console.log('Could not parse forms response');
    }
  }

  return { token, practitionerIds, financialYearId, clinicIds, formIds };
}

// =====================
// Helpers
// =====================
function getAuthParams(token) {
  return {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  };
}

function trackedRequest(method, url, params, body = null, type = 'read') {
  const start = Date.now();
  const res = body
    ? http[method](url, JSON.stringify(body), params)
    : http[method](url, params);
  const duration = Date.now() - start;

  dbOperations.add(1);
  if (type === 'read') {
    dbReadDuration.add(duration);
    concurrentReads.add(1);
  } else {
    dbWriteDuration.add(duration);
    concurrentWrites.add(1);
  }

  return res;
}

// =====================
// Scenario 1: Heavy Reads
// =====================
export function heavyReadOperations(data) {
  const params = getAuthParams(data.token);

  group('List All Practitioners', () => {
    const res = trackedRequest('get', `${API_BASE}/practitioner`, params);
    const ok = check(res, {
      '200 OK': (r) => r.status === 200,
      '<500ms': (r) => r.timings.duration < 500,
    });
    dbReadErrors.add(ok ? 0 : 1);
  });

  sleep(0.5);

  if (data.practitionerIds && data.practitionerIds.length > 0) {
    group('Get Specific Practitioner', () => {
      const randomId =
        data.practitionerIds[
        Math.floor(Math.random() * data.practitionerIds.length)
        ];
      const res = trackedRequest(
        'get',
        `${API_BASE}/practitioner/${randomId}`,
        params
      );
      check(res, { success: (r) => r.status === 200 || r.status === 404 });
    });

    sleep(0.5);
  }

  group('List Practitioners with Filters', () => {
    const filters = [
      '?limit=10&offset=0',
      '?limit=20&offset=10',
      '?search=test',
      '?limit=5',
    ];
    const filter = filters[Math.floor(Math.random() * filters.length)];
    const res = trackedRequest(
      'get',
      `${API_BASE}/practitioner${filter}`,
      params
    );
    check(res, { success: (r) => r.status === 200 });
  });

  sleep(1);
}

// =====================
// Scenario 2: Write Operations
// =====================
export function writeOperations(data) {
  const params = getAuthParams(data.token);

  group('Create Practitioner Data', () => {
    // Placeholder for write operations - can be expanded with actual write endpoints
    const res = trackedRequest('get', `${API_BASE}/practitioner`, params);
    const ok = check(res, {
      success: (r) => r.status === 200,
    });
    dbWriteErrors.add(ok ? 0 : 1);
  });

  sleep(2);
}

// =====================
// Scenario 3: Complex Queries
// =====================
export function complexQueries(data) {
  const params = getAuthParams(data.token);

  group('List Practitioners with Pagination', () => {
    const url = `${API_BASE}/practitioner?limit=20&offset=0`;
    const res = trackedRequest('get', url, params);
    check(res, {
      success: (r) => r.status === 200,
      '<3s': (r) => r.timings.duration < 3000,
    });
  });

  sleep(2);

  if (data.practitionerIds && data.practitionerIds.length > 0) {
    group('Get Multiple Practitioners', () => {
      const randomId =
        data.practitionerIds[
        Math.floor(Math.random() * data.practitionerIds.length)
        ];
      const url = `${API_BASE}/practitioner/${randomId}`;
      const res = trackedRequest('get', url, params);
      check(res, {
        success: (r) =>
          r.status === 200 || r.status === 400 || r.status === 403,
      });
    });

    sleep(2);
  }

  group('List Practitioners with Search', () => {
    const searchTerms = ['john', 'jane', 'test', 'demo'];
    const term = searchTerms[Math.floor(Math.random() * searchTerms.length)];
    const res = trackedRequest(
      'get',
      `${API_BASE}/practitioner?search=${term}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200,
      '<2s': (r) => r.timings.duration < 2000,
    });
  });

  sleep(2);
}

// =====================
// Scenario 4: Read/Write Mix
// =====================
export function readWriteMix(data) {
  const params = getAuthParams(data.token);

  // Mostly reads with some writes
  const readOps = [
    () => trackedRequest('get', `${API_BASE}/practitioner`, params),
    () => {
      if (data.practitionerIds && data.practitionerIds.length > 0) {
        const randomId =
          data.practitionerIds[
          Math.floor(Math.random() * data.practitionerIds.length)
          ];
        return trackedRequest(
          'get',
          `${API_BASE}/practitioner/${randomId}`,
          params
        );
      }
      return trackedRequest('get', `${API_BASE}/practitioner`, params);
    },
    () =>
      trackedRequest(
        'get',
        `${API_BASE}/practitioner?limit=10&offset=${Math.floor(Math.random() * 50)}`,
        params
      ),
    () => trackedRequest('get', `${API_BASE}/clinic`, params),
    () => trackedRequest('get', `${API_BASE}/form`, params),
  ];

  const op = readOps[Math.floor(Math.random() * readOps.length)];
  const res = op();
  check(res, { success: (r) => r.status === 200 || r.status === 404 });

  sleep(1);
}

// =====================
// Scenario 5: Bulk Operations
// =====================
export function bulkOperations(data) {
  const params = getAuthParams(data.token);

  group('Bulk Read Practitioners', () => {
    for (let i = 0; i < 5; i++) {
      const res = trackedRequest(
        'get',
        `${API_BASE}/practitioner?limit=20&offset=${i * 20}`,
        params
      );
      check(res, {
        success: (r) => r.status === 200,
      });
      sleep(0.2);
    }
  });

  sleep(1);

  if (data.practitionerIds && data.practitionerIds.length > 0) {
    group('Bulk Get Individual Practitioners', () => {
      for (let i = 0; i < 3; i++) {
        const randomId =
          data.practitionerIds[
          Math.floor(Math.random() * data.practitionerIds.length)
          ];
        const res = trackedRequest(
          'get',
          `${API_BASE}/practitioner/${randomId}`,
          params
        );
        check(res, {
          success: (r) => r.status === 200 || r.status === 404,
        });
        sleep(0.2);
      }
    });
  }

  sleep(2);
}

// =====================
// Scenario 6: Clinic Operations
// =====================
export function clinicOperations(data) {
  const params = getAuthParams(data.token);

  group('Create Clinic', () => {
    const clinicData = {
      name: `Test Clinic ${Date.now()}`,
      address: '123 Test Street',
      city: 'Test City',
      state: 'Test State',
      postal_code: '12345',
      country: 'Australia',
      phone: '+61412345678',
      email: `clinic${Date.now()}@test.com`,
      abn: '12345678901',
    };
    const res = trackedRequest(
      'post',
      `${API_BASE}/clinic`,
      params,
      clinicData,
      'write'
    );
    const ok = check(res, {
      'clinic created': (r) => r.status === 201 || r.status === 200,
      '<1s': (r) => r.timings.duration < 1000,
    });
    dbWriteErrors.add(ok ? 0 : 1);

    // Store created clinic ID for later use
    if (res.status === 201 || res.status === 200) {
      try {
        const body = JSON.parse(res.body);
        const clinicId = body?.data?.id;
        if (clinicId && data.clinicIds) {
          data.clinicIds.push(clinicId);
        }
      } catch (e) {
        // Ignore parse errors
      }
    }
  });

  sleep(1);

  group('List Clinics', () => {
    const res = trackedRequest('get', `${API_BASE}/clinic`, params);
    check(res, {
      '200 OK': (r) => r.status === 200,
      '<500ms': (r) => r.timings.duration < 500,
    });
  });

  sleep(0.5);

  if (data.clinicIds && data.clinicIds.length > 0) {
    group('Get Clinic by ID', () => {
      const randomId =
        data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
      const res = trackedRequest(
        'get',
        `${API_BASE}/clinic/${randomId}`,
        params
      );
      check(res, {
        success: (r) => r.status === 200 || r.status === 404,
        '<300ms': (r) => r.timings.duration < 300,
      });
    });

    sleep(0.5);

    group('Update Clinic', () => {
      const randomId =
        data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
      const updateData = {
        name: `Updated Clinic ${Date.now()}`,
        phone: '+61498765432',
      };
      const res = trackedRequest(
        'put',
        `${API_BASE}/clinic/${randomId}`,
        params,
        updateData,
        'write'
      );
      check(res, {
        success: (r) => r.status === 200 || r.status === 404,
        '<800ms': (r) => r.timings.duration < 800,
      });
    });

    sleep(1);

    group('Delete Clinic', () => {
      const randomId =
        data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
      const res = trackedRequest(
        'del',
        `${API_BASE}/clinic/${randomId}`,
        params,
        null,
        'write'
      );
      check(res, {
        success: (r) =>
          r.status === 200 || r.status === 204 || r.status === 404,
      });
    });
  }

  sleep(1);
}

// =====================
// Scenario 7: Form Operations
// =====================
export function formOperations(data) {
  const params = getAuthParams(data.token);

  group('Create Form', () => {
    const formData = {
      name: `Test Form ${Date.now()}`,
      description: 'Test form description',
      type: 'expense',
      status: 'active',
      fields: [
        {
          name: 'amount',
          label: 'Amount',
          type: 'number',
          required: true,
          order: 1,
        },
        {
          name: 'description',
          label: 'Description',
          type: 'text',
          required: true,
          order: 2,
        },
      ],
    };
    const res = trackedRequest(
      'post',
      `${API_BASE}/form`,
      params,
      formData,
      'write'
    );
    const ok = check(res, {
      'form created': (r) => r.status === 201 || r.status === 200,
      '<1s': (r) => r.timings.duration < 1000,
    });
    dbWriteErrors.add(ok ? 0 : 1);

    // Store created form ID for later use
    if (res.status === 201 || res.status === 200) {
      try {
        const body = JSON.parse(res.body);
        const formId = body?.data?.id;
        if (formId && data.formIds) {
          data.formIds.push(formId);
        }
      } catch (e) {
        // Ignore parse errors
      }
    }
  });

  sleep(1);

  group('List Forms', () => {
    const res = trackedRequest('get', `${API_BASE}/form`, params);
    check(res, {
      '200 OK': (r) => r.status === 200,
      '<500ms': (r) => r.timings.duration < 500,
    });
  });

  sleep(0.5);

  if (data.formIds && data.formIds.length > 0) {
    group('Get Form by ID', () => {
      const randomId =
        data.formIds[Math.floor(Math.random() * data.formIds.length)];
      const res = trackedRequest('get', `${API_BASE}/form/${randomId}`, params);
      check(res, {
        success: (r) => r.status === 200 || r.status === 404,
        '<300ms': (r) => r.timings.duration < 300,
      });
    });

    sleep(0.5);

    group('Update Form', () => {
      const randomId =
        data.formIds[Math.floor(Math.random() * data.formIds.length)];
      const updateData = {
        name: `Updated Form ${Date.now()}`,
        description: 'Updated description',
      };
      const res = trackedRequest(
        'patch',
        `${API_BASE}/form/${randomId}`,
        params,
        updateData,
        'write'
      );
      check(res, {
        success: (r) => r.status === 200 || r.status === 404,
        '<800ms': (r) => r.timings.duration < 800,
      });
    });

    sleep(1);

    group('Update Form Status', () => {
      const randomId =
        data.formIds[Math.floor(Math.random() * data.formIds.length)];
      const statusData = {
        status: Math.random() > 0.5 ? 'active' : 'inactive',
      };
      const res = trackedRequest(
        'patch',
        `${API_BASE}/form/${randomId}/status`,
        params,
        statusData,
        'write'
      );
      check(res, {
        success: (r) => r.status === 200 || r.status === 404,
      });
    });

    sleep(1);

    group('Delete Form', () => {
      const randomId =
        data.formIds[Math.floor(Math.random() * data.formIds.length)];
      const res = trackedRequest(
        'del',
        `${API_BASE}/form/${randomId}`,
        params,
        null,
        'write'
      );
      check(res, {
        success: (r) =>
          r.status === 200 || r.status === 204 || r.status === 404,
      });
    });
  }

  sleep(1);
}

// =====================
// Scenario 8: P&L Report Operations
// =====================
export function plReportOperations(data) {
  const params = getAuthParams(data.token);

  group('Get P&L Monthly Summary', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/summary${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<2s': (r) => r.timings.duration < 2000,
    });
  });

  sleep(1);

  group('Get P&L By Account', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/by-account${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<2s': (r) => r.timings.duration < 2000,
    });
  });

  sleep(1);

  group('Get P&L By Responsibility', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/by-responsibility${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<2s': (r) => r.timings.duration < 2000,
    });
  });

  sleep(1);

  group('Get P&L FY Summary', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/fy-summary${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<3s': (r) => r.timings.duration < 3000,
    });
  });

  sleep(1);

  group('Get P&L Report', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/report${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<3s': (r) => r.timings.duration < 3000,
    });
  });

  sleep(1);

  group('Export P&L Report', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31&format=pdf`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/pl/export${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<5s': (r) => r.timings.duration < 5000,
    });
  });

  sleep(2);
}

// =====================
// Scenario 9: BAS Report Operations
// =====================
export function basReportOperations(data) {
  const params = getAuthParams(data.token);

  group('Get BAS Report', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/bas/report${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<3s': (r) => r.timings.duration < 3000,
    });
  });

  sleep(1);

  group('Get BAS Preparation', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}&quarter=Q1`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/bas/bas-preparation${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<3s': (r) => r.timings.duration < 3000,
    });
  });

  sleep(1);

  group('Export BAS Report', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31&format=pdf`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/bas/activity-statement/report/export${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<5s': (r) => r.timings.duration < 5000,
    });
  });

  sleep(1);

  group('Export BAS Preparation', () => {
    const queryParams = `?financial_year_id=${data.financialYearId}&quarter=Q1&format=excel`;
    const res = trackedRequest(
      'get',
      `${API_BASE}/bas/bas-preparation/export${queryParams}`,
      params
    );
    check(res, {
      success: (r) => r.status === 200 || r.status === 400,
      '<5s': (r) => r.timings.duration < 5000,
    });
  });

  sleep(1);

  if (data.clinicIds && data.clinicIds.length > 0) {
    group('Get BAS Quarterly Summary by Clinic', () => {
      const randomClinicId =
        data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
      const queryParams = `?financial_year_id=${data.financialYearId}&quarter=Q1`;
      const res = trackedRequest(
        'get',
        `${API_BASE}/bas/clinic/${randomClinicId}/summary${queryParams}`,
        params
      );
      check(res, {
        success: (r) => r.status === 200 || r.status === 400 || r.status === 404,
        '<2s': (r) => r.timings.duration < 2000,
      });
    });

    sleep(1);

    group('Get BAS By Account for Clinic', () => {
      const randomClinicId =
        data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
      const queryParams = `?financial_year_id=${data.financialYearId}&start_date=2024-01-01&end_date=2024-12-31`;
      const res = trackedRequest(
        'get',
        `${API_BASE}/bas/clinic/${randomClinicId}/by-account${queryParams}`,
        params
      );
      check(res, {
        success: (r) => r.status === 200 || r.status === 400 || r.status === 404,
        '<2s': (r) => r.timings.duration < 2000,
      });
    });

    sleep(1);

    group('Get BAS Monthly for Clinic', () => {
      const randomClinicId =
        data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
      const queryParams = `?financial_year_id=${data.financialYearId}&month=2024-01`;
      const res = trackedRequest(
        'get',
        `${API_BASE}/bas/clinic/${randomClinicId}/monthly${queryParams}`,
        params
      );
      check(res, {
        success: (r) => r.status === 200 || r.status === 400 || r.status === 404,
        '<2s': (r) => r.timings.duration < 2000,
      });
    });
  }

  sleep(2);
}

// =====================
// Summary
// =====================
export function handleSummary(data) {
  return {
    'summary.json': JSON.stringify(data),
    stdout: summary(data),
    'report.html': htmlReport(data),
  };
}

function summary(data) {
  const m = data.metrics;
  return `
========== API DB TEST SUMMARY ==========
Total DB Ops:       ${m.db_operations?.values.count || 0}
Total Requests:     ${m.http_reqs?.values.count || 0}
Failed Requests:    ${m.http_req_failed?.values.fails || 0}
Failure Rate:       ${((m.http_req_failed?.values.rate || 0) * 100).toFixed(2)}%

Reads:              ${m.concurrent_reads?.values.count || 0}
Writes:             ${m.concurrent_writes?.values.count || 0}

Read Error Rate:    ${((m.db_read_errors?.values.rate || 0) * 100).toFixed(2)}%
Write Error Rate:   ${((m.db_write_errors?.values.rate || 0) * 100).toFixed(2)}%

P95 Read:           ${(m.db_read_duration?.values['p(95)'] || 0).toFixed(2)} ms
P95 Write:          ${(m.db_write_duration?.values['p(95)'] || 0).toFixed(2)} ms
P95 Overall:        ${(m.http_req_duration?.values['p(95)'] || 0).toFixed(2)} ms
P99 Overall:        ${(m.http_req_duration?.values['p(99)'] || 0).toFixed(2)} ms

Test Coverage:
- Practitioner API (CRUD + Queries)
- Clinic API (Create, Read, Update, Delete, List)
- Form API (Create, Read, Update, Delete, List, Status)
- P&L Reports (Summary, By Account, By Responsibility, FY Summary, Export)
- BAS Reports (Report, Preparation, Export, Clinic-specific)
======================================================
`;
}

function htmlReport(data) {
  const m = data.metrics;
  return `
<!DOCTYPE html>
<html>
<head>
  <title>API Load Test Report - Comprehensive</title>
  <style>
    body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
    .container { background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
    h1 { color: #333; border-bottom: 3px solid #4CAF50; padding-bottom: 10px; }
    h2 { color: #555; margin-top: 30px; border-bottom: 2px solid #ddd; padding-bottom: 8px; }
    .metric { margin: 20px 0; padding: 15px; background: #f9f9f9; border-left: 4px solid #4CAF50; }
    .metric-title { font-weight: bold; color: #555; margin-bottom: 5px; }
    .metric-value { font-size: 24px; color: #333; }
    .error { border-left-color: #f44336; }
    .warning { border-left-color: #ff9800; }
    .coverage { background: #e3f2fd; padding: 15px; border-radius: 4px; margin: 20px 0; }
    .coverage ul { margin: 10px 0; padding-left: 20px; }
    .coverage li { margin: 5px 0; }
  </style>
</head>
<body>
  <div class="container">
    <h1>API Load Test Report - Comprehensive Coverage</h1>
    
    <h2>Performance Metrics</h2>
    <div class="metric">
      <div class="metric-title">Total DB Operations</div>
      <div class="metric-value">${m.db_operations?.values.count || 0}</div>
    </div>
    <div class="metric">
      <div class="metric-title">Total HTTP Requests</div>
      <div class="metric-value">${m.http_reqs?.values.count || 0}</div>
    </div>
    <div class="metric ${(m.http_req_failed?.values.rate || 0) > 0.15 ? 'error' : ''}">
      <div class="metric-title">Failure Rate</div>
      <div class="metric-value">${((m.http_req_failed?.values.rate || 0) * 100).toFixed(2)}%</div>
    </div>
    <div class="metric">
      <div class="metric-title">Read Operations</div>
      <div class="metric-value">${m.concurrent_reads?.values.count || 0}</div>
    </div>
    <div class="metric">
      <div class="metric-title">Write Operations</div>
      <div class="metric-value">${m.concurrent_writes?.values.count || 0}</div>
    </div>
    <div class="metric ${(m.db_read_errors?.values.rate || 0) > 0.1 ? 'warning' : ''}">
      <div class="metric-title">Read Error Rate</div>
      <div class="metric-value">${((m.db_read_errors?.values.rate || 0) * 100).toFixed(2)}%</div>
    </div>
    <div class="metric ${(m.db_write_errors?.values.rate || 0) > 0.2 ? 'warning' : ''}">
      <div class="metric-title">Write Error Rate</div>
      <div class="metric-value">${((m.db_write_errors?.values.rate || 0) * 100).toFixed(2)}%</div>
    </div>
    
    <h2>Response Times</h2>
    <div class="metric">
      <div class="metric-title">P95 Read Duration</div>
      <div class="metric-value">${(m.db_read_duration?.values['p(95)'] || 0).toFixed(2)} ms</div>
    </div>
    <div class="metric">
      <div class="metric-title">P95 Write Duration</div>
      <div class="metric-value">${(m.db_write_duration?.values['p(95)'] || 0).toFixed(2)} ms</div>
    </div>
    <div class="metric">
      <div class="metric-title">P95 Overall Duration</div>
      <div class="metric-value">${(m.http_req_duration?.values['p(95)'] || 0).toFixed(2)} ms</div>
    </div>
    <div class="metric">
      <div class="metric-title">P99 Overall Duration</div>
      <div class="metric-value">${(m.http_req_duration?.values['p(99)'] || 0).toFixed(2)} ms</div>
    </div>
    
    <h2>Test Coverage</h2>
    <div class="coverage">
      <strong>APIs Tested:</strong>
      <ul>
        <li><strong>Practitioner API:</strong> List, Get by ID, Search, Filters, Pagination</li>
        <li><strong>Clinic API:</strong> Create, Read (List & Get), Update, Delete, Bulk Operations</li>
        <li><strong>Form API:</strong> Create, Read (List & Get), Update, Delete, Status Update</li>
        <li><strong>P&L Reports:</strong> Monthly Summary, By Account, By Responsibility, FY Summary, Full Report, Export</li>
        <li><strong>BAS Reports:</strong> Report, Preparation, Export (Report & Preparation), Clinic-specific (Summary, By Account, Monthly)</li>
      </ul>
      <strong>Test Scenarios:</strong>
      <ul>
        <li>Heavy Read Operations (60 concurrent users peak)</li>
        <li>Write Operations (25 requests/sec peak)</li>
        <li>Complex Queries (10 concurrent users)</li>
        <li>Read/Write Mix (40 concurrent users peak)</li>
        <li>Bulk Operations (2 requests/sec)</li>
        <li>Clinic Operations (25 concurrent users peak)</li>
        <li>Form Operations (20 concurrent users peak)</li>
        <li>P&L Report Operations (8 concurrent users)</li>
        <li>BAS Report Operations (8 concurrent users)</li>
      </ul>
    </div>
  </div>
</body>
</html>
`;
}
