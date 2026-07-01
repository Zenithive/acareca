# Subscription Access Control — Implementation Reference

This document covers every change made to implement the practitioner `subscription_status` field,
JWT embedding, 402 enforcement in middleware, and automatic expiry → access revocation via the
background worker.

---

## Problem Being Solved

Previously, when a practitioner's subscription expired the system only:
- Marked `tbl_practitioner_subscription.status = 'EXPIRED'`
- Sent an in-app notification

Nothing blocked the practitioner from continuing to use the API. This change closes that gap.

---

## Complete Change List

### 1. Database Migration
**File:** `migrations/20260701120000_add_subscription_status_to_practitioner.sql`

```sql
CREATE TYPE practitioner_subscription_status AS ENUM ('PENDING', 'COMPLETE');

ALTER TABLE tbl_practitioner
    ADD COLUMN IF NOT EXISTS subscription_status
        practitioner_subscription_status NOT NULL DEFAULT 'PENDING';
```

- Adds a new Postgres enum `practitioner_subscription_status` with two values: `PENDING` and `COMPLETE`
- Adds `subscription_status` column to `tbl_practitioner` — defaults to `PENDING` so every new registration starts locked until payment is confirmed
- Rollback drops the column and enum cleanly

---

### 2. Shared Util — JWT Claims + Constants
**File:** `internal/shared/util/util.go`

**New constants:**
```go
const SubscriptionStatusKey = "subscription_status"

const (
    SubscriptionStatusPending  = "PENDING"
    SubscriptionStatusComplete = "COMPLETE"
)
```

**Updated `CustomClaims`** — added `SubscriptionStatus` field:
```go
type CustomClaims struct {
    ID                 string `json:"id"`
    Role               string `json:"role"`
    SubscriptionStatus string `json:"subscription_status,omitempty"`
    jwt.RegisteredClaims
}
```

**Updated `SignToken` signature** — added `subscriptionStatus string` parameter:
```go
// Before
func SignToken(userID, id, role string, ttl time.Duration, jwtSecret string) (string, error)

// After
func SignToken(userID, id, role, subscriptionStatus string, ttl time.Duration, jwtSecret string) (string, error)
```

The `subscriptionStatus` value is embedded in the JWT claims. For non-practitioner roles (admin,
accountant, clinic) pass `""` — the `omitempty` tag keeps it out of the token payload.

---

### 3. Auth Service — Embed Subscription Status in Token
**File:** `internal/modules/auth/service.go` — `issueTokens()`

When issuing tokens for a **practitioner**, the service now reads `subscription_status` from
`tbl_practitioner` and embeds it in both the access token and refresh token:

```go
subscriptionStatus := ""
if user.Role == util.RolePractitioner {
    var status string
    err := s.db.GetContext(ctx, &status,
        `SELECT subscription_status FROM tbl_practitioner WHERE user_id = $1 AND deleted_at IS NULL`,
        user.ID)
    if err != nil {
        status = util.SubscriptionStatusPending // fail-safe: deny access if unresolvable
    }
    subscriptionStatus = status
}

accessToken, err  := util.SignToken(..., subscriptionStatus, 15*time.Hour, ...)
refreshToken, err := util.SignToken(..., subscriptionStatus, 7*24*time.Hour, ...)
```

This means every time a practitioner logs in or the token is refreshed, the current DB value is
captured into the JWT. Existing tokens are not invalidated immediately — the check only fires on
the **next login or token refresh** after expiry is detected.

---

### 4. Middleware — 402 Enforcement
**File:** `internal/shared/middleware/auth.go` — `Auth()`

After parsing and validating the JWT, the middleware sets the subscription status in context and
enforces the 402 block inline:

```go
c.Set(util.UserIDKey, claims.Subject)
c.Set(util.EntityIDKey, entityUUID)
c.Set("role", claims.Role)
c.Set(util.SubscriptionStatusKey, claims.SubscriptionStatus)

// Practitioner with PENDING subscription → 402 Payment Required
if claims.Role == util.RolePractitioner && claims.SubscriptionStatus == util.SubscriptionStatusPending {
    response.Error(c, http.StatusPaymentRequired, errors.New("subscription payment required"))
    c.Abort()
    return
}

c.Next()
```

**Applies to:** every authenticated route — no additional route-level wiring required.

**Does not apply to:** clinic role tokens — clinic `issueTokens` passes `""` for subscription
status so the condition never fires.

---

### 5. Practitioner Business Model
**File:** `internal/modules/business/practitioner/model.go`

Added `SubscriptionStatus string` to all three structs:

| Struct | Field added |
|---|---|
| `Practitioner` | `SubscriptionStatus string \`db:"subscription_status"\`` |
| `PractitionerWithUser` | `SubscriptionStatus string \`db:"subscription_status"\`` |
| `RsPractitioner` | `SubscriptionStatus string \`json:"subscription_status"\`` |

`ToRs()` on both `Practitioner` and `PractitionerWithUser` now maps the field through to the
response.

---

### 6. Practitioner Business Repository
**File:** `internal/modules/business/practitioner/repository.go`

All SELECT queries updated to include `subscription_status` in the column list:

| Method | Change |
|---|---|
| `CreatePractitioner` | `RETURNING` clause now includes `subscription_status` |
| `GetPractitioner` | `SELECT` includes `p.subscription_status` |
| `GetPractitionerByUserID` | `SELECT` includes `subscription_status` |
| `ListPractitioners` | `SELECT` includes `p.subscription_status` |
| `ListPractitionersForAccountant` | `SELECT` includes `p.subscription_status` |

---

### 7. Clinic Auth Service — SignToken Call Fixed
**File:** `internal/modules/clinic/auth/service.go` — `issueTokens()`

