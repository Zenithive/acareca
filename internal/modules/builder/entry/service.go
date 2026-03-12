package entry

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type IService interface {
	Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error)
	Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, formVersionID uuid.UUID, filter Filter) ([]*RsFormEntry, error)
}

type Service struct {
	repo IRepository
}

func NewService(repo IRepository) IService {
	return &Service{repo: repo}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error) {
	status := EntryStatusDraft
	if req.Status != "" {
		status = req.Status
	}
	var submittedAt *string
	if status == EntryStatusSubmitted {
		now := time.Now().UTC().Format(time.RFC3339)
		submittedAt = &now
	}
	e := &FormEntry{
		ID:            uuid.New(),
		FormVersionID: formVersionID,
		ClinicID:      req.ClinicID,
		SubmittedBy:   submittedBy,
		SubmittedAt:   submittedAt,
		Status:        status,
	}
	values := makeValues(e.ID, req.Values)
	if err := s.repo.Create(ctx, e, values); err != nil {
		return nil, err
	}
	created, vals, _ := s.repo.GetByID(ctx, e.ID)
	if created == nil {
		return e.ToRs(values), nil
	}
	return created.ToRs(vals), nil
}

// GetByID implements [IService].
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error) {
	e, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.ToRs(values), nil
}

// Update implements [IService].
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error) {
	existing, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != nil {
		existing.Status = *req.Status
		if *req.Status == EntryStatusSubmitted && existing.SubmittedAt == nil {
			now := time.Now().UTC().Format(time.RFC3339)
			existing.SubmittedAt = &now
		}
		existing.SubmittedBy = submittedBy
	}
	newValues := values
	if len(req.Values) > 0 {
		newValues = makeValues(existing.ID, req.Values)
	}
	if err := s.repo.Update(ctx, existing, newValues); err != nil {
		return nil, err
	}
	updated, vals, _ := s.repo.GetByID(ctx, id)
	if updated == nil {
		return existing.ToRs(values), nil
	}
	return updated.ToRs(vals), nil
}

// Delete implements [IService].
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List implements [IService].
func (s *Service) List(ctx context.Context, formVersionID uuid.UUID, filter Filter) ([]*RsFormEntry, error) {

	clinicID := filter.ClinicID
	if clinicID == nil {
		clinicID = &uuid.Nil
	}

	list, err := s.repo.ListByFormVersionID(ctx, formVersionID, clinicID)
	if err != nil {
		return nil, err
	}
	rs := make([]*RsFormEntry, 0, len(list))
	for _, e := range list {
		rs = append(rs, e.ToRs(nil))
	}
	return rs, nil
}

func makeValues(entryID uuid.UUID, rq []RqEntryValue) []*FormEntryValue {
	out := make([]*FormEntryValue, 0, len(rq))
	for _, v := range rq {
		fieldID, err := uuid.Parse(v.FormFieldID)
		if err != nil {
			continue
		}
		out = append(out, &FormEntryValue{
			ID:          uuid.New(),
			EntryID:     entryID,
			FormFieldID: fieldID,
			NetAmount:   v.NetAmount,
			GstAmount:   v.GstAmount,
			GrossAmount: v.GrossAmount,
		})
	}
	return out
}
