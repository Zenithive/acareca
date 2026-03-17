# Audit Trail Integration Guide

## Exact Changes Needed in `route/route.go`

### 1. Add Import

Add this to the imports section:

```go
import (
    // ... existing imports ...
    "github.com/iamarpitzala/acareca/internal/modules/admin/audit"
)
```

### 2. Initialize Audit Service

Add these lines after `dbConn` initialization (around line 50):

```go
// Initialize audit service
auditRepo := audit.NewRepository(dbConn)
auditSvc := audit.NewService(auditRepo)
```

### 3. Update Auth Service Initialization

Replace this line:
```go
authSvc := auth.NewService(authRepo, cfg, dbConn, practitionerSvc)
```

With:
```go
authSvc := auth.NewService(authRepo, cfg, dbConn, practitionerSvc, auditSvc)
```

### 4. Update Subscription Service Initialization

Replace this line:
```go
subscriptionSvc := subscription.NewService(subscriptionRepo)
```

With:
```go
subscriptionSvc := subscription.NewService(subscriptionRepo, auditSvc)
```

### 5. Add Audit Routes

Add these lines after the subscription routes registration (around line 80):

```go
// Audit routes
auditGroup := adminGroup.Group("/audit")
auditGroup.Use(middleware.Auth(cfg), middleware.RequireSuperadmin(func(ctx context.Context, userID string) (bool, error) {
	id, err := util.ParseUUID(userID)
	if err != nil {
		return false, err
	}
	return superadminCheck(ctx, id)
}))
auditHandler := audit.NewHandler(auditSvc)
audit.RegisterRoutes(auditGroup, auditHandler)
```

### 6. Add Audit Middleware (Optional but Recommended)

Add this line in the middleware chain for authenticated routes. You can add it globally or per route group:

```go
// For global application
v1.Use(middleware.ClientInfo(), middleware.AuditContext())

// OR for specific groups
adminGroup.Use(middleware.ClientInfo(), middleware.Auth(cfg), middleware.AuditContext())
```

## Complete Modified Section

Here's what the relevant section should look like after changes:

```go
func RegisterRoutes(r *gin.Engine, cfg *config.Config) {
	// ... health and swagger routes ...

	v1 := r.Group("/api/v1")
	v1.Use(middleware.ClientInfo()) // Add this if not already present

	dbConn, err := db.DBConn(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Initialize repositories
	authRepo := auth.NewRepository(dbConn)
	subscriptionRepo := subscription.NewRepository(dbConn)
	practitionerRepo := practitioner.NewRepository(dbConn)
	userSubscriptionRepo := userSubscription.NewRepository(dbConn)
	coaRepo := coa.NewRepository(dbConn)

	// Initialize audit service (NEW)
	auditRepo := audit.NewRepository(dbConn)
	auditSvc := audit.NewService(auditRepo)

	// Initialize services
	subscriptionSvc := subscription.NewService(subscriptionRepo, auditSvc) // UPDATED
	userSubscriptionSvc := userSubscription.NewService(userSubscriptionRepo)
	practitionerSvc := practitioner.NewService(practitionerRepo, subscriptionSvc, userSubscriptionSvc, coaRepo)
	authSvc := auth.NewService(authRepo, cfg, dbConn, practitionerSvc, auditSvc) // UPDATED

	// Auth routes
	authHandler := auth.NewHandler(authSvc)
	auth.RegisterRoutes(v1, authHandler)

	// ... calculation routes ...

	// Admin routes
	superadminCheck := func(ctx context.Context, userID uuid.UUID) (bool, error) {
		u, err := authRepo.FindByID(ctx, userID)
		if err != nil {
			return false, err
		}
		return u.IsSuperadmin != nil && *u.IsSuperadmin, nil
	}

	adminGroup := v1.Group("/admin")
	adminGroup.Use(middleware.AuditContext()) // Add audit context (NEW)

	// Subscription routes
	subscriptionGroup := adminGroup.Group("/subscription")
	subscriptionGroup.Use(middleware.Auth(cfg), middleware.RequireSuperadmin(func(ctx context.Context, userID string) (bool, error) {
		id, err := util.ParseUUID(userID)
		if err != nil {
			return false, err
		}
		return superadminCheck(ctx, id)
	}))
	subscriptionHandler := subscription.NewHandler(subscriptionSvc)
	subscription.RegisterRoutes(subscriptionGroup, subscriptionHandler)

	// Audit routes (NEW)
	auditGroup := adminGroup.Group("/audit")
	auditGroup.Use(middleware.Auth(cfg), middleware.RequireSuperadmin(func(ctx context.Context, userID string) (bool, error) {
		id, err := util.ParseUUID(userID)
		if err != nil {
			return false, err
		}
		return superadminCheck(ctx, id)
	}))
	auditHandler := audit.NewHandler(auditSvc)
	audit.RegisterRoutes(auditGroup, auditHandler)

	// ... rest of the routes ...
}
```

## Quick Start Commands

1. **Run migration:**
```bash
# Check your makefile for the correct command
make migrate-up
# or
go run cmd/api/main.go migrate up
```

2. **Update route/route.go** with the changes above

3. **Run the application:**
```bash
make run
# or
go run cmd/api/main.go
```

4. **Test basic functionality:**
```bash
# Register a user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test123!","first_name":"Test","last_name":"User"}'

# Check audit logs (need superadmin token)
curl -X GET http://localhost:8080/api/v1/admin/audit?module=auth \
  -H "Authorization: Bearer YOUR_SUPERADMIN_TOKEN"
```

## Troubleshooting

### Error: "not enough arguments in call to subscription.NewService"
- Make sure you added `auditSvc` parameter: `subscription.NewService(subscriptionRepo, auditSvc)`

### Error: "not enough arguments in call to auth.NewService"
- Make sure you added `auditSvc` parameter: `auth.NewService(authRepo, cfg, dbConn, practitionerSvc, auditSvc)`

### Error: "undefined: audit"
- Make sure you added the import: `"github.com/iamarpitzala/acareca/internal/modules/admin/audit"`

### Audit logs not appearing
- Check middleware is added: `middleware.AuditContext()`
- Check migration ran successfully
- Check application logs for errors

### Permission denied on UPDATE/DELETE
- This is expected! The table is append-only by design
- Only INSERT and SELECT are allowed

## Next Steps

After integration:
1. Run the manual tests from `AUDIT_IMPLEMENTATION.md`
2. Verify audit logs are being created
3. Test query endpoints with different filters
4. Add audit logging to other modules as needed

## Adding Audit to Other Modules

For any other service (clinic, practitioner, etc.):

1. Add `auditSvc audit.Service` to service struct
2. Update `NewService` constructor to accept `auditSvc`
3. Update initialization in `route/route.go`
4. Add audit logging calls after successful operations

Example:
```go
// In clinic service
clinicSvc := clinic.NewService(clinicRepo, auditSvc)
```
