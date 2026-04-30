package setting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("practitioner not found")

type Repository interface {
	Create(ctx context.Context, t *Practitioner) (*Practitioner, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Practitioner, error)
	GetByUserID(ctx context.Context, userID string) (*Practitioner, error)
	List(ctx context.Context, f common.Filter) ([]*Practitioner, error)
	Update(ctx context.Context, t *Practitioner) (*Practitioner, error)
	Delete(ctx context.Context, id uuid.UUID) error

	GetSettingByPractitionerID(ctx context.Context, practitionerID uuid.UUID) (*PractitionerSetting, error)
	UpsertSetting(ctx context.Context, s *PractitionerSetting) (*PractitionerSetting, error)

	Count(ctx context.Context, f common.Filter) (int, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, t *Practitioner) (*Practitioner, error) {
	query := `
		INSERT INTO tbl_practitioner (user_id, abn, verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, abn, verified, created_at, updated_at, deleted_at
	`
	now := time.Now()
	var out Practitioner
	if err := r.db.QueryRowxContext(ctx, query,
		t.UserID, t.ABN, t.Verified, now, now,
	).StructScan(&out); err != nil {
		return nil, fmt.Errorf("create practitioner: %w", err)
	}
	return &out, nil
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Practitioner, error) {
	query := `
		SELECT id, user_id, abn, verified, created_at, updated_at, deleted_at
		FROM tbl_practitioner
		WHERE id = $1 AND deleted_at IS NULL
	`
	var t Practitioner
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&t); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get practitioner: %w", err)
	}
	return &t, nil
}

