package subscription

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	Create(ctx context.Context, practitionerID uuid.UUID, req *RqCreatePractitionerSubscription, tx *sqlx.Tx) (*RsPractitionerSubscription, error)
	GetByID(ctx context.Context, id int) (*RsPractitionerSubscription, error)
	ListByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f *Filter) (*util.RsList, error)
	Update(ctx context.Context, id int, req *RqUpdatePractitionerSubscription) (*RsPractitionerSubscription, error)
	Delete(ctx context.Context, id int) error

	GetActiveSubscription(ctx context.Context, practitionerID uuid.UUID) (*RsActiveSubscription, error)
	GetSubscriptionHistory(ctx context.Context, practitionerID uuid.UUID, f *Filter) (*util.RsList, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, practitionerID uuid.UUID, req *RqCreatePractitionerSubscription, tx *sqlx.Tx) (*RsPractitionerSubscription, error) {
	start, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return nil, err
	}
	sub := &PractitionerSubscription{
		PractitionerID: practitionerID,
		SubscriptionID: req.SubscriptionID,
		StartDate:      start,
		EndDate:        end,
		Status:         req.Status,
	}
	created, err := s.repo.Create(ctx, sub, tx)
	if err != nil {
		return nil, err
	}
	return created.ToRs(), nil
}

func (s *service) GetByID(ctx context.Context, id int) (*RsPractitionerSubscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return sub.ToRs(), nil
}

func (s *service) ListByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()
	list, err := s.repo.ListByPractitionerID(ctx, practitionerID, ft)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountByPractitionerID(ctx, practitionerID, ft)
	if err != nil {
		return nil, err
	}

	data := make([]*RsPractitionerSubscription, len(list))
	for i := range list {
		data[i] = list[i].ToRs()
	}

	var rsList util.RsList
	rsList.MapToList(data, total, *ft.Offset, *ft.Limit)
	return &rsList, nil
}

func (s *service) Update(ctx context.Context, id int, req *RqUpdatePractitionerSubscription) (*RsPractitionerSubscription, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	existing.UpdatedAt = time.Now()
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}

func (s *service) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) GetActiveSubscription(ctx context.Context, practitionerID uuid.UUID) (*RsActiveSubscription, error) {
	sub, err := s.repo.GetActiveSubscription(ctx, practitionerID)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

/*
func (s *service) GetSubscriptionHistory(ctx context.Context, practitionerID uuid.UUID, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()
	list, err := s.repo.ListHistoryByPractitionerID(ctx, practitionerID, ft)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountByPractitionerID(ctx, practitionerID, ft)
	if err != nil {
		return nil, err
	}

	data := make([]*RsPractitionerSubscription, len(list))
	for i := range list {
		data[i] = list[i].ToRs()
	}

	var rsList util.RsList
	rsList.MapToList(data, total, ft.Offset, ft.Limit)
	return &rsList, nil
}
*/

func (s *service) GetSubscriptionHistory(ctx context.Context, practitionerID uuid.UUID, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()

	// 1. Call the new specialized repository method
	// list is now []*RsActiveSubscription
	list, err := s.repo.ListHistoryByPractitionerID(ctx, practitionerID, ft)
	if err != nil {
		return nil, err
	}

	// 2. Get the total count for pagination
	total, err := s.repo.CountByPractitionerID(ctx, practitionerID, ft)
	if err != nil {
		return nil, err
	}

	var rsList util.RsList
	rsList.MapToList(list, total, *ft.Offset, *ft.Limit)

	return &rsList, nil
}
