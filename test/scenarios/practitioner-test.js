import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { htmlReport } from 'https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js';

// =====================
// Custom Metrics
// =====================
const practitionerReadErrors = new Rate('practitioner_read_errors');
const practitionerWriteErrors = new Rate('practitioner_write_errors');
const practitionerReadDuration = new Trend('practitioner_read_duration');
const practitionerWriteDuration = new Trend('practitioner_write_duration');
const practitionerOperations = new Counter('practitioner_operations');

// Track endpoint-specific metrics
const endpointMetrics = {
  list: new Trend('endpoint_list_duration'),
  getById: new Trend('endpoint_get_by_id_duration'),
  search: new Trend('endpoint_search_duration'),
  pagination: new Trend('endpoint_pagination_duration'),
  updateLockDate: new Trend('endpoint_update_lock_date_duration'),
  createClinic: new Trend('endpoint_create_clinic_duration'),
  updateClinic: new Trend('endpoint_update_clinic_duration'),
  listClinic: new Trend('endpoint_list_clinic_duration'),
  createForm: new Trend('endpoint_create_form_duration'),
  listForm: new Trend('endpoint_list_form_duration'),
  getForm: new Trend('endpoint_get_form_duration'),
};

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
        { duration: '20s', target: 10 },
        { duration: '40s', target: 15 },
        { duration: '20s', target: 0 },
      ],
      exec: 'listPractitioners',
    },
    practitioner_get: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '20s', target: 10 },
        { duration: '40s', target: 20 },
        { duration: '20s', target: 0 },
      ],
      exec: 'getPractitioner',
      startTime: '10s',
    },
    practitioner_search: {
      executor: 'constant-vus',
      vus: 8,
      duration: '1m',
      exec: 'searchPractitioners',
      startTime: '20s',
    },
    practitioner_update_lock_date: {
      executor: 'ramping-vus',
      startVUs: 2,
      stages: [
        { duration: '15s', target: 5 },
        { duration: '30s', target: 8 },
        { duration: '15s', target: 2 },
      ],
      exec: 'updateLockDate',
      startTime: '30s',
    },
    clinic_create: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '15s', target: 5 },
        { duration: '30s', target: 10 },
        { duration: '15s', target: 0 },
      ],
      exec: 'createClinic',
      startTime: '20s',
    },
    clinic_list: {
      executor: 'constant-vus',
      vus: 10,
      duration: '1m',
      exec: 'listClinics',
      startTime: '25s',
    },
    clinic_update: {
      executor: 'constant-vus',
      vus: 5,
      duration: '45s',
      exec: 'updateClinic',
      startTime: '40s',
    },
    form_create: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '15s', target: 3 },
        { duration: '30s', target: 5 },
        { duration: '15s', target: 0 },
      ],
      exec: 'createForm',
      startTime: '30s',
    },
    form_list: {
      executor: 'constant-vus',
      vus: 8,
      duration: '1m',
      exec: 'listForms',
      startTime: '35s',
    },
    form_get: {
      executor: 'constant-vus',
      vus: 6,
      duration: '45s',
      exec: 'getForm',
      startTime: '40s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<3000'],
    http_req_failed: ['rate<0.20'],
    practitioner_read_errors: ['rate<0.10'],
    practitioner_write_errors: ['rate<0.20'],
    practitioner_read_duration: ['p(95)<1000'],
    practitioner_write_duration: ['p(95)<2000'],
    practitioner_operations: ['count>200'],
  },
};

// =====================
// Constants
// =====================
const BASE_URL = __ENV.BASE_URL || 'https://acareca-backend-staging.up.railway.app';
const API_BASE = `${BASE_URL}/api/v1`;
const FINANCIAL_YEAR_ID = 'a381d86c-0f2b-4aac-9b6c-7289c3618762';

