# Audit Trail Implementation Checklist

## ✅ Completed

- [x] Task 1: Database migration created
- [x] Task 2: Audit module core (model, repository, service)
- [x] Task 3: Shared audit utilities (constants, context)
- [x] Task 4: Audit middleware created
- [x] Task 5: Auth module integration
- [x] Task 6: Subscription module integration
- [x] Task 7: Query endpoints (handler, routes)

## ⏳ TODO - Integration Steps

### Step 1: Run Migration
- [ ] Run migration: `make migrate-up` or your migration command
- [ ] Verify table created: `\d tbl_audit_log` in psql
- [ ] Verify permissions: Try UPDATE/DELETE (should fail)

### Step 2: Wire Up Services in Main App

Find your main application file (likely `cmd/api/main.go` or `route/route.go`) and add:

- [ ] Import audit package
- [ ] Initialize audit repository: `auditRepo := audit.NewRepository(db)`
- [ ] Initialize audit service: `auditSvc := audit.NewService(auditRepo)`
- [ ] Update auth service: Add `auditSvc` parameter
- [ ] Update subscription service: Add `auditSvc` parameter
- [ ] Register audit routes in admin group

### Step 3: Add Middleware

In your router setup:

- [ ] Add `middleware.AuditContext()` after Auth middleware
- [ ] Verify middleware order: ClientInfo → Auth → AuditContext

### Step 4: Test Basic Functionality

- [ ] Test user registration → Check audit log created
- [ ] Test user login → Check audit log created
- [ ] Test create subscription → Check audit log created
- [ ] Test update subscription → Check before/after states
- [ ] Test delete subscription → Check before state captured

### Step 5: Test Query Endpoints

- [ ] GET /admin/audit (list all)
- [ ] GET /admin/audit?module=auth
- [ ] GET /admin/audit?action=user.registered
- [ ] GET /admin/audit/{id} (get specific)
- [ ] Test pagination (limit, offset)
- [ ] Test date range filtering

### Step 6: Verify Append-Only

- [ ] Try UPDATE in database (should fail)
- [ ] Try DELETE in database (should fail)
- [ ] Verify INSERT works

## 📝 Future Enhancements (Optional)

- [ ] Add audit logging to clinic module
- [ ] Add audit logging to practitioner module
- [ ] Add audit logging to COA module
- [ ] Add audit logging to forms module
- [ ] Add audit logging to financial year module
- [ ] Add logout audit logging
- [ ] Add password reset audit logging
- [ ] Add session revocation audit logging
- [ ] Add export audit logs feature
- [ ] Add audit log retention policy
- [ ] Add audit log archival

## 🐛 Troubleshooting

If audit logs are not appearing:

1. Check middleware is registered: `middleware.AuditContext()`
2. Check service is initialized with auditSvc parameter
3. Check migration ran successfully
4. Check database permissions
5. Check application logs for errors
6. Verify async worker is running (check service initialization)

## 📚 Documentation

- Implementation details: `AUDIT_IMPLEMENTATION.md`
- Manual test plan: See AUDIT_IMPLEMENTATION.md
- Code examples: See AUDIT_IMPLEMENTATION.md
