# Audit Trail - Quick Start Guide

## 🚀 Get Started in 3 Steps

### Step 1: Run Migration
```bash
make migrate-up
```

### Step 2: Start Application
```bash
make run
```

### Step 3: Test It
```bash
# Register a user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test123!","first_name":"Test","last_name":"User"}'

# Check audit logs (need superadmin token)
curl http://localhost:8080/api/v1/admin/audit?module=auth \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 📋 What's Logged

| Module | Action | When |
|--------|--------|------|
| Auth | `user.registered` | User signs up |
| Auth | `user.logged_in` | User logs in |
| Admin | `subscription.created` | New subscription |
| Admin | `subscription.updated` | Subscription modified |
| Admin | `subscription.deleted` | Subscription removed |

## 🔍 Query Examples

```bash
# All auth events
GET /api/v1/admin/audit?module=auth

# Specific user's actions
GET /api/v1/admin/audit?user_id={user_id}

# Specific action
GET /api/v1/admin/audit?action=user.registered

# Date range
GET /api/v1/admin/audit?start_date=2026-03-01T00:00:00Z&end_date=2026-03-31T23:59:59Z

# Pagination
GET /api/v1/admin/audit?limit=20&offset=40

# Combined filters
GET /api/v1/admin/audit?module=admin&action=subscription.created&limit=10
```

## 📁 Key Files

| File | Purpose |
|------|---------|
| `migrations/20260317120000_tbl_audit_log.sql` | Database table |
| `internal/modules/admin/audit/` | Audit module |
| `internal/shared/audit/` | Shared utilities |
| `AUDIT_IMPLEMENTATION.md` | Full documentation |
| `INTEGRATION_GUIDE.md` | Integration details |

## ✅ Verification

After starting the app:

1. Register a user
2. Query audit logs
3. Verify log entry exists with:
   - ✓ action: "user.registered"
   - ✓ module: "auth"
   - ✓ ip_address: your IP
   - ✓ user_agent: your client

## 🔧 Troubleshooting

| Issue | Solution |
|-------|----------|
| No audit logs | Check middleware is added |
| Permission denied | Expected! Table is append-only |
| 401 Unauthorized | Need superadmin token |
| Migration failed | Check database connection |

## 📚 Full Documentation

- **Complete Guide**: `AUDIT_IMPLEMENTATION.md`
- **Integration Steps**: `INTEGRATION_GUIDE.md`
- **Checklist**: `AUDIT_CHECKLIST.md`
- **Status**: `IMPLEMENTATION_COMPLETE.md`

## 🎯 Next Steps

1. ✅ Code is ready
2. ⏳ Run migration
3. ⏳ Test endpoints
4. ⏳ Add to other modules (optional)

---

**Ready to go!** Just run the migration and start testing.
