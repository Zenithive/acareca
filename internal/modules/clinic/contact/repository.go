package contact

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Create(ctx context.Context, contact Contact) (Contact, error)
	Update(ctx context.Context, contact Contact) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (Contact, error)
	List(ctx context.Context, clinicID uuid.UUID, f common.Filter) ([]Contact, int64, error)
	DeleteAddressByID(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, contact Contact) (Contact, error) {
	err := util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {

		if contact.ID == uuid.Nil {
			contact.ID = uuid.New()
		}

		_, err := tx.ExecContext(ctx,
			`
			INSERT INTO tbl_clinic_contact_person (
				id,
				clinic_id,
				fname,
				lname,
				phone,
				email,
				website,
				abn,
				note
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			`,
			contact.ID,
			contact.ClinicId,
			contact.Fname,
			contact.Lname,
			contact.Phone,
			contact.Email,
			contact.Website,
			contact.ABN,
			contact.Note,
		)
		if err != nil {
			return err
		}

		if len(contact.Address) > 0 {
			if err := r.insertAddressesTx(ctx, tx, contact.ID, contact.Address); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return Contact{}, err
	}

	return contact, nil
}

func (r *repository) insertAddressesTx(
	ctx context.Context,
	tx *sqlx.Tx,
	contactID uuid.UUID,
	addresses []*Address,
) error {

	query := `
	INSERT INTO tbl_clinic_contact_person_address (
		id,
		contact_id,
		address_line1,
		address_line2,
		city,
		state,
		postal_code,
		country,
		is_primary
	)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`

	for _, addr := range addresses {

		if addr.Id == uuid.Nil {
			addr.Id = uuid.New()
		}

		_, err := tx.ExecContext(
			ctx,
			query,
			addr.Id,
			contactID,
			addr.AddressLine1,
			addr.AddressLine2,
			addr.City,
			addr.State,
			addr.PostalCode,
			addr.Country,
			addr.IsPrimary,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {

	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {

		result, err := tx.ExecContext(ctx,
			`
			UPDATE tbl_clinic_contact_person
			SET
				deleted_at = NOW(),
				updated_at = NOW()
			WHERE id = $1
			AND deleted_at IS NULL
			`,
			id,
		)
		if err != nil {
			return err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return errors.New("contact not found")
		}

		_, err = tx.ExecContext(ctx,
			`
			UPDATE tbl_clinic_contact_person_address
			SET
				deleted_at = NOW(),
				updated_at = NOW()
			WHERE contact_id = $1
			AND deleted_at IS NULL
			`,
			id,
		)

		return err
	})
}

func (r *repository) Get(ctx context.Context, id uuid.UUID) (Contact, error) {

	var contact Contact

	err := r.db.QueryRowContext(ctx,
		`
		SELECT
			id,
			clinic_id,
			fname,
			lname,
			COALESCE(phone, ''),
			email,
			COALESCE(website, ''),
			COALESCE(abn, ''),
			COALESCE(note, ''),
			created_at,
			updated_at
		FROM tbl_clinic_contact_person
		WHERE id = $1
		AND deleted_at IS NULL
		`,
		id,
	).Scan(
		&contact.ID,
		&contact.ClinicId,
		&contact.Fname,
		&contact.Lname,
		&contact.Phone,
		&contact.Email,
		&contact.Website,
		&contact.ABN,
		&contact.Note,
		&contact.CreatedAt,
		&contact.UpdatedAt,
	)
	if err != nil {
		return Contact{}, err
	}

	addresses, err := r.getAddressesByContactID(ctx, contact.ID)
	if err != nil {
		return Contact{}, err
	}

	contact.Address = addresses

	return contact, nil
}

func (r *repository) getAddressesByContactID(ctx context.Context, contactID uuid.UUID) ([]*Address, error) {

	rows, err := r.db.QueryContext(ctx,
		`
		SELECT
			id,
			address_line1,
			address_line2,
			city,
			state,
			postal_code,
			country,
			is_primary
		FROM tbl_clinic_contact_person_address
		WHERE contact_id = $1
		AND deleted_at IS NULL
		ORDER BY is_primary DESC, created_at ASC
		`,
		contactID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addresses []*Address

	for rows.Next() {

		var addr Address

		err := rows.Scan(
			&addr.Id,
			&addr.AddressLine1,
			&addr.AddressLine2,
			&addr.City,
			&addr.State,
			&addr.PostalCode,
			&addr.Country,
			&addr.IsPrimary,
		)
		if err != nil {
			return nil, err
		}

		addresses = append(addresses, &addr)
	}

	return addresses, rows.Err()
}

func (r *repository) List(ctx context.Context, clinicID uuid.UUID, f common.Filter) ([]Contact, int64, error) {
	allowedColumns := map[string]string{
		"id":         "id",
		"fname":      "fname",
		"email":      "email",
		"phone":      "phone",
		"abn":        "abn",
		"created_at": "created_at",
	}

	searchCols := []string{"fname", "lname", "email", "phone", "abn"}

	baseQuery := `FROM tbl_clinic_contact_person WHERE deleted_at IS NULL AND clinic_id = ?`
	baseArgs := []interface{}{clinicID}

	countQueryPart, countArgsPart := common.BuildQuery(baseQuery, f, allowedColumns, searchCols, true)
	countArgs := append(baseArgs, countArgsPart...)

	var total int64
	err := r.db.GetContext(ctx, &total, sqlx.Rebind(sqlx.DOLLAR, countQueryPart), countArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("count contacts: %w", err)
	}

	selectQueryBase := `SELECT id, clinic_id, fname, lname, COALESCE(phone, ''), email, COALESCE(website, ''), COALESCE(abn, ''), COALESCE(note, ''), created_at, updated_at ` + baseQuery
	itemsQuery, itemsArgsPart := common.BuildQuery(selectQueryBase, f, allowedColumns, searchCols, false)
	itemsArgs := append(baseArgs, itemsArgsPart...)

	rows, err := r.db.QueryContext(ctx, sqlx.Rebind(sqlx.DOLLAR, itemsQuery), itemsArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("select contacts items: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var contact Contact
		err := rows.Scan(
			&contact.ID,
			&contact.ClinicId,
			&contact.Fname,
			&contact.Lname,
			&contact.Phone,
			&contact.Email,
			&contact.Website,
			&contact.ABN,
			&contact.Note,
			&contact.CreatedAt,
			&contact.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		addresses, err := r.getAddressesByContactID(ctx, contact.ID)
		if err != nil {
			return nil, 0, err
		}
		contact.Address = addresses
		contacts = append(contacts, contact)
	}

	return contacts, total, rows.Err()
}

func (r *repository) Update(ctx context.Context, contact Contact) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {

		result, err := tx.ExecContext(ctx,
			`
			UPDATE tbl_clinic_contact_person
			SET
				clinic_id = $1,
				fname = $2,
				lname = $3,
				phone = $4,
				email = $5,
				website = $6,
				abn = $7,
				note = $8,
				updated_at = NOW()
			WHERE id = $9
			AND deleted_at IS NULL
			`,
			contact.ClinicId,
			contact.Fname,
			contact.Lname,
			contact.Phone,
			contact.Email,
			contact.Website,
			contact.ABN,
			contact.Note,
			contact.ID,
		)
		if err != nil {
			return err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return errors.New("contact not found")
		}

		rows, err := tx.QueryContext(ctx,
			`
			SELECT id
			FROM tbl_clinic_contact_person_address
			WHERE contact_id = $1
			AND deleted_at IS NULL
			`,
			contact.ID,
		)
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

		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		updatedIDs := make(map[uuid.UUID]bool)

		for _, addr := range contact.Address {
			if addr.Id != uuid.Nil {
				updatedIDs[addr.Id] = true
			}
		}

		for existingID := range existingIDs {
			if !updatedIDs[existingID] {
				_, err := tx.ExecContext(ctx,
					`
					UPDATE tbl_clinic_contact_person_address
					SET
						deleted_at = NOW(),
						updated_at = NOW()
					WHERE id = $1
					AND deleted_at IS NULL
					`,
					existingID,
				)
				if err != nil {
					return err
				}
			}
		}

		_, err = tx.ExecContext(ctx,
			`
			UPDATE tbl_clinic_contact_person_address
			SET
				is_primary = FALSE,
				updated_at = NOW()
			WHERE contact_id = $1
			AND deleted_at IS NULL
			`,
			contact.ID,
		)
		if err != nil {
			return err
		}

		for _, addr := range contact.Address {

			if addr.Id == uuid.Nil {
				addr.Id = uuid.New()
			}

			updatedIDs[addr.Id] = true

			if existingIDs[addr.Id] {

				_, err := tx.ExecContext(ctx,
					`
					UPDATE tbl_clinic_contact_person_address
					SET
						address_line1 = $1,
						address_line2 = $2,
						city = $3,
						state = $4,
						postal_code = $5,
						country = $6,
						is_primary = $7,
						updated_at = NOW()
					WHERE id = $8
					AND deleted_at IS NULL
					`,
					addr.AddressLine1,
					addr.AddressLine2,
					addr.City,
					addr.State,
					addr.PostalCode,
					addr.Country,
					addr.IsPrimary,
					addr.Id,
				)
				if err != nil {
					return err
				}

			} else {

				_, err := tx.ExecContext(ctx,
					`
					INSERT INTO tbl_clinic_contact_person_address (
						id,
						contact_id,
						address_line1,
						address_line2,
						city,
						state,
						postal_code,
						country,
						is_primary
					)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
					`,
					addr.Id,
					contact.ID,
					addr.AddressLine1,
					addr.AddressLine2,
					addr.City,
					addr.State,
					addr.PostalCode,
					addr.Country,
					addr.IsPrimary,
				)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func (r *repository) DeleteAddressByID(ctx context.Context, id uuid.UUID) error {

	result, err := r.db.ExecContext(ctx,
		`
		UPDATE tbl_clinic_contact_person_address
		SET
			deleted_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
		AND deleted_at IS NULL
		`,
		id,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("address not found")
	}

	return nil
}
