package events

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Save(ctx context.Context, e SharedEvent) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Save(ctx context.Context, e SharedEvent) error {
	query := `
		INSERT INTO tbl_shared_events (
			id, practitioner_id, accountant_id, actor_id, actor_name, 
			actor_type, event_type, entity_type, entity_id, description, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := r.db.ExecContext(ctx, query,
		e.ID, e.PractitionerID, e.AccountantID, e.ActorID, e.ActorName,
		e.ActorType, e.EventType, e.EntityType, e.EntityID, e.Description, e.Metadata, e.CreatedAt,
	)
	return err
}