func (r *repository) GetByUserID(ctx context.Context, userID string) (*Practitioner, error) {
	query := `
		SELECT id, user_id, abn, verified, created_at, updated_at, deleted_at
		FROM tbl_practitioner
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	var t Practitioner
	if err := r.db.QueryRowxContext(ctx, query, userID).StructScan(&t); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get practitioner by user_id: %w", err)
	}
	return &t, nil
}

var practitionerColumns = map[string]string{
	"id":         "id",
	"user_id":    "user_id",
	"abn":        "abn",
	"verified":   "verified",
	"created_at": "created_at",
}

var practitionerSearchCols = []string{"user_id", "abn"}

func (r *repository) List(ctx context.Context, f common.Filter) ([]*Practitioner, error) {
	base := `
		SELECT id, user_id, abn, verified, created_at, updated_at, deleted_at
		FROM tbl_practitioner
		WHERE deleted_at IS NULL
	`

	query, filterArgs := common.BuildQuery(base, f, practitionerColumns, practitionerSearchCols, false)

	var list []*Practitioner
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), filterArgs...); err != nil {
		return nil, fmt.Errorf("list practitioners: %w", err)
	}
	return list, nil
}

func (r *repository) Update(ctx context.Context, t *Practitioner) (*Practitioner, error) {
	query := `
		UPDATE tbl_practitioner
		SET abn = $2, verified = $3, updated_at = $4
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, user_id, abn, verified, created_at, updated_at, deleted_at
	`
	var out Practitioner
	if err := r.db.QueryRowxContext(ctx, query, t.ID, t.ABN, t.Verified, t.UpdatedAt).StructScan(&out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update practitioner: %w", err)
	}
	return &out, nil
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. delete Practitioner
	queryPrac := `UPDATE tbl_practitioner SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL RETURNING id`
	var pracID uuid.UUID
	if err := tx.GetContext(ctx, &pracID, queryPrac, id); err != nil {
		return ErrNotFound
	}

	// 2. delete linked User
	queryUser := `UPDATE tbl_user SET deleted_at = now(), email = email || '.del.' || id::text 
	              WHERE id = (SELECT user_id FROM tbl_practitioner WHERE id = $1)`
	if _, err := tx.ExecContext(ctx, queryUser, id); err != nil {
		return fmt.Errorf("delete linked user: %w", err)
	}

	// 3. Revoke all Accountant Associations
	queryRevoke := `UPDATE tbl_invite_permissions SET deleted_at = now() WHERE practitioner_id = $1 AND deleted_at IS NULL`
	if _, err := tx.ExecContext(ctx, queryRevoke, id); err != nil {
		return fmt.Errorf("revoke accountant permissions: %w", err)
	}

	// 4. Delete Practitioner Setting
	querySettings := `UPDATE tbl_practitioner_setting SET deleted_at = now() WHERE practitioner_id = $1 AND deleted_at IS NULL`
	if _, err := tx.ExecContext(ctx, querySettings, id); err != nil {
		return fmt.Errorf("delete practitioner settings: %w", err)
	}

	// 5. Delete Chart of Accounts
	queryAccounts := `UPDATE tbl_chart_of_accounts SET deleted_at = now() WHERE practitioner_id = $1 AND deleted_at IS NULL`
	if _, err := tx.ExecContext(ctx, queryAccounts, id); err != nil {
		return fmt.Errorf("delete chart of accounts: %w", err)
	}

	// 6. Delete Practitioner Subscriptions
	querySubscriptions := `UPDATE tbl_practitioner_subscription SET deleted_at = now() WHERE practitioner_id = $1 AND deleted_at IS NULL`
	if _, err := tx.ExecContext(ctx, querySubscriptions, id); err != nil {
		return fmt.Errorf("delete practitioner subscriptions: %w", err)
	}

	// 11. delete Clinics
	var clinicIDs []uuid.UUID
	queryClinics := `UPDATE tbl_clinic SET deleted_at = now() WHERE practitioner_id = $1 AND deleted_at IS NULL RETURNING id`
	if err := tx.SelectContext(ctx, &clinicIDs, queryClinics, id); err != nil {
		return fmt.Errorf("delete clinics: %w", err)
	}
	//  Delete custom form versions
	var versionIDs []uuid.UUID
	queryVersions := `UPDATE tbl_custom_form_version 
                  SET deleted_at = now() 
                  WHERE practitioner_id = $1 AND deleted_at IS NULL`

	if _, err := tx.ExecContext(ctx, queryVersions, id); err != nil {
		return fmt.Errorf("delete custom form versions: %w", err)
	}

	if len(clinicIDs) > 0 {
		// 11a. Delete Clinic Addresses
		clinicAddrs, args, _ := sqlx.In(`UPDATE tbl_clinic_address SET deleted_at = now() WHERE clinic_id IN (?) AND deleted_at IS NULL`, clinicIDs)
		if _, err := tx.ExecContext(ctx, tx.Rebind(clinicAddrs), args...); err != nil {
			return fmt.Errorf("delete clinic addresses: %w", err)
		}

		// 11b. Delete Clinic Contacts
		clinicContacts, args, _ := sqlx.In(`UPDATE tbl_clinic_contact SET deleted_at = now() WHERE clinic_id IN (?) AND deleted_at IS NULL`, clinicIDs)
		if _, err := tx.ExecContext(ctx, tx.Rebind(clinicContacts), args...); err != nil {
			return fmt.Errorf("delete clinic contacts: %w", err)
		}

		// 11c. Delete Clinic Financial Settings
		FinSettings, args, _ := sqlx.In(`UPDATE tbl_financial_settings SET deleted_at = now() WHERE clinic_id IN (?) AND deleted_at IS NULL`, clinicIDs)
		if _, err := tx.ExecContext(ctx, tx.Rebind(FinSettings), args...); err != nil {
			return fmt.Errorf("delete financial settings: %w", err)
		}

		// 12. Delete Forms
		var formIDs []uuid.UUID
		queryForms, args, _ := sqlx.In(`UPDATE tbl_form SET deleted_at = now() WHERE clinic_id IN (?) AND deleted_at IS NULL RETURNING id`, clinicIDs)
		if err := tx.SelectContext(ctx, &formIDs, tx.Rebind(queryForms), args...); err != nil {
			return fmt.Errorf("delete forms: %w", err)
		}

		// 12a. Delete Form Fields linked to those Forms
		if len(versionIDs) > 0 {
			queryFields, args, _ := sqlx.In(`UPDATE tbl_form_field SET deleted_at = now() WHERE form_version_id IN (?) AND deleted_at IS NULL`, versionIDs)
			if _, err := tx.ExecContext(ctx, tx.Rebind(queryFields), args...); err != nil {
				return fmt.Errorf("delete form fields: %w", err)
			}
		}

		// 13. Delete Form Entries
		var entryIDs []uuid.UUID
		queryEntries, args, _ := sqlx.In(`UPDATE tbl_form_entry SET deleted_at = now() WHERE clinic_id IN (?) AND deleted_at IS NULL RETURNING id`, clinicIDs)
		if err := tx.SelectContext(ctx, &entryIDs, tx.Rebind(queryEntries), args...); err != nil {
			return fmt.Errorf("delete entries: %w", err)
		}

		// 13a. Delete Entry Values
		if len(entryIDs) > 0 {
			queryValues, args, _ := sqlx.In(`UPDATE tbl_form_entry_value SET deleted_at = now() WHERE entry_id IN (?) AND deleted_at IS NULL`, entryIDs)
			if _, err := tx.ExecContext(ctx, tx.Rebind(queryValues), args...); err != nil {
				return fmt.Errorf("delete entry values: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (r *repository) GetSettingByPractitionerID(ctx context.Context, practitionerID uuid.UUID) (*PractitionerSetting, error) {
	query := `
		SELECT id, practitioner_id, timezone, logo, color, created_at, updated_at, deleted_at
		FROM tbl_practitioner_setting
		WHERE practitioner_id = $1 AND deleted_at IS NULL
	`
	var s PractitionerSetting
	if err := r.db.QueryRowxContext(ctx, query, practitionerID).StructScan(&s); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get practitioner setting: %w", err)
	}
	return &s, nil
}

func (r *repository) UpsertSetting(ctx context.Context, s *PractitionerSetting) (*PractitionerSetting, error) {
	// Try update first if record exists
	_, err := r.GetSettingByPractitionerID(ctx, s.PractitionerID)
	if err == nil {
		query := `
			UPDATE tbl_practitioner_setting
			SET timezone = $2, logo = $3, color = $4, updated_at = $5
			WHERE practitioner_id = $1 AND deleted_at IS NULL
			RETURNING id, practitioner_id, timezone, logo, color, created_at, updated_at, deleted_at
		`
		var out PractitionerSetting
		if err := r.db.QueryRowxContext(ctx, query, s.PractitionerID, s.Timezone, s.Logo, s.Color, s.UpdatedAt).StructScan(&out); err != nil {
			return nil, fmt.Errorf("update practitioner setting: %w", err)
		}
		return &out, nil
	}
	// Insert new
	query := `
		INSERT INTO tbl_practitioner_setting (practitioner_id, timezone, logo, color, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, practitioner_id, timezone, logo, color, created_at, updated_at, deleted_at
	`
	now := time.Now()
	var out PractitionerSetting
	if err := r.db.QueryRowxContext(ctx, query, s.PractitionerID, s.Timezone, s.Logo, s.Color, now, now).StructScan(&out); err != nil {
		return nil, fmt.Errorf("create practitioner setting: %w", err)
	}
	return &out, nil
}

func (r *repository) Count(ctx context.Context, f common.Filter) (int, error) {
	base := `FROM tbl_practitioner WHERE deleted_at IS NULL`

	query, filterArgs := common.BuildQuery(base, f, practitionerColumns, practitionerSearchCols, true)

	var count int
	if err := r.db.GetContext(ctx, &count, r.db.Rebind(query), filterArgs...); err != nil {
		return 0, fmt.Errorf("count practitioners: %w", err)
	}
	return count, nil
}

func (r *repository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `
        UPDATE tbl_practitioner 
        SET deleted_at = now(), updated_at = now(),status = 'INACTIVE'
        WHERE user_id = $1 AND deleted_at IS NULL
    `
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete practitioner by user id: %w", err)
	}
	return nil
}
