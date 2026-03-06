package calculation

import (
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	// Calculate(ctx context.Context) (*Result, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// func (r *repository) Calculate(ctx context.Context) (*Result, error) {
// 	query := `
// 		SELECT result
// 		FROM tbl_calculation
// 		WHERE deleted_at IS NULL
// 	`
// 	var result Result
// 	if err := r.db.QueryRowxContext(ctx, query).StructScan(&result); err != nil {
// 		return nil, fmt.Errorf("calculate: %w", err)
// 	}
// 	return &result, nil
// }