Updated the two `SignToken` calls to pass `""` as `subscriptionStatus` to match the new signature.
Clinic tokens are unaffected by the subscription check.

```go
// Before
util.SignToken(clinic.ID.String(), clinicID, roleString, 15*time.Hour, s.cfg.JWTSecret)

// After
util.SignToken(clinic.ID.String(), clinicID, roleString, "", 15*time.Hour, s.cfg.JWTSecret)
```

---

### 8. Subscription Repository — New Method
**File:** `internal/modules/business/subscription/repository.go`

Added to the `Repository` interface and implemented:

```go
MarkPractitionerSubscriptionPending(ctx context.Context, practitionerID uuid.UUID) error
```

Implementation:
```sql
UPDATE tbl_practitioner
SET subscription_status = 'PENDING', updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
```

Called by the expiry worker when a subscription is marked `EXPIRED`.

---

### 9. Expiry Worker — Access Revocation on Expiry
**File:** `internal/modules/business/subscription/expiry_worker.go` — `markExpiredSubscriptions()`

After successfully marking a subscription as `EXPIRED`, the worker now also sets
`subscription_status = 'PENDING'` on the corresponding practitioner:

```go
for _, sub := range expired {
    // 1. Mark subscription row EXPIRED
    if err := w.repo.MarkAsExpired(ctx, sub.ID); err != nil {
        log.Printf("ERROR: ...")
        continue
    } 

    // 2. Set subscription_status = PENDING on tbl_practitioner
    //    → practitioner's NEXT login will embed PENDING in their token
    //    → Auth middleware then returns 402 on all subsequent requests
    if err := w.repo.MarkPractitionerSubscriptionPending(ctx, sub.PractitionerID); err != nil {
        log.Printf("ERROR: ...") // non-fatal, continue to notification
    }

    // 3. Send in-app expiry notification
    w.sendExpiryNotification(ctx, sub)
}
```

The `MarkPractitionerSubscriptionPending` call is **non-fatal** — if it fails, the subscription is
still marked `EXPIRED` and the notification is still sent. The practitioner's access will be
revoked on their next login regardless.

---

## End-to-End Flow

```
Practitioner registers
  → tbl_practitioner.subscription_status = 'PENDING'  (DB default)
  → Trial subscription created in tbl_practitioner_subscription (status = 'ACTIVE')
  → Admin / Stripe marks subscription COMPLETE
      → UPDATE tbl_practitioner SET subscription_status = 'COMPLETE'   ← manual/webhook step

Practitioner logs in
  → issueTokens() reads subscription_status from tbl_practitioner
  → Embeds value in JWT (access + refresh token)
  → Token payload: { role: "PRACTITIONER", subscription_status: "COMPLETE" }

Every authenticated request
  → Auth middleware parses JWT
  → If role=PRACTITIONER AND subscription_status=PENDING → 402, abort
  → Otherwise → c.Next()

Subscription end_date passes (ExpiryWorker runs every 6 hours)
  → ListExpiredSubscriptions(): WHERE status='ACTIVE' AND end_date < NOW()
  → MarkAsExpired(sub.ID):
      UPDATE tbl_practitioner_subscription SET status = 'EXPIRED'
  → MarkPractitionerSubscriptionPending(sub.PractitionerID):
      UPDATE tbl_practitioner SET subscription_status = 'PENDING'
  → sendExpiryNotification(): in-app "❌ Your Subscription Has Expired"

Practitioner's next login after expiry
  → issueTokens() reads subscription_status = 'PENDING'
  → New token embeds PENDING
  → All API calls → 402 Payment Required
```

---

## Status Values Reference

### `tbl_practitioner.subscription_status` (new column)
| Value | Meaning |
|---|---|
| `PENDING` | No active subscription — API access blocked (402) |
| `COMPLETE` | Active subscription — API access allowed |

### `tbl_practitioner_subscription.status` (existing column, unchanged)
| Value | Meaning |
|---|---|
| `ACTIVE` | Subscription live and valid |
| `EXPIRED` | End date passed — set by ExpiryWorker |
| `PAST_DUE` | Stripe payment failed |
| `CANCELLED` | Cancelled by user or Stripe |
| `PAUSED` | Paused |
| `INACTIVE` | Not yet active |

---

## Files Changed

| File | Change |
|---|---|
| `migrations/20260701120000_add_subscription_status_to_practitioner.sql` | New migration — enum + column |
| `internal/shared/util/util.go` | New constants, updated `CustomClaims`, updated `SignToken` signature |
| `internal/shared/middleware/auth.go` | Sets `subscription_status` in context, enforces 402 for PENDING practitioners |
| `internal/modules/auth/service.go` | `issueTokens()` reads and embeds `subscription_status` |
| `internal/modules/clinic/auth/service.go` | Updated `SignToken` calls to pass `""` |
| `internal/modules/business/practitioner/model.go` | Added `SubscriptionStatus` to structs and response |
| `internal/modules/business/practitioner/repository.go` | All SELECT/RETURNING queries include `subscription_status` |
| `internal/modules/business/subscription/repository.go` | New `MarkPractitionerSubscriptionPending()` method |
| `internal/modules/business/subscription/expiry_worker.go` | Calls `MarkPractitionerSubscriptionPending` after marking subscription expired |

---

## What Is NOT Changed

- Route-level middleware — no `RequireActiveSubscription` function exists; the check is inline in `Auth`
- Billing webhook (`billing/webhook.go`) — Stripe `PAST_DUE` does not currently set `subscription_status = PENDING`; only expiry via the worker does
- Existing tokens — not invalidated when expiry is detected; revocation takes effect on next login or token refresh
- Clinic, accountant, admin roles — completely unaffected by this check
