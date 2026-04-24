package invitation

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

type IRepository interface {
	Create(ctx context.Context, tx *sqlx.Tx, inv *Invitation) error
	ListInvitations(ctx context.Context, f common.Filter, count bool) ([]*Invitation, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Invitation, error)
	GetByEmail(ctx context.Context, email string) (*Invitation, error)
	GetInvitationByEmail(ctx context.Context, accountantID *uuid.UUID, email string) (*Invitation, error)

	GetPractitionerName(ctx context.Context, practitionerID uuid.UUID) (string, error)

	GetInvitationByID(ctx context.Context, id uuid.UUID) (*InvitationExtended, error)
	// CountByEmail(ctx context.Context, email string, f common.Filter) (int, error)

	ListPermission(ctx context.Context, accId uuid.UUID, f common.Filter) (*InvitationWithPermissions, error)
	GetPermission(ctx context.Context, accountantID *uuid.UUID, practitionerID uuid.UUID, email *string) (*Permissions, error)
	GrantEntityPermission(ctx context.Context, tx *sqlx.Tx, pID uuid.UUID, accID *uuid.UUID, email string, perms Permissions) error
	UpdateStatus(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status InvitationStatus, accountantID *uuid.UUID) error
	CountInvitesByEmail(ctx context.Context, pID uuid.UUID, email string) (int, error)

	DeletePermission(ctx context.Context, tx *sqlx.Tx, pID uuid.UUID, accID *uuid.UUID, email string) error
	IsUserLink(ctx context.Context, practitionerID *uuid.UUID, accountantId *uuid.UUID) (bool, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, tx *sqlx.Tx, inv *Invitation) error {
	query := `INSERT INTO tbl_invitation (id, practitioner_id, accountant_id, email, status, expires_at) 
              VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := tx.ExecContext(ctx, query, inv.ID, inv.PractitionerID, inv.AccountantID, inv.Email, inv.Status, inv.ExpiresAt)
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

func (r *repository) UpdateStatus(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status InvitationStatus, accountantID *uuid.UUID) error {
	query := `UPDATE tbl_invitation SET status = $1, accountant_id = $2 WHERE id = $3`
	_, err := tx.ExecContext(ctx, query, status, accountantID, id)
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

func (r *repository) ListInvitations(ctx context.Context, f common.Filter, count bool) ([]*Invitation, int, error) {
	base := `SELECT id, practitioner_id, accountant_id, email, status, created_at, expires_at FROM tbl_invitation`

	var query string
	var filterArgs []interface{}

	if count {
		query, filterArgs = common.BuildQuery("SELECT COUNT(*) FROM tbl_invitation", f, invitationColumns, invitationSearchCols, true)
	} else {
		query, filterArgs = common.BuildQuery(base, f, invitationColumns, invitationSearchCols, false)
	}

	var list []*Invitation
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), filterArgs...); err != nil {
		return nil, 0, fmt.Errorf("list invitations repo: %w", err)
	}
	return list, len(list), nil
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

// func (r *repository) CountByEmail(ctx context.Context, email string, f common.Filter) (int, error) {
// 	query := `SELECT COUNT(*) FROM tbl_invitation WHERE email = $1 AND status::text != $2`

// 	var total int
// 	if err := r.db.GetContext(ctx, &total, query, email, string(StatusResent)); err != nil {
// 		return 0, fmt.Errorf("count invitations by email: %w", err)
// 	}
// 	return total, nil
// }

// GetPermission checks if an accountant has access to a practitioner
func (r *repository) GetPermission(ctx context.Context, accountantID *uuid.UUID, practitionerID uuid.UUID, email *string) (*Permissions, error) {
	var invitationID uuid.UUID

	// First, get the invitation ID
	if accountantID != nil && *accountantID != uuid.Nil {
		query := `SELECT id FROM tbl_invitation WHERE practitioner_id = $1 AND accountant_id = $2 AND status IN ('SENT', 'ACCEPTED', 'COMPLETED') LIMIT 1`
		err := r.db.GetContext(ctx, &invitationID, query, practitionerID, accountantID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("get invitation by accountant: %w", err)
		}
	} else if email != nil && *email != "" {
		query := `SELECT id FROM tbl_invitation WHERE practitioner_id = $1 AND email = $2 AND status IN ('SENT', 'ACCEPTED', 'COMPLETED') LIMIT 1`
		err := r.db.GetContext(ctx, &invitationID, query, practitionerID, email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("get invitation by email: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either accountant_id or email must be provided")
	}

	// Now get the permissions for this invitation
	type PermRow struct {
		PermissionName string `db:"name"`
		CanRead        bool   `db:"can_read"`
		CanWrite       bool   `db:"can_write"`
	}

	var rows []PermRow
	query := `
		SELECT p.name, ip.can_read, ip.can_write
		FROM tbl_invite_permissions ip
		JOIN tbl_permission p ON ip.permission_id = p.id
		WHERE ip.invitation_id = $1
	`

	if err := r.db.SelectContext(ctx, &rows, query, invitationID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get permissions: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	perms := make(Permissions)
	for _, row := range rows {
		perms[PermissionName(row.PermissionName)] = AccessLevel{
			Read:  row.CanRead,
			Write: row.CanWrite,
		}
	}

	return &perms, nil
}

func (r *repository) GrantEntityPermission(ctx context.Context, tx *sqlx.Tx, pID uuid.UUID, accID *uuid.UUID, email string, perms Permissions) error {
	// First, get or verify the invitation
	var invitationID uuid.UUID
	var query string

	if accID != nil && *accID != uuid.Nil {
		query = `SELECT id FROM tbl_invitation WHERE practitioner_id = $1 AND accountant_id = $2 LIMIT 1`
		err := tx.GetContext(ctx, &invitationID, query, pID, accID)
		if err != nil {
			return fmt.Errorf("invitation not found for accountant: %w", err)
		}
	} else {
		if email == "" {
			return fmt.Errorf("email required when accountant ID is absent")
		}
		query = `SELECT id FROM tbl_invitation WHERE practitioner_id = $1 AND email = $2 LIMIT 1`
		err := tx.GetContext(ctx, &invitationID, query, pID, email)
		if err != nil {
			return fmt.Errorf("invitation not found for email: %w", err)
		}
	}

	// Insert or update permissions
	for _, row := range perms.ToRows() {
		// Get permission_id from tbl_permission
		var permissionID int
		permQuery := `SELECT id FROM tbl_permission WHERE name = $1`
		err := tx.GetContext(ctx, &permissionID, permQuery, string(row.Name))
		if err != nil {
			return fmt.Errorf("permission %q not found: %w", row.Name, err)
		}

		// Upsert the permission
		upsertQuery := `
			INSERT INTO tbl_invite_permissions (id, invitation_id, permission_id, can_read, can_write, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			ON CONFLICT (invitation_id, permission_id) 
			DO UPDATE SET can_read = EXCLUDED.can_read, can_write = EXCLUDED.can_write, updated_at = NOW()
		`

		_, err = tx.ExecContext(ctx, upsertQuery,
			uuid.New(),
			invitationID,
			permissionID,
			row.AccessLevel.Read,
			row.AccessLevel.Write,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert permission %q: %w", row.Name, err)
		}
	}

	return nil
}

func (r *repository) UpdateStatusTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status InvitationStatus, accountantID *uuid.UUID) error {
	query := `UPDATE tbl_invitation SET status = $1, accountant_id = $2 WHERE id = $3`
	_, err := tx.ExecContext(ctx, query, status, accountantID, id)
	return err
}

func (r *repository) ListPermission(ctx context.Context, accId uuid.UUID, f common.Filter) (*InvitationWithPermissions, error) {
	// Define columns with table aliases to avoid ambiguity
	permissionColumns := map[string]string{
		"email":           "i.email",
		"status":          "i.status::text",
		"created_at":      "i.created_at",
		"practitioner_id": "i.practitioner_id",
		"accountant_id":   "i.accountant_id",
	}

	base := `SELECT 
        i.id as invitation_id,
        i.practitioner_id,
        i.accountant_id,
        i.created_at as invitation_created_at,
        ip.updated_at as permissions_updated_at,
        p.name as permission_name, 
        ip.can_read, 
        ip.can_write 
    FROM tbl_invite_permissions ip
	JOIN tbl_permission p ON ip.permission_id = p.id
	JOIN tbl_invitation i ON ip.invitation_id = i.id
	WHERE i.accountant_id = $1 AND i.status IN ('ACCEPTED', 'COMPLETED')
	ORDER BY i.created_at DESC`

	type PermRow struct {
		InvitationID         uuid.UUID `db:"invitation_id"`
		PractitionerID       uuid.UUID `db:"practitioner_id"`
		AccountantID         uuid.UUID `db:"accountant_id"`
		InvitationCreatedAt  time.Time `db:"invitation_created_at"`
		PermissionsUpdatedAt time.Time `db:"permissions_updated_at"`
		PermissionName       string    `db:"permission_name"`
		CanRead              bool      `db:"can_read"`
		CanWrite             bool      `db:"can_write"`
	}

	var rows []PermRow
	if err := r.db.SelectContext(ctx, &rows, query, accountantID); err != nil {
		return nil, fmt.Errorf("list accountant permissions repo: %w", err)
	}

	if len(rows) == 0 {
		return []*InvitationWithPermissions{}, nil
	}

	// Use the first row for invitation details (all rows have same invitation data)
	firstRow := rows[0]

	perms := make(Permissions)
	for _, row := range rows {
		if _, exists := invitationMap[row.InvitationID]; !exists {
			invitationMap[row.InvitationID] = &InvitationWithPermissions{
				ID:             row.InvitationID,
				PractitionerID: row.PractitionerID,
				AccountantID:   row.AccountantID,
				CreatedAt:      row.InvitationCreatedAt,
				UpdatedAt:      row.PermissionsUpdatedAt,
				Permissions:    make(Permissions),
			}
		}

		invitationMap[row.InvitationID].Permissions[PermissionName(row.PermissionName)] = AccessLevel{
			Read:  row.CanRead,
			Write: row.CanWrite,
		}
	}

	// Convert map to slice
	result := make([]*InvitationWithPermissions, 0, len(invitationMap))
	for _, inv := range invitationMap {
		result = append(result, inv)
	}

	return result, nil
}

func (r *repository) CountPermission(ctx context.Context, f common.Filter) (int, error) {
	base := `FROM tbl_invite_permissions ip
	JOIN tbl_permission p ON ip.permission_id = p.id
	JOIN tbl_invitation i ON ip.invitation_id = i.id`

	query, filterArgs := common.BuildQuery(base, f, invitationColumns, invitationSearchCols, true)

	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(query), filterArgs...); err != nil {
		return 0, fmt.Errorf("count accountant permissions repo: %w", err)
	}

	return total, nil
}

// CountInvitesByEmail implements [IRepository].
func (r *repository) CountInvitesByEmail(ctx context.Context, pID uuid.UUID, email string) (int, error) {
	query := `SELECT COUNT(*) FROM tbl_invitation WHERE practitioner_id = $1 AND email = $2 AND status IN ('SENT', 'ACCEPTED', 'COMPLETED')`

	var total int
	if err := r.db.GetContext(ctx, &total, query, pID, email); err != nil {
		return 0, fmt.Errorf("count invites by email: %w", err)
	}
	return total, nil
}

// DeletePermission implements [IRepository].
func (r *repository) DeletePermission(ctx context.Context, tx *sqlx.Tx, pID uuid.UUID, accID *uuid.UUID, email string) error {
	// First, get the invitation ID
	var invitationID uuid.UUID
	var query string
	if accID != nil && *accID != uuid.Nil {
		query = `SELECT id FROM tbl_invitation WHERE practitioner_id = $1 AND accountant_id = $2 LIMIT 1`
		err := tx.GetContext(ctx, &invitationID, query, pID, accID)
		if err != nil {
			return fmt.Errorf("get invitation by accountant: %w", err)
		}
	} else {
		if email == "" {
			return fmt.Errorf("email required when accountant ID is absent")
		}
		query = `SELECT id FROM tbl_invitation WHERE practitioner_id = $1 AND email = $2 LIMIT 1`
		err := tx.GetContext(ctx, &invitationID, query, pID, email)
		if err != nil {
			return fmt.Errorf("get invitation by email: %w", err)
		}
	}

	deleteQuery := `DELETE FROM tbl_invite_permissions WHERE invitation_id = $1`
	_, err := tx.ExecContext(ctx, deleteQuery, invitationID)
	return err
}

// IsLink implements [IRepository].
func (r *repository) IsUserLink(ctx context.Context, practitionerID *uuid.UUID, accountantId *uuid.UUID) (bool, error) {
	var linked bool
	query := `SELECT EXISTS (
		SELECT 1 
		FROM tbl_invitation
		WHERE practitioner_id = $1
		AND accountant_id = $2
	AND status IN ('COMPLETED')
	)`

	err := r.db.GetContext(ctx, &linked, query, practitionerID, accountantId)
	if err != nil {
		return false, fmt.Errorf("check user link: %w", err)
	}

	return linked, nil
}

// GetInvitationByEmail implements [IRepository].
func (r *repository) GetInvitationByEmail(ctx context.Context, accountantID *uuid.UUID, email string) (*Invitation, error) {
	inv := &Invitation{}
	if accountantID != nil && *accountantID != uuid.Nil {
		query := `SELECT * FROM tbl_invitation WHERE email = $1 AND accountant_id = $2 ORDER BY created_at DESC LIMIT 1`
		err := r.db.GetContext(ctx, inv, query, email, accountantID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		return inv, nil
	}
	query := `SELECT * FROM tbl_invitation WHERE email = $1 ORDER BY created_at DESC LIMIT 1`
	err := r.db.GetContext(ctx, inv, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return inv, nil
}
