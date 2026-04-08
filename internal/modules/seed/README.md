# Seed API Module

HTTP endpoints for seeding and cleaning up test data.

## Endpoints

### POST /api/v1/seed

Seed test data (clinics, forms, and formulas) for a practitioner.

**Request Body:**
```json
{
  "practitioner_id": "uuid-string (optional)",
  "num_clinics": 10,
  "num_forms": 5,
  "num_fields": 6,
  "verbose": false
}
```

**Parameters:**
- `practitioner_id` (optional): Specific practitioner UUID. If not provided, uses/creates default practitioner
- `num_clinics` (required): Number of clinics to create (1-100)
- `num_forms` (required): Number of forms per clinic (1-50)
- `num_fields` (optional): Number of fields per form (1-10, default: 6)
- `verbose` (optional): Include detailed information in response (default: false)

**Response:**
```json
{
  "status": "success",
  "message": "Data seeded successfully",
  "data": {
    "practitioner_id": "uuid-string",
    "clinics_created": 10,
    "forms_created": 50,
    "fields_created": 300,
    "formulas_created": 0,
    "duration": "1.234s",
    "details": []  // Only if verbose=true
  }
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/v1/seed \
  -H "Content-Type: application/json" \
  -d '{
    "num_clinics": 5,
    "num_forms": 3,
    "num_fields": 6,
    "verbose": true
  }'
```

**With specific practitioner:**
```bash
curl -X POST http://localhost:8080/api/v1/seed \
  -H "Content-Type: application/json" \
  -d '{
    "practitioner_id": "aa9ec38a-27e4-4a3e-82b3-e3061a8363c8",
    "num_clinics": 2,
    "num_forms": 2
  }'
```

### POST /api/v1/seed/cleanup

Delete all clinics and forms for a practitioner (preserves chart of accounts).

**Request Body:**
```json
{
  "practitioner_id": "uuid-string (required)"
}
```

**Parameters:**
- `practitioner_id` (required): Practitioner UUID whose data should be deleted

**Response:**
```json
{
  "status": "success",
  "message": "Data cleaned up successfully",
  "data": {
    "practitioner_id": "uuid-string",
    "clinics_deleted": 10,
    "forms_deleted": 50,
    "fields_deleted": 300,
    "addresses_deleted": 10,
    "contacts_deleted": 20,
    "form_versions_deleted": 50,
    "duration": "0.456s"
  }
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/v1/seed/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "practitioner_id": "aa9ec38a-27e4-4a3e-82b3-e3061a8363c8"
  }'
```

## Features

- ✅ Create realistic test data using gofakeit
- ✅ Unique COA IDs per form field
- ✅ Automatic practitioner creation if needed
- ✅ Validation of practitioner existence
- ✅ Transactional operations (all or nothing)
- ✅ Performance timing
- ✅ Detailed response with verbose mode
- ✅ Preserves chart of accounts during cleanup

## Use Cases

### Development
```bash
# Quick test data
curl -X POST http://localhost:8080/api/v1/seed \
  -H "Content-Type: application/json" \
  -d '{"num_clinics": 2, "num_forms": 2}'
```

### Testing
```bash
# Create test data
curl -X POST http://localhost:8080/api/v1/seed \
  -H "Content-Type: application/json" \
  -d '{"practitioner_id": "test-uuid", "num_clinics": 5, "num_forms": 3}'

# Run tests...

# Cleanup
curl -X POST http://localhost:8080/api/v1/seed/cleanup \
  -H "Content-Type: application/json" \
  -d '{"practitioner_id": "test-uuid"}'
```

### Performance Testing
```bash
# Large dataset
curl -X POST http://localhost:8080/api/v1/seed \
  -H "Content-Type: application/json" \
  -d '{"num_clinics": 50, "num_forms": 10, "verbose": true}'
```

## Notes

- Chart of accounts are preserved during cleanup for reuse
- All operations are transactional
- Verbose mode includes detailed clinic and form information
- Formulas are not created in the API version (use scripts for full formula support)
- Maximum limits: 100 clinics, 50 forms per clinic, 10 fields per form

## Error Responses

**Invalid Request:**
```json
{
  "status": "error",
  "message": "Invalid request body",
  "error": "validation error details"
}
```

**Practitioner Not Found:**
```json
{
  "status": "error",
  "message": "Failed to seed data",
  "error": "practitioner with ID xxx does not exist"
}
```

**Database Error:**
```json
{
  "status": "error",
  "message": "Failed to seed data",
  "error": "database error details"
}
```