// =====================
// Setup (LOGIN ONCE)
// =====================
// =====================
// Setup (LOGIN ONCE & CREATE INITIAL DATA)
// =====================
export function setup() {
  const loginPayload = JSON.stringify({
    email: __ENV.TEST_EMAIL || 'loadtest@yopmail.com',
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

  const headers = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };

  console.log('✅ Login successful');

  // Fetch practitioner list
  const practitionersRes = http.get(`${API_BASE}/practitioner`, { headers });
  let practitionerIds = [];
  if (practitionersRes.status === 200) {
    const practitionersBody = JSON.parse(practitionersRes.body);
    const practitioners = practitionersBody?.data?.items || [];
    practitionerIds = practitioners.map((p) => p.id).filter(Boolean);
    console.log(`✅ Found ${practitionerIds.length} practitioner IDs`);
  }

  // Fetch existing clinics
  const clinicsRes = http.get(`${API_BASE}/clinic`, { headers });
  let clinicIds = [];
  if (clinicsRes.status === 200) {
    const clinicsBody = JSON.parse(clinicsRes.body);
    const clinics = clinicsBody?.data?.items || [];
    clinicIds = clinics.map((c) => c.id).filter(Boolean);
    console.log(`✅ Found ${clinicIds.length} existing clinic IDs`);
  }

  // Create 3 test clinics if none exist
  if (clinicIds.length === 0) {
    console.log('📝 Creating initial test clinics...');
    for (let i = 0; i < 3; i++) {
      const clinicPayload = {
        name: `Setup Test Clinic ${i + 1}`,
        abn: `${Math.floor(Math.random() * 90000000000) + 10000000000}`,
        is_active: true,
        addresses: [{
          address: `${i + 1} Test Street`,
          city: 'Sydney',
          state: 'NSW',
          postcode: '2000',
          is_primary: true,
        }],
        contacts: [{
          contact_type: 'PHONE',
          value: `+61400000${i}00`,
          is_primary: true,
        }],
      };

      const createRes = http.post(
        `${API_BASE}/clinic`,
        JSON.stringify(clinicPayload),
        { headers }
      );

      if (createRes.status === 201) {
        const createBody = JSON.parse(createRes.body);
        if (createBody?.data?.id) {
          clinicIds.push(createBody.data.id);
        }
      }
    }
    console.log(`✅ Created ${clinicIds.length} test clinics`);
  }

  // Fetch existing forms
  const formsRes = http.get(`${API_BASE}/form`, { headers });
  let formIds = [];
  if (formsRes.status === 200) {
    const formsBody = JSON.parse(formsRes.body);
    const forms = formsBody?.data?.items || [];
    formIds = forms.map((f) => f.id).filter(Boolean);
    console.log(`✅ Found ${formIds.length} existing form IDs`);
  }

  // Create 2 test forms if none exist and we have clinics
  if (formIds.length === 0 && clinicIds.length > 0) {
    console.log('📝 Creating initial test forms...');
    for (let i = 0; i < 2; i++) {
      const formPayload = {
        clinic_id: clinicIds[i % clinicIds.length],
        name: `Setup Test Form ${i + 1}`,
        method: 'SERVICE_FEE',
        clinic_share: 60,
        owner_share: 40,
        status: 'DRAFT',
        fields: [
          {
            key: 'A',
            slug: `setup_field_a_${i}`,
            label: 'Test Field A',
            coa_id: 'a6f919e6-94b6-43e2-9b01-0eeb2267631b',
            section_type: 'COLLECTION',
            payment_responsibility: 'OWNER',
            tax_type: '',
            is_formula: false,
            sort_order: 1,
            is_computed: false,
          },
        ],
        formulas: [],
      };

      const createRes = http.post(
        `${API_BASE}/form`,
        JSON.stringify(formPayload),
        { headers }
      );

      if (createRes.status === 201) {
        const createBody = JSON.parse(createRes.body);
        if (createBody?.data?.form?.id) {
          formIds.push(createBody.data.form.id);
        }
      } else {
        console.log(`⚠️ Failed to create setup form ${i + 1}: ${createRes.status}`);
      }
    }
    console.log(`✅ Created ${formIds.length} test forms`);
  }

  console.log(`\n📊 Setup Complete:`);
  console.log(`   - Practitioners: ${practitionerIds.length}`);
  console.log(`   - Clinics: ${clinicIds.length}`);
  console.log(`   - Forms: ${formIds.length}\n`);

  return { token, practitionerIds, clinicIds, formIds };
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

function trackedRequest(method, url, headers, body = null, endpointType = null) {
  const start = Date.now();
  
  // Map 'delete' to 'del' for k6 http module
  const httpMethod = method.toLowerCase() === 'delete' ? 'del' : method.toLowerCase();
  
  const res = body
    ? http[httpMethod](url, JSON.stringify(body), { headers })
    : http[httpMethod](url, { headers });
  const duration = Date.now() - start;

  practitionerOperations.add(1);
  
  // Track read vs write operations
  const isWrite = ['post', 'put', 'patch', 'delete', 'del'].includes(method.toLowerCase());
  if (isWrite) {
    practitionerWriteDuration.add(duration);
  } else {
    practitionerReadDuration.add(duration);
  }
  
  // Track endpoint-specific metrics
  if (endpointType && endpointMetrics[endpointType]) {
    endpointMetrics[endpointType].add(duration);
  }

  return res;
}

// =====================
// Scenario 1: List Practitioners
// =====================
export function listPractitioners(data) {
  const headers = getAuthHeaders(data.token);

  group('List All Practitioners', () => {
    const res = trackedRequest('get', `${API_BASE}/practitioner`, headers, null, 'list');
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
      headers,
      null,
      'getById'
    );
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 500ms': (r) => r.timings.duration < 500,
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
      headers,
      null,
      'search'
    );
    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
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
// Scenario 4: Update Lock Date
// =====================
export function updateLockDate(data) {
  const headers = getAuthHeaders(data.token);

  group('Update Practitioner Lock Date', () => {
    const today = new Date();
    const lockDate = new Date(
      today.getFullYear(),
      Math.floor(Math.random() * 12),
      Math.floor(Math.random() * 28) + 1
    );
    const lockDateStr = lockDate.toISOString().split('T')[0];

    const updatePayload = {
      financial_year_id: FINANCIAL_YEAR_ID,
      lock_date: lockDateStr,
    };

    const res = trackedRequest(
      'patch',
      `${API_BASE}/practitioner/lock-date`,
      headers,
      updatePayload,
      'updateLockDate'
    );

    const ok = check(res, {
      'status is 200 or 400': (r) => r.status === 200 || r.status === 400,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
    practitionerWriteErrors.add(ok ? 0 : 1);
  });

  sleep(1);
}

// =====================
// Scenario 5: Create Clinic
// =====================
export function createClinic(data) {
  const headers = getAuthHeaders(data.token);

  group('Create Clinic', () => {
    const timestamp = Date.now();
    const randomNum = Math.floor(Math.random() * 10000);
    
    const payload = {
      name: `LoadTest Clinic ${timestamp}_${randomNum}`,
      abn: `${Math.floor(Math.random() * 90000000000) + 10000000000}`,
      description: 'Load test clinic',
      is_active: true,
      addresses: [
        {
          address: '123 Test Street',
          city: 'Sydney',
          state: 'NSW',
          postcode: '2000',
          is_primary: true,
        },
      ],
      contacts: [
        {
          contact_type: 'PHONE',
          value: `+61${Math.floor(Math.random() * 900000000) + 100000000}`,
          label: 'Main',
          is_primary: true,
        },
      ],
    };

    const res = trackedRequest(
      'post',
      `${API_BASE}/clinic`,
      headers,
      payload,
      'createClinic'
    );

    const ok = check(res, {
      'status is 201': (r) => r.status === 201,
      'response time < 2500ms': (r) => r.timings.duration < 2500,
      'has clinic data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data?.id !== undefined;
        } catch {
          return false;
        }
      },
    });
    practitionerWriteErrors.add(ok ? 0 : 1);
  });

  sleep(1);
}

// =====================
// Scenario 6: List Clinics
// =====================
export function listClinics(data) {
  const headers = getAuthHeaders(data.token);

  group('List Clinics', () => {
    const res = trackedRequest('get', `${API_BASE}/clinic`, headers, null, 'listClinic');
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
    practitionerReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);
}

// =====================
// Scenario 7: Update Clinic
// =====================
export function updateClinic(data) {
  const headers = getAuthHeaders(data.token);

  if (!data.clinicIds || data.clinicIds.length === 0) {
    console.log('No clinic IDs available, skipping update test');
    sleep(1);
    return;
  }

  group('Update Clinic', () => {
    const randomId = data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
    
    const updatePayload = {
      name: `Updated Clinic ${Date.now()}`,
      description: 'Updated during load test',
      is_active: true,
    };

    const res = trackedRequest(
      'put',
      `${API_BASE}/clinic/${randomId}`,
      headers,
      updatePayload,
      'updateClinic'
    );

    const ok = check(res, {
      'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
      'response time < 2000ms': (r) => r.timings.duration < 2000,
    });
    practitionerWriteErrors.add(ok ? 0 : 1);
  });

  sleep(1);
}

// =====================
// Scenario 9: Create Form
// =====================
export function createForm(data) {
  const headers = getAuthHeaders(data.token);

  if (!data.clinicIds || data.clinicIds.length === 0) {
    console.log('No clinic IDs available, skipping form create test');
    sleep(1);
    return;
  }

  group('Create Form', () => {
    const timestamp = Date.now();
    const randomNum = Math.floor(Math.random() * 10000);
    const clinicId = data.clinicIds[Math.floor(Math.random() * data.clinicIds.length)];
    
    // Use hardcoded COA IDs from working payload
    const payload = {
      clinic_id: clinicId,
      name: `Service Agreement (60%/40%) - ${timestamp}`,
      method: 'SERVICE_FEE',
      clinic_share: 60,
      owner_share: 40,
      status: 'PUBLISHED',
      fields: [
        {
          key: 'A',
          slug: `total_patient_fees_collected_gst_free_${randomNum}`,
          label: 'Total Patient Fees Collected (GST Free)',
          coa_id: 'a6f919e6-94b6-43e2-9b01-0eeb2267631b',
          section_type: 'COLLECTION',
          payment_responsibility: 'OWNER',
          tax_type: '',
          is_formula: false,
          sort_order: 1,
          is_computed: false,
        },
        {
          key: 'B',
          slug: `lab_fees_net_after_deducting_gst_${randomNum}`,
          label: 'Lab Fees (net after deducting GST)',
          coa_id: '1ceffddf-fd79-4309-9397-c433468594e6',
          section_type: 'COST',
          payment_responsibility: 'CLINIC',
          tax_type: '',
          is_formula: false,
          sort_order: 2,
          is_computed: false,
        },
        {
          key: 'C',
          slug: `net_patient_fees_${randomNum}`,
          label: 'Net Patient Fees',
          is_computed: true,
          sort_order: 3,
        },
        {
          key: 'D',
          slug: `total_sf_fee_${randomNum}`,
          label: 'Total S&F Fee',
          is_computed: true,
          sort_order: 4,
          coa_id: '4547e1bd-f181-45c5-9b44-7d3049172bf2',
          section_type: '',
          tax_type: 'EXCLUSIVE',
          is_taxable: true,
        },
        {
          key: 'E',
          slug: `amount_remitted_to_owner_${randomNum}`,
          label: 'Amount Remitted to Owner',
          is_computed: true,
          sort_order: 5,
          coa_id: '276c38f3-6274-4c68-9460-3165f081638e',
          tax_type: '',
        },
      ],
      formulas: [
        {
          field_key: 'C',
          name: 'Net Patient Fees',
          expression: {
            type: 'operator',
            op: '-',
            left: {
              type: 'field',
              key: 'A',
            },
            right: {
              type: 'field',
              key: 'B',
            },
          },
        },
        {
          field_key: 'D',
          name: 'Total S&F Fee',
          expression: {
            type: 'operator',
            op: '*',
            left: {
              type: 'field',
              key: 'C',
            },
            right: {
              type: 'constant',
              value: 0.6,
            },
          },
        },
        {
          field_key: 'E',
          name: 'Amount Remitted to Owner',
          expression: {
            type: 'operator',
            op: '-',
            left: {
              type: 'field',
              key: 'C',
            },
            right: {
              type: 'field',
              key: 'D',
            },
          },
        },
      ],
    };

    const res = trackedRequest(
      'post',
      `${API_BASE}/form`,
      headers,
      payload,
      'createForm'
    );

    const ok = check(res, {
      'status is 201': (r) => r.status === 201,
      'response time < 4000ms': (r) => r.timings.duration < 4000,
      'has form data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data?.form?.id !== undefined;
        } catch {
          return false;
        }
      },
    });
    
    if (!ok) {
      console.log(`❌ Form create failed: Status ${res.status}`);
      if (res.body) {
        try {
          const errorBody = JSON.parse(res.body);
          console.log(`Error details: ${JSON.stringify(errorBody, null, 2)}`);
        } catch {
          console.log(`Response body: ${res.body.substring(0, 500)}`);
        }
      }
    }
    
    practitionerWriteErrors.add(ok ? 0 : 1);
  });

  sleep(1);
}

// =====================
// Scenario 10: List Forms
// =====================
export function listForms(data) {
  const headers = getAuthHeaders(data.token);

  group('List Forms', () => {
    const res = trackedRequest('get', `${API_BASE}/form`, headers, null, 'listForm');
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
    practitionerReadErrors.add(ok ? 0 : 1);
  });

  sleep(1);
}

// =====================
// Scenario 11: Get Form by ID
// =====================
export function getForm(data) {
  const headers = getAuthHeaders(data.token);

  if (!data.formIds || data.formIds.length === 0) {
    console.log('No form IDs available, skipping get form test');
    sleep(1);
    return;
  }

  group('Get Form by ID', () => {
    const randomFormId = data.formIds[Math.floor(Math.random() * data.formIds.length)];
    
    const res = trackedRequest(
      'get',
      `${API_BASE}/form/${randomFormId}`,
      headers,
      null,
      'getForm'
    );

    const ok = check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
      'has form data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body?.data?.form?.id !== undefined;
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
// Summary
// =====================
export function handleSummary(data) {
  return {
    'practitioner-report.html': htmlReport(data, { 
      title: 'Practitioner API Load Test Report',
      description: 'Performance testing results for Practitioner, Clinic, and Form endpoints'
    }),
    'practitioner-summary.json': JSON.stringify(data, null, 2),
    stdout: textSummary(data),
  };
}

function textSummary(data) {
  const m = data.metrics;
  
  // Extract endpoint-specific metrics
  const listP95 = m.endpoint_list_duration?.values['p(95)'] || 0;
  const getByIdP95 = m.endpoint_get_by_id_duration?.values['p(95)'] || 0;
  const searchP95 = m.endpoint_search_duration?.values['p(95)'] || 0;
  const updateLockDateP95 = m.endpoint_update_lock_date_duration?.values['p(95)'] || 0;
  
  const createClinicP95 = m.endpoint_create_clinic_duration?.values['p(95)'] || 0;
  const updateClinicP95 = m.endpoint_update_clinic_duration?.values['p(95)'] || 0;
  const listClinicP95 = m.endpoint_list_clinic_duration?.values['p(95)'] || 0;
  
  const createFormP95 = m.endpoint_create_form_duration?.values['p(95)'] || 0;
  const listFormP95 = m.endpoint_list_form_duration?.values['p(95)'] || 0;
  const getFormP95 = m.endpoint_get_form_duration?.values['p(95)'] || 0;
  
  return `
========== PRACTITIONER TEST SUMMARY ==========
Total Operations:     ${m.practitioner_operations?.values.count || 0}
Total Requests:       ${m.http_reqs?.values.count || 0}
Failed Requests:      ${m.http_req_failed?.values.fails || 0}
Failure Rate:         ${((m.http_req_failed?.values.rate || 0) * 100).toFixed(2)}%

Read Error Rate:      ${((m.practitioner_read_errors?.values.rate || 0) * 100).toFixed(2)}%
Write Error Rate:     ${((m.practitioner_write_errors?.values.rate || 0) * 100).toFixed(2)}%
P95 Read Duration:    ${(m.practitioner_read_duration?.values['p(95)'] || 0).toFixed(2)} ms
P95 Write Duration:   ${(m.practitioner_write_duration?.values['p(95)'] || 0).toFixed(2)} ms
P99 Read Duration:    ${(m.practitioner_read_duration?.values['p(99)'] || 0).toFixed(2)} ms
P99 Write Duration:   ${(m.practitioner_write_duration?.values['p(99)'] || 0).toFixed(2)} ms

Endpoint Performance (P95):
PRACTITIONER Operations:
  GET    /practitioner                  ${listP95.toFixed(2)} ms
  GET    /practitioner/:id              ${getByIdP95.toFixed(2)} ms
  GET    /practitioner?search=...       ${searchP95.toFixed(2)} ms
  PATCH  /practitioner/lock-date        ${updateLockDateP95.toFixed(2)} ms

CLINIC Operations:
  POST   /clinic                        ${createClinicP95.toFixed(2)} ms
  GET    /clinic                        ${listClinicP95.toFixed(2)} ms
  PUT    /clinic/:id                    ${updateClinicP95.toFixed(2)} ms

FORM Operations:
  POST   /form                          ${createFormP95.toFixed(2)} ms
  GET    /form                          ${listFormP95.toFixed(2)} ms
  GET    /form/:id                      ${getFormP95.toFixed(2)} ms

Test Coverage:
✓ List Practitioners
✓ Get Practitioner by ID
✓ Search Practitioners
✓ Update Lock Date (FY: ${FINANCIAL_YEAR_ID})
✓ Create Clinic
✓ List Clinics
✓ Update Clinic
✓ Create Form (with fields & formulas)
✓ List Forms
✓ Get Form by ID

HTML Report: practitioner-report.html
================================================
`;
}
