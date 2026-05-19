package contact

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Create(ctx context.Context, contact Contact) (Contact, error)
	Update(ctx context.Context, contact Contact) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (Contact, error)
	List(ctx context.Context) ([]Contact, error)

	DeleteAddressByID(ctx context.Context, Id uuid.UUID) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// Create implements [Repository].
func (r *repository) Create(ctx context.Context, contact Contact) (Contact, error) {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO tbl_clinic_contact (id, clinic_id, fname, lname, phone, email, website, abn, note) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		contact.ID, contact.ClinicId, contact.Fname, contact.Lname, contact.Phone, contact.Email, contact.Website, contact.ABN, contact.Note,
	).Scan(&contact.ID)
	if err != nil {
		return Contact{}, err
	}

	if len(contact.Address) > 0 {
		if err := r.insertAddresses(ctx, contact.ID, contact.Address); err != nil {
			return Contact{}, err
		}
	}

	return contact, nil
}

// insertAddresses inserts multiple addresses for a contact
func (r *repository) insertAddresses(ctx context.Context, contactID uuid.UUID, addresses []*Address) error {
	query := `INSERT INTO tbl_clinic_contact_address (id, contact_id, address_line1, address_line2, city, state, postal_code, country, is_primary) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	for _, addr := range addresses {
		if _, err := r.db.ExecContext(ctx, query, addr.Id, contactID, addr.AddressLine1, addr.AddressLine2, addr.City, addr.State, addr.PostalCode, addr.Country, addr.IsPrimary); err != nil {
			return err
		}
	}
	return nil
}

// Delete implements [Repository].
func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tbl_clinic_contact WHERE id = $1`, id)
	return err
}

// Get implements [Repository].
func (r *repository) Get(ctx context.Context, id uuid.UUID) (Contact, error) {
	var contact Contact
	err := r.db.QueryRowContext(ctx,
		`SELECT id, clinic_id, fname, lname, phone, email, website, abn, note FROM tbl_clinic_contact WHERE id = $1`,
		id,
	).Scan(&contact.ID, &contact.ClinicId, &contact.Fname, &contact.Lname, &contact.Phone, &contact.Email, &contact.Website, &contact.ABN, &contact.Note)
	if err != nil {
		return Contact{}, err
	}

	contact.Address, err = r.getAddressesByContactID(ctx, contact.ID)
	return contact, err
}

// getAddressesByContactID fetches all addresses for a given contact ID
func (r *repository) getAddressesByContactID(ctx context.Context, contactID uuid.UUID) ([]*Address, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, address_line1, address_line2, city, state, postal_code, country, is_primary 
		FROM tbl_clinic_contact_address WHERE contact_id = $1`,
		contactID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addresses []*Address
	for rows.Next() {
		var addr Address
		if err := rows.Scan(&addr.Id, &addr.AddressLine1, &addr.AddressLine2, &addr.City, &addr.State, &addr.PostalCode, &addr.Country, &addr.IsPrimary); err != nil {
			return nil, err
		}
		addresses = append(addresses, &addr)
	}
	return addresses, rows.Err()
}

// List implements [Repository].
func (r *repository) List(ctx context.Context) ([]Contact, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, clinic_id, fname, lname, phone, email, website, abn, note FROM tbl_clinic_contact`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var contact Contact
		if err := rows.Scan(&contact.ID, &contact.ClinicId, &contact.Fname, &contact.Lname, &contact.Phone, &contact.Email, &contact.Website, &contact.ABN, &contact.Note); err != nil {
			return nil, err
		}

		contact.Address, err = r.getAddressesByContactID(ctx, contact.ID)
		if err != nil {
			return nil, err
		}

		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

// Update implements [Repository].
func (r *repository) Update(ctx context.Context, contact Contact) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Update contact
		if _, err := tx.ExecContext(ctx,
			`UPDATE tbl_clinic_contact SET clinic_id = $1, fname = $2, lname = $3, phone = $4, email = $5, website = $6, abn = $7, note = $8 WHERE id = $9`,
			contact.ClinicId, contact.Fname, contact.Lname, contact.Phone, contact.Email, contact.Website, contact.ABN, contact.Note, contact.ID,
		); err != nil {
			return err
		}

		// Get existing address IDs
		rows, err := tx.QueryContext(ctx, `SELECT id FROM tbl_clinic_contact_address WHERE contact_id = $1`, contact.ID)
		if err != nil {
			return err
		}

		existingIDs := make(map[uuid.UUID]bool)
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			existingIDs[id] = true
		}
		rows.Close()

		// Upsert addresses and track updated IDs
		updatedIDs := make(map[uuid.UUID]bool)
		for _, addr := range contact.Address {
			updatedIDs[addr.Id] = true

			if existingIDs[addr.Id] {
				// Update existing
				if _, err := tx.ExecContext(ctx,
					`UPDATE tbl_clinic_contact_address SET address_line1 = $1, address_line2 = $2, city = $3, state = $4, postal_code = $5, country = $6, is_primary = $7, updated_at = CURRENT_TIMESTAMP WHERE id = $8`,
					addr.AddressLine1, addr.AddressLine2, addr.City, addr.State, addr.PostalCode, addr.Country, addr.IsPrimary, addr.Id,
				); err != nil {
					return err
				}
			} else {
				// Insert new
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO tbl_clinic_contact_address (id, contact_id, address_line1, address_line2, city, state, postal_code, country, is_primary) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
					addr.Id, contact.ID, addr.AddressLine1, addr.AddressLine2, addr.City, addr.State, addr.PostalCode, addr.Country, addr.IsPrimary,
				); err != nil {
					return err
				}
			}
		}

		// Delete removed addresses
		for existingID := range existingIDs {
			if !updatedIDs[existingID] {
				if _, err := tx.ExecContext(ctx, `DELETE FROM tbl_clinic_contact_address WHERE id = $1`, existingID); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// DeleteAddressByID implements [Repository].
func (r *repository) DeleteAddressByID(ctx context.Context, Id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tbl_clinic_contact_address WHERE id = $1`, Id)
	if err != nil {
		return err
	}
	return nil
}
