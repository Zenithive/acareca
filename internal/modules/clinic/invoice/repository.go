package invoice

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type IRepository interface {
	Create(ctx context.Context, invoice *Invoice) error
	Update(ctx context.Context, invoice *Invoice) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*Invoice, error)
	List(ctx context.Context) ([]*Invoice, error)
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{
		db: db,
	}
}

// Create implements [IRepository].
func (r *Repository) Create(ctx context.Context, invoice *Invoice) error {
	panic("unimplemented")
}

// Delete implements [IRepository].
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	panic("unimplemented")
}

// Get implements [IRepository].
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	panic("unimplemented")
}

// List implements [IRepository].
func (r *Repository) List(ctx context.Context) ([]*Invoice, error) {
	panic("unimplemented")
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, invoice *Invoice) error {
	panic("unimplemented")
}
