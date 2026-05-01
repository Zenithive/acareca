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

type Repository interface {
	Create(ctx context.Context, inv *Invitation) error
	CreateTx(ctx context.Context, tx *sqlx.Tx, inv *Invitation) error
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
	GetPractitionerEmailByID(ctx context.Context, practitionerID uuid.UUID) (string, error)
	ListForPractitioner(ctx context.Context, practitionerID uuid.UUID, f common.Filter) ([]*RsInvitationListItem, error)
	ListForAccountant(ctx context.Context, accountantEmail string, f common.Filter) ([]*RsInvitationListItem, error)
	CountByEmail(ctx context.Context, email string, f common.Filter) (int, error)

	ListPermissions(ctx context.Context, accountantID uuid.UUID, f common.Filter) ([]*InvitationWithPermissions, error)

	GetPermission(ctx context.Context, accountantID *uuid.UUID, entityID uuid.UUID, email *string) (*Permissions, error)
	GetPermissionsByPractitionerAndAccountant(ctx context.Context, practitionerID uuid.UUID, accountantID uuid.UUID) (*Permissions, error)
	// GetPermissionsByEmail(ctx context.Context, pID uuid.UUID, email string) ([]RqPermissionDetail, error)
	GrantEntityPermissionTx(ctx context.Context, tx *sqlx.Tx, pID uuid.UUID, accID *uuid.UUID, email string, perms Permissions) error
	DeletePermissionTx(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID) error
	IsAccountantLinkedToPractitioner(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error)
	GetPractitionersLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) ([]uuid.UUID, error)
	DeleteAllPermissionsForAccountantTx(ctx context.Context, tx *sqlx.Tx, practitionerID, accountantID uuid.UUID) error
	UpdateStatusTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status InvitationStatus, entityID *uuid.UUID) error
	CountPermission(ctx context.Context, f common.Filter) (int, error)
	LinkPermissionsToAccountantTx(ctx context.Context, tx *sqlx.Tx, email string, accountantID uuid.UUID) error
	DeletePermission(ctx context.Context, pID uuid.UUID, entityID uuid.UUID, accID *uuid.UUID, email string) error
	// GetAllAccountantPermissions(ctx context.Context, pID uuid.UUID, email string, accID *uuid.UUID) ([]RqPermissionDetail, error)
	AccountantExists(ctx context.Context, id uuid.UUID) (bool, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, inv *Invitation) error {
	query := `INSERT INTO tbl_invitation (id, practitioner_id, accountant_id, email, status, expires_at) 
              VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, query, inv.ID, inv.PractitionerID, inv.AccountantID, inv.Email, inv.Status, inv.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create invitation: %w", err)
	}
	return nil
}

func (r *repository) CreateTx(ctx context.Context, tx *sqlx.Tx, inv *Invitation) error {
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

func (r *repository) UpdateStatus(ctx context.Context, id uuid.UUID, status InvitationStatus, accountantID *uuid.UUID) error {
	query := `UPDATE tbl_invitation SET status = $1, accountant_id = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, accountantID, id)
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
	base := `SELECT id, practitioner_id, accountant_id, email, status, created_at, expires_at FROM tbl_invitation`

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

func (r *repository) GetPractitionerEmailByID(ctx context.Context, practitionerID uuid.UUID) (string, error) {
	var email string
	query := `
		SELECT u.email FROM tbl_practitioner p
		JOIN tbl_user u ON p.user_id = u.id
		WHERE p.id = $1 AND p.deleted_at IS NULL`
	if err := r.db.GetContext(ctx, &email, query, practitionerID); err != nil {
		return "", fmt.Errorf("get email by practitioner id: %w", err)
	}
	return email, nil
}

func (r *repository) ListForPractitioner(ctx context.Context, practitionerID uuid.UUID, f common.Filter) ([]*RsInvitationListItem, error) {
	base := `SELECT i.id, i.practitioner_id, u.email AS practitioner_email, i.accountant_id, i.email, i.status, i.created_at, i.expires_at
	         FROM tbl_invitation i
	         JOIN tbl_practitioner p ON i.practitioner_id = p.id
	         JOIN tbl_user u ON p.user_id = u.id`

	query, filterArgs := common.BuildQuery(base, f, invitationColumns, invitationSearchCols, false)

	var list []*RsInvitationListItem
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), filterArgs...); err != nil {
		return nil, fmt.Errorf("list invitations for practitioner: %w", err)
	}
	return list, nil
}

