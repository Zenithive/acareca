package contact

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Create(ctx context.Context, contact Contact) (Contact, error)
	Update(ctx context.Context, contact Contact) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (Contact, error)
	List(ctx context.Context) ([]Contact, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// Create implements [Repository].
func (r *repository) Create(ctx context.Context, contact Contact) (Contact, error) {
	query := `INSERT INTO tbl_clinic_contact (id, clinic_id, fname, lname, phone, email, website, abn, note) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`
	err := r.db.QueryRowContext(ctx, query, contact.ID, contact.ClinicId, contact.Fname, contact.Lname, contact.Phone, contact.Email, contact.Website, contact.ABN, contact.Note).Scan(&contact.ID)
	if err != nil {
		return Contact{}, err
	}

	for _, address := range contact.Address {
		query := `INSERT INTO tbl_clinic_contact_address (id, contact_id, address_line1, address_line2, city, state, postal_code, country, is_primary) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
		_, err := r.db.ExecContext(ctx, query, address.Id, contact.ID, address.AddressLine1, address.AddressLine2, address.City, address.State, address.PostalCode, address.Country, address.IsPrimary)
		if err != nil {
			return Contact{}, err
		}
	}

	return contact, nil
}

// Delete implements [Repository].
func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tbl_clinic_contact WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}

// Get implements [Repository].
func (r *repository) Get(ctx context.Context, id uuid.UUID) (Contact, error) {
	var contact Contact
	query := `SELECT id, clinic_id, fname, lname, phone, email, website, abn, note FROM tbl_clinic_contact WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&contact.ID, &contact.ClinicId, &contact.Fname, &contact.Lname, &contact.Phone, &contact.Email, &contact.Website, &contact.ABN, &contact.Note)
	if err != nil {
		return Contact{}, err
	}

	return contact, nil
}

// List implements [Repository].
func (r *repository) List(ctx context.Context) ([]Contact, error) {
	var contacts []Contact
	query := `SELECT id, clinic_id, fname, lname, phone, email, website, abn, note FROM tbl_clinic_contact`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var contact Contact
		err := rows.Scan(&contact.ID, &contact.ClinicId, &contact.Fname, &contact.Lname, &contact.Phone, &contact.Email, &contact.Website, &contact.ABN, &contact.Note)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	return contacts, nil
}

// Update implements [Repository].
func (r *repository) Update(ctx context.Context, contact Contact) error {
	query := `UPDATE tbl_clinic_contact SET clinic_id = $1, fname = $2, lname = $3, phone = $4, email = $5, website = $6, abn = $7, note = $8 WHERE id = $9`
	_, err := r.db.ExecContext(ctx, query, contact.ClinicId, contact.Fname, contact.Lname, contact.Phone, contact.Email, contact.Website, contact.ABN, contact.Note, contact.ID)
	if err != nil {
		return err
	}

	return nil
}
