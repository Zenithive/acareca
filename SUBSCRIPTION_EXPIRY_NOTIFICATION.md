# Subscription Expiry Notification System

This document describes the implementation of the automated subscription expiry notification system for practitioners.

## Overview

The system automatically monitors practitioner subscriptions and:
1. **Marks expired subscriptions** - Changes status from `ACTIVE` to `EXPIRED` when the end date has passed
2. **Sends warning notifications** - Alerts practitioners 7 days and 1 day before expiry
3. **Sends expiry notifications** - Notifies practitioners when their subscription has expired

## Architecture

### 1. New Event Types

Added to `internal/shared/util/constant.go`:

**Event Types:**
- `EventSubscriptionExpiring` - Triggered when a subscription is about to expire
- `EventSubscriptionExpired` - Triggered when a subscription has expired

**Notification Event Type:**
- `EventSubscriptionExpiryAlert` - Maps expiry events to user notification preferences

### 2. Repository Methods

Added to `internal/modules/business/subscription/repository.go`:

**New Methods:**
- `ListExpiringSubscriptions(ctx, daysBeforeExpiry)` - Returns active subscriptions expiring within N days
- `ListExpiredSubscriptions(ctx)` - Returns subscriptions past their end_date but still marked ACTIVE
- `MarkAsExpired(ctx, id)` - Updates a subscription status to EXPIRED

### 3. Expiry Worker

Created `internal/modules/business/subscription/expiry_worker.go`:

**Configuration:**
- Runs every **6 hours**
- Checks for subscriptions expiring in **7 days** and **1 day**
- Automatically marks expired subscriptions

**Workflow:**
1. Find all subscriptions where `end_date < NOW()` and `status = 'ACTIVE'`
2. Update status to `EXPIRED`
3. Send expiry notification to practitioner
4. Find subscriptions expiring in 7 days
5. Send warning notification (only once at exactly 7 days before)
6. Find subscriptions expiring in 1 day
7. Send warning notification (only once at exactly 1 day before)

**Smart Deduplication:**
- Notifications are only sent when `days_remaining` exactly matches the threshold
- Prevents duplicate notifications if the worker runs multiple times per day

### 4. Notification Messages

**7-Day Warning:**
```
Title: ⚠️ Your Subscription Expires in 7 Days
Body: Your subscription will expire on [DATE]. Please renew to avoid any service interruption.
```

**1-Day Warning:**
```
Title: ⚠️ Your Subscription Expires Tomorrow
Body: Your subscription will expire on [DATE]. Please renew to continue using all features.
```

**Expired:**
```
Title: ❌ Your Subscription Has Expired
Body: Your subscription expired on [DATE]. Please renew to regain access to all features.
```

**Notification Metadata:**
- `subscription_id` - The ID of the subscription
- `end_date` - ISO 8601 formatted expiry date
- `days_remaining` - Days until expiry (for warning notifications)
- `status` - "expired" (for expiry notifications)

### 5. Database Migration

Created `migrations/20260701131549_add_subscription_expiry_indexes.sql`:

**Added:**
- Unique constraint on `stripe_subscription_id` for upsert logic
- Index on `(status, end_date)` for efficient expiry queries
- Index on `(practitioner_id, status, start_date, end_date)` for active subscription lookups

### 6. Integration

**Route Registration** (`route/route.go`):
- Creates the expiry worker with subscription repository and notification publisher
- Returns worker to main.go for lifecycle management

**Main Application** (`cmd/api/main.go`):
- Starts the expiry worker in a goroutine with a cancellable context
- Properly shuts down the worker on application termination

## Event Mapping

The system maps internal events to user notification preferences:

```go
EventSubscriptionExpiring  → EventSubscriptionExpiryAlert
EventSubscriptionExpired   → EventSubscriptionExpiryAlert
```

Practitioners can control whether they receive these notifications through their notification preferences by enabling/disabling `EventSubscriptionExpiryAlert`.

## Notification Channels

