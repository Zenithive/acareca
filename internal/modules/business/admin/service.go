package admin

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	CreateAdmin(ctx context.Context, req *RqCreateAdmin) (*RsAdmin, error)
	GetAdminByUserID(ctx context.Context, userID uuid.UUID) (*RsAdmin, error)
	GetAdminByID(ctx context.Context, id uuid.UUID) (*RsAdminDetail, error)
}

type service struct {
	repo Repository
	db   *sqlx.DB
}

func NewService(repo Repository, db *sqlx.DB) IService {
	return &service{repo: repo, db: db}
}

func (s *service) CreateAdmin(ctx context.Context, req *RqCreateAdmin) (*RsAdmin, error) {
	var createdAdmin *Admin

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		hashed, err := util.GenerateHash(req.Password)
		if err != nil {
			return err
		}

		userModel := &User{
			Email:     req.Email,
			Password:  &hashed,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Phone:     req.Phone,
			Role:      util.RoleAdmin,
		}

		u, err := s.repo.CreateUser(ctx, userModel, tx)
		if err != nil {
			return err
		}

		adm := &Admin{UserID: u.ID}
		createdAdmin, err = s.repo.CreateAdmin(ctx, adm, tx)
		return err
	})

	if err != nil {
		return nil, err
	}

	return &RsAdmin{ID: createdAdmin.ID, UserID: createdAdmin.UserID}, nil
}

func (s *service) GetAdminByUserID(ctx context.Context, userID uuid.UUID) (*RsAdmin, error) {
	adm, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &RsAdmin{ID: adm.ID, UserID: adm.UserID}, nil
}

func (s *service) GetAdminByID(ctx context.Context, id uuid.UUID) (*RsAdminDetail, error) {
	return s.repo.FindByID(ctx, id)
}
