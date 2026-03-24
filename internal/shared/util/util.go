package util

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

const UserIDKey = "userID"
const PractitionerIDKey = "practitionerID"

func NewUUID() string {
	return uuid.New().String()
}

func GenerateHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CompareHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func BindAndValidate(c *gin.Context, v any) error {
	validate := validator.New()
	if err := c.ShouldBindJSON(v); err != nil {
		return err
	}
	if err := validate.Struct(v); err != nil {
		return err
	}

	if err := c.ShouldBindQuery(v); err != nil {
		return err
	}

	return nil
}

type CustomClaims struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func SignToken(userID string, id string, role string, ttl time.Duration, jwtSecret string) (string, error) {
	now := time.Now()

	claims := CustomClaims{
		ID:   id,
		Role: role,

		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(jwtSecret))
}

func Round(value float64, precision int) float64 {
	multiplier := math.Pow(10, float64(precision))
	return math.Round(value*multiplier) / multiplier
}

func ParseUUID(s string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, err
	}
	return parsed, nil
}

func GetPractitionerID(c *gin.Context) (uuid.UUID, bool) {
	idVal, exists := c.Get(PractitionerIDKey)
	if !exists {
		response.Error(c, http.StatusBadRequest, errors.New("practitioner id not in context"))
		return uuid.Nil, false
	}
	id, ok := idVal.(uuid.UUID)
	if !ok {
		response.Error(c, http.StatusInternalServerError, errors.New("invalid practitioner id type"))
		return uuid.Nil, false
	}
	return id, true
}

// const ClinicIDKey = "clinicID"

// func GetClinicID(c *gin.Context) (uuid.UUID, bool) {
// 	idVal, exists := c.Get(ClinicIDKey)
// 	if !exists {
// 		response.Error(c, http.StatusBadRequest, errors.New("clinic id not in context"))
// 		return uuid.Nil, false
// 	}
// 	id, ok := idVal.(uuid.UUID)
// 	if !ok {
// 		response.Error(c, http.StatusInternalServerError, errors.New("invalid clinic id type"))
// 		return uuid.Nil, false
// 	}
// 	return id, true
// }

func ParseIntID(c *gin.Context, param string) (int, bool) {
	id, err := strconv.Atoi(c.Param(param))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return 0, false
	}
	return id, true
}

func ParseUuidID(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return uuid.Nil, false
	}
	return id, true
}

// RunInTransaction starts a transaction on db, calls fn(ctx, tx), and commits if fn returns nil; otherwise rolls back.
func RunInTransaction(ctx context.Context, db *sqlx.DB, fn func(ctx context.Context, tx *sqlx.Tx) error) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