func (r *repository) ListForAccountant(ctx context.Context, accountantEmail string, f common.Filter) ([]*RsInvitationListItem, error) {
	query := `SELECT i.id, i.practitioner_id, u.email AS practitioner_email, i.accountant_id, i.email, i.status, i.created_at, i.expires_at
	          FROM tbl_invitation i
	          JOIN tbl_practitioner p ON i.practitioner_id = p.id
	          JOIN tbl_user u ON p.user_id = u.id
	          WHERE i.email = $1 AND i.status::text != $2`

	args := []interface{}{accountantEmail, string(StatusResent)}

	if f.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *f.Limit)
	}
	if f.Offset != nil {
		query += fmt.Sprintf(" OFFSET %d", *f.Offset)
	}

	var list []*RsInvitationListItem
	if err := r.db.SelectContext(ctx, &list, query, args...); err != nil {
		return nil, fmt.Errorf("list invitations for accountant: %w", err)
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

// GetPermissionsByPractitionerAndAccountant gets permissions for an accountant-practitioner relationship
func (r *repository) GetPermissionsByPractitionerAndAccountant(ctx context.Context, practitionerID uuid.UUID, accountantID uuid.UUID) (*Permissions, error) {
	// First, get the invitation ID
	var invitationID uuid.UUID
	query := `SELECT id FROM tbl_invitation WHERE practitioner_id = $1 AND accountant_id = $2 AND status IN ('SENT', 'ACCEPTED', 'COMPLETED') LIMIT 1`
	err := r.db.GetContext(ctx, &invitationID, query, practitionerID, accountantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get invitation: %w", err)
	}

	// Now get the permissions
	type PermRow struct {
		PermissionName string `db:"name"`
		CanRead        bool   `db:"can_read"`
		CanWrite       bool   `db:"can_write"`
	}

	var rows []PermRow
	permQuery := `
		SELECT p.name, ip.can_read, ip.can_write
		FROM tbl_invite_permissions ip
		JOIN tbl_permission p ON ip.permission_id = p.id
		WHERE ip.invitation_id = $1
	`

	err = r.db.SelectContext(ctx, &rows, permQuery, invitationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
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

func (r *repository) GrantEntityPermissionTx(ctx context.Context, tx *sqlx.Tx, pID uuid.UUID, accID *uuid.UUID, email string, perms Permissions) error {
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

func (r *repository) DeletePermissionTx(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID) error {
	query := `
		DELETE FROM tbl_invite_permissions 
		WHERE invitation_id IN (
			SELECT id FROM tbl_invitation WHERE practitioner_id = $1
		)
	`
	_, err := tx.ExecContext(ctx, query, practitionerID)
	if err != nil {
		return fmt.Errorf("delete permissions by practitioner tx: %w", err)
	}

	return nil
}

func (r *repository) IsAccountantLinkedToPractitioner(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(
		SELECT 1 FROM tbl_invitation 
		WHERE practitioner_id = $1 AND accountant_id = $2 
		AND status IN ('SENT', 'ACCEPTED', 'COMPLETED')
		LIMIT 1
	)`
	err := r.db.GetContext(ctx, &exists, query, practitionerID, accountantID)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *repository) GetPractitionersLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT DISTINCT practitioner_id 
		FROM tbl_invitation 
		WHERE accountant_id = $1 
		  AND status = 'COMPLETED'`

	var practitionerIDs []uuid.UUID
	err := r.db.SelectContext(ctx, &practitionerIDs, query, accountantID)
	if err != nil {
		return nil, fmt.Errorf("get linked practitioners: %w", err)
	}

	return practitionerIDs, nil
}

func (r *repository) DeleteAllPermissionsForAccountantTx(ctx context.Context, tx *sqlx.Tx, practitionerID, accountantID uuid.UUID) error {
	query := `
        DELETE FROM tbl_invite_permissions 
        WHERE invitation_id IN (
			SELECT id FROM tbl_invitation 
			WHERE practitioner_id = $1 AND accountant_id = $2
		)
    `
	_, err := tx.ExecContext(ctx, query, practitionerID, accountantID)
	if err != nil {
		return fmt.Errorf("delete accountant permissions tx: %w", err)
	}
	return nil
}

func (r *repository) UpdateStatusTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status InvitationStatus, accountantID *uuid.UUID) error {
	query := `UPDATE tbl_invitation SET status = $1, accountant_id = $2 WHERE id = $3`
	_, err := tx.ExecContext(ctx, query, status, accountantID, id)
	return err
}

func (r *repository) ListPermissions(ctx context.Context, accountantID uuid.UUID, f common.Filter) ([]*InvitationWithPermissions, error) {
	query := `SELECT 
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

	// Group permissions by invitation ID
	invitationMap := make(map[uuid.UUID]*InvitationWithPermissions)

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

func (r *repository) LinkPermissionsToAccountantTx(ctx context.Context, tx *sqlx.Tx, email string, accountantID uuid.UUID) error {
	// This function is no longer needed with the new schema since permissions are linked to invitation_id
	// The invitation already has the accountant_id, so permissions are automatically linked
	// We just need to update the invitation's accountant_id, which is done elsewhere
	return nil
}

func (r *repository) DeletePermission(ctx context.Context, pID uuid.UUID, entityID uuid.UUID, accID *uuid.UUID, email string) error {
	return fmt.Errorf("DeletePermission is deprecated: use DeleteAllPermissionsForAccountantTx instead")
}

func (r *repository) AccountantExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM tbl_accountant WHERE id = $1 AND deleted_at IS NULL)`
	err := r.db.GetContext(ctx, &exists, query, id)
	return exists, err
}