Notifications are delivered through channels based on user preferences:
- **In-App** - Real-time WebSocket notifications and stored for later retrieval
- **Email** - Email notifications (if configured in preferences)
- **Push** - Push notifications (if configured in preferences)

## Testing the System

### Manual Testing

1. **Create a test subscription expiring soon:**
```sql
INSERT INTO tbl_practitioner_subscription (
    practitioner_id, 
    subscription_id, 
    start_date, 
    end_date, 
    status
) VALUES (
    '<practitioner-uuid>',
    1,
    NOW(),
    NOW() + INTERVAL '1 day',
    'ACTIVE'
);
```

2. **Wait for worker to run** (or restart the application to trigger immediately)

3. **Check logs for:**
```
✅ Subscription expiry worker started
Found X subscriptions expiring in 1 day(s)
✅ Sent expiry warning for subscription X (expires in 1 day(s))
```

4. **Verify notification in database:**
```sql
SELECT * FROM tbl_notification 
WHERE event_type = 'subscription.expiring' 
ORDER BY created_at DESC LIMIT 5;
```

### Simulating Expired Subscriptions

```sql
-- Create already expired subscription
INSERT INTO tbl_practitioner_subscription (
    practitioner_id, 
    subscription_id, 
    start_date, 
    end_date, 
    status
) VALUES (
    '<practitioner-uuid>',
    1,
    NOW() - INTERVAL '30 days',
    NOW() - INTERVAL '1 day',
    'ACTIVE'
);
```

After the next worker run, this subscription should be marked as `EXPIRED` and the practitioner should receive an expiry notification.

## Configuration

The worker behavior can be adjusted by modifying constants in `expiry_worker.go`:

```go
const (
    // How often the worker runs
    expiryCheckInterval = 6 * time.Hour
    
    // Warning thresholds
    expiryWarning7Days = 7
    expiryWarning1Day  = 1
)
```

**Recommendations:**
- Keep the check interval ≤ 6 hours to ensure timely notifications
- Don't reduce interval below 1 hour to avoid unnecessary database load
- Consider adding more warning thresholds (e.g., 30 days, 3 days) if needed

## User Notification Preferences

Practitioners can manage subscription expiry notifications through the preferences API:

**Get Preferences:**
```
GET /api/v1/notification/preferences
```

**Update Preference:**
```
PUT /api/v1/notification/preference
{
  "event_type": "subscription.expiry.alert",
  "channels": ["in_app", "email"]
}
```

## Monitoring

Monitor the worker through application logs:

**Successful Operation:**
```
✅ Subscription expiry worker started
Found 2 subscriptions expiring in 7 day(s)
✅ Sent expiry warning for subscription 123 (expires in 7 day(s))
✅ Sent expiry warning for subscription 124 (expires in 7 day(s))
Found 1 expired subscriptions to process
✅ Marked subscription 125 as EXPIRED for practitioner abc-123
```

**Errors to Watch For:**
```
ERROR: Failed to mark expired subscriptions: [error details]
ERROR: Failed to notify 7-day expiring subscriptions: [error details]
ERROR: Failed to send expiry notification for subscription X: [error details]
```

## Future Enhancements

Consider implementing:

1. **Grace Period** - Allow X days of access after expiry before hard cutoff
2. **Email Templates** - Rich HTML emails with renewal links
3. **SMS Notifications** - For critical expiry warnings
4. **Admin Dashboard** - View expiring subscriptions across all practitioners
5. **Auto-renewal Reminders** - For subscriptions with auto-renewal enabled
6. **Configurable Thresholds** - Let admins configure warning intervals
7. **Subscription Pause** - Allow practitioners to pause subscriptions

## Related Files

- `internal/modules/business/subscription/expiry_worker.go` - Worker implementation
- `internal/modules/business/subscription/repository.go` - Database queries
- `internal/shared/util/constant.go` - Event type definitions
- `internal/shared/util/util.go` - Event type mapping
- `cmd/api/main.go` - Worker startup
- `route/route.go` - Dependency wiring
- `migrations/20260701131549_add_subscription_expiry_indexes.sql` - Database schema
