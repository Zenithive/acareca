# Audit Trail Implementation Summary

## Completed Tasks

### ✅ Task 1: Database Foundation
- Created migration `migrations/20260317120000_tbl_audit_log.sql`
- Table with INSERT-only permissions (UPDATE/DELETE revoked)
- Indexes on: practice_id, user_id, created_at, module, action, entity
- Removed trace_id as requested

### ✅ Task 2: Audit Module Core
- `internal/modules/admin/audit/model.go` - Data models
- `internal/modules/admin/audit/repository.go` - Database operations
- `internal/modules/admin/audit/service.go` - Business logic with async logging

### ✅ Task 3: Shared Audit Utilities
- `internal/shared/audit/constants.go` - Action, module, entity constants
- `internal/shared/audit/context.go` - Context helpers for metadata

### ✅ Task 4: Middleware Enhancement
- `internal/shared/middleware/audit.go` - Captures IP, user agent, user ID, practice ID

### ✅ Task 5: Integration - Auth Module
- Updated `internal/modules/auth/service.go`
- Logs: user.registered, user.logged_in (password & OAuth)

### ✅ Task 6: Integration - Subscription Module
- Updated `internal/modules/admin/subscription/service.go`
- Logs: subscription.created, subscription.updated, subscription.deleted

### ✅ Task 7: Query Endpoints
- `internal/modules/admin/audit/handler.go` - HTTP handlers
- `internal/modules/admin/audit/routes.go` - Route registration

## Integration Steps Required

### 1. Run Migration
```bash
# Run the migration to create tbl_audit_log
make migrate-up
# or your migration command
```

### 2. Update Main Application Setup

You need to wire up the audit service in your main application. Here's what needs to be added:

```go
// In cmd/api/main.go or wherever you initialize services

// Initialize audit service
auditRepo := audit.NewRepository(db)
auditSvc := audit.NewService(auditRepo)

// Update auth service initialization
authSvc := auth.NewService(authRepo, cfg, db, practitionerSvc, auditSvc)

// Update subscription service initialization
subscriptionSvc := subscription.NewService(subscriptionRepo, auditSvc)

// Register audit routes
auditHandler := audit.NewHandler(auditSvc)
adminGroup.Group("/audit", auditHandler)
```

### 3. Add Middleware to Router

Add the audit middleware to your router chain (after Auth and ClientInfo):

```go
// In your router setup
router.Use(middleware.ClientInfo())
router.Use(middleware.Auth(cfg))
router.Use(middleware.AuditContext())  // Add this
```

## Manual Test Plan

### Test 1: User Registration
```bash
POST /auth/register
Content-Type: application/json

{
  "email": "test@example.com",
  "password": "SecurePass123",
  "first_name": "Test",
  "last_name": "User"
}

# Then query audit logs
GET /admin/audit?module=auth&action=user.registered
```

**Expected Audit Log:**
- action: "user.registered"
- module: "auth"
- entity_type: "tbl_user"
- after_state: { user data without password }
- ip_address: your IP
- user_agent: Postman/Browser

### Test 2: User Login
```bash
POST /auth/login
Content-Type: application/json

{
  "email": "test@example.com",
  "password": "SecurePass123"
}

# Query audit logs
GET /admin/audit?user_id={user_id}&action=user.logged_in
```

**Expected Audit Log:**
- action: "user.logged_in"
- module: "auth"
- entity_type: "tbl_session"
- after_state: { email }

### Test 3: Create Subscription
```bash
POST /admin/subscriptions
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "Premium Plan",
  "description": "Premium subscription",
  "price": 99.99,
  "duration_days": 30,
  "is_active": true
}

# Query audit logs
GET /admin/audit?module=admin&action=subscription.created
```

**Expected Audit Log:**
- action: "subscription.created"
- module: "admin"
- entity_type: "tbl_subscription"
- user_id: admin user ID
- after_state: { full subscription data }

### Test 4: Update Subscription
```bash
PUT /admin/subscriptions/{id}
Authorization: Bearer {token}
Content-Type: application/json

{
  "price": 149.99
}

# Query audit logs
GET /admin/audit?entity_id={subscription_id}&action=subscription.updated
```

**Expected Audit Log:**
- action: "subscription.updated"
- module: "admin"
- before_state: { "price": 99.99, ... }
- after_state: { "price": 149.99, ... }

