package invitation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Create(ctx context.Context, inv *Invitation) error
	GetByID(ctx context.Context, id uuid.UUID) (*Invitation, error)
	GetByEmail(ctx context.Context, email string) (*Invitation, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status InvitationStatus, entityID *uuid.UUID) error
	GetPractitionerName(ctx context.Context, practitionerID uuid.UUID) (string, error)

	GetAccountantIDByEmail(ctx context.Context, email string) (*uuid.UUID, error)
	GetUserIDByEmail(ctx context.Context, email string) (*uuid.UUID, error)
	List(ctx context.Context, f common.Filter) ([]*Invitation, error)
	Count(ctx context.Context, f common.Filter) (int, error)
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*InvitationExtended, error)
	GetUserDetailsByEmail(ctx context.Context, email string) (*UserDetails, error)
	CountDailyInvitesByEmail(ctx context.Context, practitionerID uuid.UUID, email string) (int, error)
	GetEmailByAccountantID(ctx context.Context, accountantID uuid.UUID) (string, error)
	ListByEmail(ctx context.Context, email string, f common.Filter) ([]*Invitation, error)
	CountByEmail(ctx context.Context, email string, f common.Filter) (int, error)

	GetPermissions(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (*Permissions, error)
	GrantEntityPermissionTx(ctx context.Context, tx *sqlx.Tx, practitionerID, accountantID, entityID uuid.UUID, entityType string, perms Permissions) error
	DeletePermissionsByEntityTx(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error
	GetPractitionerLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) (uuid.UUID, error)
	GrantEntityPermission(ctx context.Context, pID, aID, eID uuid.UUID, eType string, permJson []byte) error
	DeleteAllPermissionsForAccountantTx(ctx context.Context, tx *sqlx.Tx, practitionerID, accountantID uuid.UUID) error
	UpdateStatusTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status InvitationStatus, entityID *uuid.UUID) error
	ListAccountantPermissions(ctx context.Context, accountantID uuid.UUID, f common.Filter) ([]AccountantPermissionRow, error)
	CountAccountantPermissions(ctx context.Context, accountantID uuid.UUID, f common.Filter) (int, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, inv *Invitation) error {
	query := `INSERT INTO tbl_invitation (id, practitioner_id, entity_id, email, status, expires_at) 
              VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, query, inv.ID, inv.PractitionerID, inv.EntityID, inv.Email, inv.Status, inv.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create invitation: %w", err)
	}
	return nil
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Invitation, error) {
	inv := &Invitation{}
	query := `SELECT * FROM tbl_invitation WHERE id = $1`
	err := r.db.GetContext(ctx, inv, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return inv, nil
}

func (r *repository) GetByEmail(ctx context.Context, email string) (*Invitation, error) {
	inv := &Invitation{}
	query := `SELECT * FROM tbl_invitation WHERE email = $1 ORDER BY created_at DESC LIMIT 1`
	err := r.db.GetContext(ctx, inv, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return inv, nil
}

func (r *repository) UpdateStatus(ctx context.Context, id uuid.UUID, status InvitationStatus, entityID *uuid.UUID) error {
	query := `UPDATE tbl_invitation SET status = $1, entity_id = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, entityID, id)
	return err
}

func (r *repository) GetPractitionerName(ctx context.Context, practitionerID uuid.UUID) (string, error) {
	var name struct {
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}

	// Joining practitioner to user to get the name
	query := `
		SELECT u.first_name, u.last_name 
		FROM tbl_practitioner p
		JOIN tbl_user u ON p.user_id = u.id
		WHERE p.id = $1`

	err := r.db.GetContext(ctx, &name, query, practitionerID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch practitioner name: %w", err)
	}

	return fmt.Sprintf("%s %s", name.FirstName, name.LastName), nil
}

func (r *repository) GetAccountantIDByEmail(ctx context.Context, email string) (*uuid.UUID, error) {
	var accountantID uuid.UUID
	query := `
        SELECT a.id 
        FROM tbl_accountant a
        JOIN tbl_user u ON a.user_id = u.id
        WHERE u.email = $1 
        LIMIT 1`

	err := r.db.GetContext(ctx, &accountantID, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &accountantID, nil
}

func (r *repository) GetUserIDByEmail(ctx context.Context, email string) (*uuid.UUID, error) {
	var userID uuid.UUID
	query := `SELECT id FROM tbl_user WHERE email = $1 LIMIT 1`
	err := r.db.GetContext(ctx, &userID, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &userID, nil
}

func (r *repository) List(ctx context.Context, f common.Filter) ([]*Invitation, error) {
	base := `SELECT id, practitioner_id, entity_id, email, status, created_at, expires_at FROM tbl_invitation`

	query, filterArgs := common.BuildQuery(base, f, invitationColumns, invitationSearchCols, false)

	var list []*Invitation
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), filterArgs...); err != nil {
		return nil, fmt.Errorf("list invitations repo: %w", err)
	}
	return list, nil
}

