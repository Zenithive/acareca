package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken      = errors.New("email already in use")
	ErrInvalidPassword = errors.New("invalid credentials")
)

type Service interface {
	Register(ctx context.Context, req *RqUser) (*RsUser, error)
	Login(ctx context.Context, req *RqLogin) (*RsToken, error)
}

type service struct {
	repo      Repository
	jwtSecret string
}

func NewService(repo Repository, jwtSecret string) Service {
	return &service{repo: repo, jwtSecret: jwtSecret}
}

func (s *service) Register(ctx context.Context, req *RqUser) (*RsUser, error) {
	if _, err := s.repo.FindByEmail(ctx, req.Email); err == nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := req.ToDBModel()
	u.ID = uuid.New().String()
	u.Password = string(hash)

	created, err := s.repo.CreateUser(ctx, u)
	if err != nil {
		return nil, err
	}

	return created.ToRsUser(), nil
}

func (s *service) Login(ctx context.Context, req *RqLogin) (*RsToken, error) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidPassword
	}

	if req.IsSuperadmin != nil && *req.IsSuperadmin {
		if user.IsSuperadmin == nil || !*user.IsSuperadmin {
			return nil, ErrInvalidPassword
		}
	}

	accessToken, err := s.signToken(user.ID, 15*time.Minute)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.signToken(user.ID, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &RsToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		IsSuperadmin: user.IsSuperadmin,
	}, nil
}

func (s *service) signToken(userID string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(ttl).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}
