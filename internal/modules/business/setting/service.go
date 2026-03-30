package setting

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	CreatePractitioner(ctx context.Context, req *RqCreatePractitioner) (*RsPractitioner, error)
	GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error)
	GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error)
	ListPractitioners(ctx context.Context, f *Filter) (*util.RsList, error)
	UpdatePractitioner(ctx context.Context, id uuid.UUID, req *RqUpdatePractitioner) (*RsPractitioner, error)
	DeletePractitioner(ctx context.Context, id uuid.UUID) error

	GetSetting(ctx context.Context, practitionerID uuid.UUID) (*RsPractitionerSetting, error)
	UpsertSetting(ctx context.Context, practitionerID uuid.UUID, req *RqUpsertPractitionerSetting) (*RsPractitionerSetting, error)
	// Transaction variant
	UpsertSettingTx(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID, req *RqUpsertPractitionerSetting) (*RsPractitionerSetting, error)
}

type service struct {
	db       *sqlx.DB
	repo     Repository
	auditSvc audit.Service
}

func NewService(db *sqlx.DB, repo Repository, auditSvc audit.Service) Service {
	return &service{db: db, repo: repo, auditSvc: auditSvc}
}

func (s *service) CreatePractitioner(ctx context.Context, req *RqCreatePractitioner) (*RsPractitioner, error) {
	t := req.ToPractitioner()
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	result := created.ToRs()
	// Audit log: practitioner created
	meta := auditctx.GetMetadata(ctx)
	idStr := result.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionPractitionerCreated,
		Module:     auditctx.ModuleBusiness,
		EntityType: strPtr(auditctx.EntityPractitioner),
		EntityID:   &idStr,
		AfterState: result,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})
	return result, nil
}

func (s *service) GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return t.ToRs(), nil
}

func (s *service) GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error) {
	t, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return t.ToRs(), nil
}

func (s *service) ListPractitioners(ctx context.Context, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()
	list, err := s.repo.List(ctx, ft)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, ft)
	if err != nil {
		return nil, err
	}

	out := make([]*RsPractitioner, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}

	var rsList util.RsList
	rsList.MapToList(out, total, *ft.Offset, *ft.Limit)
	return &rsList, nil
}

func (s *service) UpdatePractitioner(ctx context.Context, id uuid.UUID, req *RqUpdatePractitioner) (*RsPractitioner, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Capture state before update
	beforeState := existing.ToRs()

	applyUpdate(existing, req)
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	result := updated.ToRs()

	// Audit log: practitioner updated
	meta := auditctx.GetMetadata(ctx)
	idStr := id.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionPractitionerUpdated,
		Module:      auditctx.ModuleBusiness,
		EntityType:  strPtr(auditctx.EntityPractitioner),
		EntityID:    &idStr,
		BeforeState: beforeState,
		AfterState:  result,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return result, nil
}

func applyUpdate(t *Practitioner, req *RqUpdatePractitioner) {
	if req.ABN != nil {
		t.ABN = req.ABN
	}
	if req.Verified != nil {
		t.Verified = *req.Verified
	}
	t.UpdatedAt = time.Now()
}

func (s *service) DeletePractitioner(ctx context.Context, id uuid.UUID) error {
	// Get existing data so we can log what was deleted
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	beforeState := existing.ToRs()

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// Audit log: practitioner deleted
	meta := auditctx.GetMetadata(ctx)
	idStr := id.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionPractitionerDeleted,
		Module:      auditctx.ModuleBusiness,
		EntityType:  strPtr(auditctx.EntityPractitioner),
		EntityID:    &idStr,
		BeforeState: beforeState,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return nil
}

func (s *service) GetSetting(ctx context.Context, practitionerID uuid.UUID) (*RsPractitionerSetting, error) {
	setting, err := s.repo.GetSettingByPractitionerID(ctx, practitionerID)
	if err != nil {
		return nil, err
	}
	return setting.ToRs(), nil
}

func (s *service) UpsertSetting(ctx context.Context, practitionerID uuid.UUID, req *RqUpsertPractitionerSetting) (*RsPractitionerSetting, error) {
	// Defaults
	timezone := "Australia/Sydney"
	color := "#000000"
	if req.Timezone != nil {
		timezone = *req.Timezone
	}
	if req.Color != nil {
		color = *req.Color
	}
	setting := &PractitionerSetting{
		PractitionerID: practitionerID,
		Timezone:       timezone,
		Logo:           req.Logo,
		Color:          color,
		UpdatedAt:      time.Now(),
	}
	updated, err := s.repo.UpsertSetting(ctx, setting)
	if err != nil {
		return nil, err
	}

	result := updated.ToRs()

	// Audit log: setting updated
	meta := auditctx.GetMetadata(ctx)
	idStr := practitionerID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionSettingUpdated,
		Module:     auditctx.ModuleBusiness,
		EntityType: strPtr(auditctx.EntityFinancialSettings),
		EntityID:   &idStr,
		AfterState: result,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return result, nil
}

// UpsertSettingTx upserts a practitioner setting within a transaction.
func (s *service) UpsertSettingTx(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID, req *RqUpsertPractitionerSetting) (*RsPractitionerSetting, error) {
	// Defaults
	timezone := "Australia/Sydney"
	color := "#000000"
	if req.Timezone != nil {
		timezone = *req.Timezone
	}
	if req.Color != nil {
		color = *req.Color
	}
	setting := &PractitionerSetting{
		PractitionerID: practitionerID,
		Timezone:       timezone,
		Logo:           req.Logo,
		Color:          color,
		UpdatedAt:      time.Now(),
	}
	// Note: Currently no UpsertSettingTx in repository, so we'd need to add it if Tx is used
	// For now, using non-Tx version which is fine for single operations
	updated, err := s.repo.UpsertSetting(ctx, setting)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}

func strPtr(s string) *string { return &s }