func (r *repository) Count(ctx context.Context, f common.Filter) (int, error) {
	base := `FROM tbl_invitation`
	query, filterArgs := common.BuildQuery(base, f, invitationColumns, invitationSearchCols, true)

	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(query), filterArgs...); err != nil {
		return 0, fmt.Errorf("count invitations repo: %w", err)
	}
	return total, nil
}

func (r *repository) GetInvitationByID(ctx context.Context, id uuid.UUID) (*InvitationExtended, error) {
	inv := &InvitationExtended{}
	query := `
		SELECT 
			i.*, 
			u.first_name AS sender_first_name, 
			u.last_name AS sender_last_name, 
			u.email AS sender_email
		FROM tbl_invitation i
		JOIN tbl_practitioner p ON i.practitioner_id = p.id
		JOIN tbl_user u ON p.user_id = u.id
		WHERE i.id = $1`

	err := r.db.GetContext(ctx, inv, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return inv, nil
}

func (r *repository) GetUserDetailsByEmail(ctx context.Context, email string) (*UserDetails, error) {
	var details UserDetails
	query := `SELECT first_name, last_name, email FROM tbl_user WHERE email = $1 AND deleted_at IS NULL LIMIT 1`
	err := r.db.GetContext(ctx, &details, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil so the service knows it's not found
		}
		return nil, err
	}
	return &details, nil
}

