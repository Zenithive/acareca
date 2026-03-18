// Package limits provides subscription-based resource limit checking.
// It is a read-only package — it never writes to the database.
// Inject limits.Service into any service that needs to enforce plan limits.
package limits

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Permission keys — use these constants everywhere instead of raw strings.
const (
	KeyClinicCreate      = "clinic.create"
	KeyFormCreate        = "form.create"
	KeyTransactionCreate = "transaction.create"
	KeyUserInvite        = "user.invite"
)

// ErrLimitReached is returned when the practitioner has hit their plan limit.
var ErrLimitReached = errors.New("subscription limit reached")

// ErrNoActiveSubscription is returned when no active subscription is found.
var ErrNoActiveSubscription = errors.New("no active subscription found")

// Service checks whether a practitioner is allowed to perform a resource action.
type Service interface {
	// Check returns nil if the action is allowed, ErrLimitReached if the limit
	// is hit, or ErrNoActiveSubscription if the practitioner has no active plan.
	Check(ctx context.Context, practitionerID uuid.UUID, key string) error
}

type service struct {
	repo repository
}

// NewService constructs a limits.Service backed by the given DB connection.
func NewService(db *sqlx.DB) Service {
	return &service{repo: &repoImpl{db: db}}
}

func (s *service) Check(ctx context.Context, practitionerID uuid.UUID, key string) error {
	limit, err := s.repo.getLimit(ctx, practitionerID, key)
	if err != nil {
		return fmt.Errorf("limits: get limit for %q: %w", key, err)
	}

	// -1 means unlimited — fast path, no count query needed
	if limit == -1 {
		return nil
	}

	count, err := s.repo.countUsage(ctx, practitionerID, key)
	if err != nil {
		return fmt.Errorf("limits: count usage for %q: %w", key, err)
	}

	if count >= limit {
		return fmt.Errorf("%w: %s (used %d / %d)", ErrLimitReached, key, count, limit)
	}

	return nil
}
