package util

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
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
const EntityIDKey = "EntityID"

var validate = validator.New()

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
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(v); err != nil {
			return err
		}
	}

	// Sanitize query parameters - remove or replace invalid values like NaN, Infinity
	sanitizeQueryParams(c)

	if err := c.ShouldBindQuery(v); err != nil {
		return err
	}

	return validate.Struct(v)
}

// sanitizeQueryParams removes or replaces invalid numeric values in query parameters
func sanitizeQueryParams(c *gin.Context) {
	query := c.Request.URL.Query()
	modified := false

	for _, values := range query {
		for i, value := range values {
			// Replace NaN and Infinity with 0 to prevent parsing errors
			if value == "NaN" || value == "Infinity" || value == "-Infinity" {
				values[i] = "0"
				modified = true
			}
		}
	}

	if modified {
		c.Request.URL.RawQuery = query.Encode()
	}
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
	idVal, exists := c.Get(EntityIDKey)
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

func GetAccountantID(c *gin.Context) (uuid.UUID, bool) {
	idVal, exists := c.Get(EntityIDKey)
	if !exists {
		response.Error(c, http.StatusBadRequest, errors.New("accountant id not in context"))
		return uuid.Nil, false
	}
	id, ok := idVal.(uuid.UUID)
	if !ok {
		response.Error(c, http.StatusInternalServerError, errors.New("invalid accountant id type"))
		return uuid.Nil, false
	}
	return id, true
}

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

type RsList struct {
	Items interface{} `json:"items"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

func (rs *RsList) MapToList(data interface{}, total, page, limit int) {
	rs.Items = data
	rs.Total = total
	rs.Page = page
	rs.Limit = limit
}

func GetMonthRange(monthName string) (time.Time, time.Time, error) {
	now := time.Now()
	loc := now.Location()

	months := map[string]time.Month{
		"january":   time.January,
		"february":  time.February,
		"march":     time.March,
		"april":     time.April,
		"may":       time.May,
		"june":      time.June,
		"july":      time.July,
		"august":    time.August,
		"september": time.September,
		"october":   time.October,
		"november":  time.November,
		"december":  time.December,
	}

	month, ok := months[strings.ToLower(monthName)]
	if !ok {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid month")
	}

	start := time.Date(now.Year(), month, 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)

	return start, end, nil
}

func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	idVal, exists := c.Get(UserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, errors.New("user id not in context"))
		return uuid.Nil, false
	}

	idStr, ok := idVal.(string)
	if !ok {
		response.Error(c, http.StatusInternalServerError, errors.New("invalid user id type"))
		return uuid.Nil, false
	}

	// Parse the string into a UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, errors.New("failed to parse user uuid"))
		return uuid.Nil, false
	}

	return id, true
}

func GetEntityID(c *gin.Context) (uuid.UUID, bool) {
	idVal, exists := c.Get(EntityIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, nil)
		return uuid.Nil, false
	}
	id, ok := idVal.(uuid.UUID)
	if !ok || id == uuid.Nil {
		response.Error(c, http.StatusUnauthorized, nil)
		return uuid.Nil, false
	}
	return id, true
}