func (r *repository) CountDailyInvitesByEmail(ctx context.Context, practitionerID uuid.UUID, email string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) 
		FROM tbl_invitation 
		WHERE practitioner_id = $1 
		  AND email = $2 
		  AND created_at > NOW() - INTERVAL '24 hours'`

	err := r.db.GetContext(ctx, &count, query, practitionerID, email)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *repository) GetEmailByAccountantID(ctx context.Context, accountantID uuid.UUID) (string, error) {
	var email string
	query := `
		SELECT u.email FROM tbl_accountant a
		JOIN tbl_user u ON a.user_id = u.id
		WHERE a.id = $1 AND a.deleted_at IS NULL`
	if err := r.db.GetContext(ctx, &email, query, accountantID); err != nil {
		return "", fmt.Errorf("get email by accountant id: %w", err)
	}
	return email, nil
}

func (r *repository) ListByEmail(ctx context.Context, email string, f common.Filter) ([]*Invitation, error) {
	query := `SELECT id, practitioner_id, entity_id, email, status, created_at, expires_at 
	          FROM tbl_invitation WHERE email = $1 AND status::text != $2`

	args := []interface{}{email, string(StatusResent)}

	if f.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *f.Limit)
	}
	if f.Offset != nil {
		query += fmt.Sprintf(" OFFSET %d", *f.Offset)
	}

	var list []*Invitation
	if err := r.db.SelectContext(ctx, &list, query, args...); err != nil {
		return nil, fmt.Errorf("list invitations by email: %w", err)
	}
	return list, nil
}

func (r *repository) CountByEmail(ctx context.Context, email string, f common.Filter) (int, error) {
	query := `SELECT COUNT(*) FROM tbl_invitation WHERE email = $1 AND status::text != $2`

	var total int
	if err := r.db.GetContext(ctx, &total, query, email, string(StatusResent)); err != nil {
		return 0, fmt.Errorf("count invitations by email: %w", err)
	}
	return total, nil
}

// GetPermissions checks if an accountant has access to a specific entity (Clinic or Form)
func (r *repository) GetPermissions(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (*Permissions, error) {
	var permissions Permissions
	var raw json.RawMessage

	// We check for a mapping where the accountant is linked to the entity
	query := `
        SELECT permissions
        FROM tbl_invite_permissions
        WHERE accountant_id = $1 AND entity_id = $2 AND deleted_at IS NULL LIMIT 1
    `
	// var raw []byte
	err := r.db.GetContext(ctx, &raw, query, accountantID, entityID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No mapping means no permission
		}
		return nil, err
	}

	if err := json.Unmarshal(raw, &permissions); err != nil {
		return nil, err
	}

	return &permissions, nil
}

// This method is for granting access to the resouce on "create" action since no mapping exisitng while creating a new
// entity so we check for parent entity permissions and after creating the resource by default assign a "read" permission.
func (r *repository) GrantEntityPermissionTx(ctx context.Context, tx *sqlx.Tx, practitionerID, accountantID, entityID uuid.UUID, entityType string, perms Permissions) error {
	permJSON, err := json.Marshal(perms)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	query := `
		INSERT INTO tbl_invite_permissions (
			id, practitioner_id, accountant_id, entity_id, entity_type, permissions, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, NOW(), Now()
		)
	`

	_, err = tx.ExecContext(ctx, query,
		uuid.New(),
		practitionerID,
		accountantID,
		entityID,
		entityType,
		permJSON, // This sends the JSON string to the DB
	)

	if err != nil {
		return fmt.Errorf("grant entity permission repo: %w", err)
	}

	return nil
}

func (r *repository) DeletePermissionsByEntityTx(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error {
	query := `
        UPDATE tbl_invite_permissions 
        SET deleted_at = NOW(), updated_at = NOW() 
        WHERE entity_id = $1 AND deleted_at IS NULL
    `
	_, err := tx.ExecContext(ctx, query, entityID)
	if err != nil {
		return fmt.Errorf("delete entity permissions: %w", err)
	}
	return nil
}

func (r *repository) GetPractitionerLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) (uuid.UUID, error) {
	var practitionerID uuid.UUID
	query := `SELECT practitioner_id FROM tbl_invitation WHERE entity_id = $1 AND status = 'COMPLETED' LIMIT 1`
	err := r.db.GetContext(ctx, &practitionerID, query, accountantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("accountant %s is not linked to any practitioner", accountantID)
		}
		return uuid.Nil, err
	}
	return practitionerID, err
}

func (r *repository) GrantEntityPermission(ctx context.Context, pID, aID, eID uuid.UUID, eType string, permJson []byte) error {
	query := `
        INSERT INTO tbl_invite_permissions (
            id, practitioner_id, accountant_id, entity_id, entity_type, permissions, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, NOW(), NOW()
        )
        ON CONFLICT ON CONSTRAINT unique_permission_scope
        DO UPDATE SET 
            -- This performs the 'Full Overwrite' to remove old keys
            permissions = EXCLUDED.permissions,
            updated_at = NOW(),
            deleted_at = NULL;
    `

	_, err := r.db.ExecContext(ctx, query, uuid.New(), pID, aID, eID, eType, permJson)
	return err
}

func (r *repository) DeleteAllPermissionsForAccountantTx(ctx context.Context, tx *sqlx.Tx, practitionerID, accountantID uuid.UUID) error {
	query := `
        UPDATE tbl_invite_permissions 
        SET deleted_at = NOW(), updated_at = NOW() 
        WHERE practitioner_id = $1 
          AND accountant_id = $2 
          AND deleted_at IS NULL
    `
	_, err := tx.ExecContext(ctx, query, practitionerID, accountantID)
	if err != nil {
		return fmt.Errorf("delete accountant permissions tx: %w", err)
	}
	return nil
}

func (r *repository) UpdateStatusTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status InvitationStatus, entityID *uuid.UUID) error {
	query := `UPDATE tbl_invitation SET status = $1, entity_id = $2 WHERE id = $3`
	_, err := tx.ExecContext(ctx, query, status, entityID, id)
	return err
}

func (r *repository) ListAccountantPermissions(ctx context.Context, accountantID uuid.UUID, f common.Filter) ([]AccountantPermissionRow, error) {
	f.Where = append(f.Where, common.Condition{
		Field:    "accountant_id",
		Operator: common.OpEq,
		Value:    accountantID,
	})

	// 2. Base query must be just the FROM clause (BuildQuery adds WHERE, ORDER, LIMIT)
	// IMPORTANT: Do not include "WHERE accountant_id = $1" here!
	base := `FROM tbl_invite_permissions`

	// Select columns
	columns := `SELECT id, entity_id, entity_type, practitioner_id, accountant_id, permissions, created_at, updated_at, deleted_at `

	// 3. Build the query and the argument slice
	// BuildQuery will handle the "?", the sorting, and the LIMIT/OFFSET integers.
	querySuffix, filterArgs := common.BuildQuery(base, f, invitationColumns, invitationSearchCols, false)

	fullQuery := columns + querySuffix

	var perms []AccountantPermissionRow
	// 4. Rebind converts all "?" into sequential "$1, $2, $3..."
	// This ensures accountantID is $1 and LIMIT is a later number (BigInt).
	if err := r.db.SelectContext(ctx, &perms, r.db.Rebind(fullQuery), filterArgs...); err != nil {
		return nil, fmt.Errorf("list accountant permissions repo: %w", err)
	}

	return perms, nil
}

func (r *repository) CountAccountantPermissions(ctx context.Context, accountantID uuid.UUID, f common.Filter) (int, error) {

	f.Where = append(f.Where, common.Condition{
		Field:    "accountant_id",
		Operator: common.OpEq,
		Value:    accountantID,
	})

	base := `FROM tbl_invite_permissions`

	query, filterArgs := common.BuildQuery(base, f, invitationColumns, invitationSearchCols, true)

	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(query), filterArgs...); err != nil {
		return 0, fmt.Errorf("count accountant permissions repo: %w", err)
	}

	return total, nil
}
