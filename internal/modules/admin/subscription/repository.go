package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("subscription not found")

type Repository interface {
	Create(ctx context.Context, s *Subscription) (*Subscription, error)
	GetByID(ctx context.Context, id int) (*Subscription, error)
	FindByName(ctx context.Context, name string) (*Subscription, error)
	List(ctx context.Context, f common.Filter) ([]*Subscription, error)
	Count(ctx context.Context, f common.Filter) (int, error)
	Update(ctx context.Context, s *Subscription) (*Subscription, error)
	Delete(ctx context.Context, id int) error

	// Permission management
	ListPermissions(ctx context.Context, subscriptionID int) ([]*SubscriptionPermission, error)
	UpdatePermission(ctx context.Context, subscriptionID int, key string, req *RqUpdatePermission) (*SubscriptionPermission, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, s *Subscription) (*Subscription, error) {
	query := `
		INSERT INTO tbl_subscription (name, description, price, duration_days, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, description, price, duration_days, is_active, created_at, updated_at, deleted_at
	`
	var out Subscription
	if err := r.db.QueryRowxContext(ctx, query,
		s.Name, s.Description, s.Price, s.DurationDays, s.IsActive, s.CreatedAt, s.UpdatedAt,
	).StructScan(&out); err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}
	return &out, nil
}

func (r *repository) GetByID(ctx context.Context, id int) (*Subscription, error) {
	query := `
		SELECT id, name, description, price, duration_days, is_active, created_at, updated_at, deleted_at
		FROM tbl_subscription
		WHERE id = $1 AND deleted_at IS NULL
	`
	var s Subscription
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&s); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	return &s, nil
}

func (r *repository) FindByName(ctx context.Context, name string) (*Subscription, error) {
	query := `
		SELECT id, name, description, price, duration_days, is_active, created_at, updated_at, deleted_at
		FROM tbl_subscription
		WHERE name = $1 AND deleted_at IS NULL
	`
	var s Subscription
	if err := r.db.QueryRowxContext(ctx, query, name).StructScan(&s); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find subscription by name: %w", err)
	}
	return &s, nil
}

var subscriptionColumns = map[string]string{
	"id":         "id",
	"name":       "name",
	"price":      "price",
	"created_at": "created_at",
}

var subscriptionSearchColumns = []string{"name", "description"}

func (r *repository) List(ctx context.Context, f common.Filter) ([]*Subscription, error) {
	base := `
		SELECT id, name, description, price, duration_days, is_active, created_at, updated_at, deleted_at
		FROM tbl_subscription
		WHERE deleted_at IS NULL
	`

	query, filterArgs := common.BuildQuery(base, f, subscriptionColumns, subscriptionSearchColumns, false)
	query = r.db.Rebind(query)

	var list []*Subscription
	if err := r.db.SelectContext(ctx, &list, query, filterArgs...); err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	return list, nil
}

func (r *repository) Update(ctx context.Context, s *Subscription) (*Subscription, error) {
	query := `
		UPDATE tbl_subscription
		SET name = $2, description = $3, price = $4, duration_days = $5, is_active = $6, updated_at = $7
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, name, description, price, duration_days, is_active, created_at, updated_at, deleted_at
	`
	var out Subscription
	if err := r.db.QueryRowxContext(ctx, query,
		s.ID, s.Name, s.Description, s.Price, s.DurationDays, s.IsActive, s.UpdatedAt,
	).StructScan(&out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update subscription: %w", err)
	}
	return &out, nil
}

func (r *repository) Delete(ctx context.Context, id int) error {
	query := `UPDATE tbl_subscription SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) ListPermissions(ctx context.Context, subscriptionID int) ([]*SubscriptionPermission, error) {
	const q = `
		SELECT sp.id, sp.subscription_id, sp.permission_id, pp.key, sp.is_enabled, sp.usage_limit
		FROM tbl_subscription_permission sp
		JOIN tbl_plan_permission pp ON pp.id = sp.permission_id
		WHERE sp.subscription_id = $1
		  AND sp.deleted_at IS NULL
		  AND pp.deleted_at IS NULL
		ORDER BY pp.key
	`
	var list []*SubscriptionPermission
	if err := r.db.SelectContext(ctx, &list, q, subscriptionID); err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	return list, nil
}

func (r *repository) UpdatePermission(ctx context.Context, subscriptionID int, key string, req *RqUpdatePermission) (*SubscriptionPermission, error) {
	const q = `
		UPDATE tbl_subscription_permission sp
		SET
			usage_limit = COALESCE($3, sp.usage_limit),
			is_enabled  = COALESCE($4, sp.is_enabled),
			updated_at  = now()
		FROM tbl_plan_permission pp
		WHERE sp.permission_id = pp.id
		  AND sp.subscription_id = $1
		  AND pp.key = $2
		  AND sp.deleted_at IS NULL
		RETURNING sp.id, sp.subscription_id, sp.permission_id, pp.key, sp.is_enabled, sp.usage_limit
	`
	var out SubscriptionPermission
	if err := r.db.QueryRowxContext(ctx, q, subscriptionID, key, req.UsageLimit, req.IsEnabled).StructScan(&out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update permission: %w", err)
	}
	return &out, nil
}

func (r *repository) Count(ctx context.Context, f common.Filter) (int, error) {
	base := `FROM tbl_subscription WHERE deleted_at IS NULL `
	query, filterArgs := common.BuildQuery(base, f, subscriptionColumns, subscriptionSearchColumns, true)
	query = r.db.Rebind(query)

	var count int
	if err := r.db.GetContext(ctx, &count, query, filterArgs...); err != nil {
		return 0, fmt.Errorf("count subscriptions: %w", err)
	}
	return count, nil
}