### Test 5: Delete Subscription
```bash
DELETE /admin/subscriptions/{id}
Authorization: Bearer {token}

# Query audit logs
GET /admin/audit?action=subscription.deleted
```

**Expected Audit Log:**
- action: "subscription.deleted"
- module: "admin"
- before_state: { full subscription data }
- after_state: null

### Test 6: Query Audit Logs with Filters
```bash
# By module
GET /admin/audit?module=auth&limit=10

# By user
GET /admin/audit?user_id={user_id}

# By practice
GET /admin/audit?practice_id={practice_id}

# By date range
GET /admin/audit?start_date=2026-03-01T00:00:00Z&end_date=2026-03-31T23:59:59Z

# By entity
GET /admin/audit?entity_type=tbl_subscription&entity_id={id}

# Combined filters
GET /admin/audit?module=admin&action=subscription.created&limit=5&offset=0
```

### Test 7: Verify Append-Only (Database Level)
```sql
-- Try to update (should fail)
UPDATE tbl_audit_log SET action = 'modified' WHERE id = '{some_id}';
-- Expected: ERROR: permission denied

-- Try to delete (should fail)
DELETE FROM tbl_audit_log WHERE id = '{some_id}';
-- Expected: ERROR: permission denied

-- Insert should work
INSERT INTO tbl_audit_log (action, module) VALUES ('test.action', 'test');
-- Expected: SUCCESS
```

### Test 8: OAuth Login
```bash
# Initiate Google OAuth
GET /auth/google

# After callback, check audit logs
GET /admin/audit?action=user.logged_in

# For new users via OAuth
GET /admin/audit?action=user.registered
```

**Expected Audit Log:**
- action: "user.registered" or "user.logged_in"
- after_state: { email, provider: "google" }

### Test 9: Concurrent Operations
```bash
# Make 5 simultaneous requests using a tool like Apache Bench or k6
ab -n 5 -c 5 -H "Authorization: Bearer {token}" \
   http://localhost:8080/admin/subscriptions

# Verify all 5 audit logs were created
GET /admin/audit?module=admin&limit=10
```

### Test 10: Get Specific Audit Log
```bash
# Get a specific audit log by ID
GET /admin/audit/{audit_log_id}
```

## Key Features

1. **Append-Only**: Database-level enforcement prevents modifications
2. **Async Logging**: Non-blocking audit logging via goroutine worker
3. **Context Propagation**: Automatic capture of user, practice, IP, user agent
4. **Flexible Querying**: Filter by module, action, user, practice, date range
5. **Before/After States**: Captures state changes for UPDATE operations
6. **Minimal Performance Impact**: Buffered channel with 1000 capacity

## Adding Audit Logging to Other Modules

To add audit logging to any other module:

1. **Import packages:**
```go
import (
    "github.com/iamarpitzala/acareca/internal/modules/admin/audit"
    auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
)
```

2. **Add audit service to struct:**
```go
type service struct {
    repo     Repository
    auditSvc audit.Service
}
```

3. **Update constructor:**
```go
func NewService(repo Repository, auditSvc audit.Service) Service {
    return &service{repo: repo, auditSvc: auditSvc}
}
```

4. **Log operations:**
```go
// After successful operation
meta := auditctx.GetMetadata(ctx)
s.auditSvc.LogAsync(&audit.LogEntry{
    PracticeID:  meta.PracticeID,
    UserID:      meta.UserID,
    Action:      auditctx.ActionYourAction,
    Module:      auditctx.ModuleYourModule,
    EntityType:  strPtr(auditctx.EntityYourEntity),
    EntityID:    &entityID,
    BeforeState: beforeData,  // For updates/deletes
    AfterState:  afterData,   // For creates/updates
    IPAddress:   meta.IPAddress,
    UserAgent:   meta.UserAgent,
})
```

## Constants to Add

When adding new actions, add them to `internal/shared/audit/constants.go`:

```go
const (
    ActionYourAction = "your.action"
    EntityYourEntity = "tbl_your_table"
)
```

## Notes

- Audit failures are logged but don't break main operations
- Channel buffer prevents blocking (drops logs if full)
- NULL user_id for system operations
- NULL practice_id for superadmin operations
- All timestamps in UTC
