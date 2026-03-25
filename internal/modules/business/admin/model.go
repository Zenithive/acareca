package admin

import (
	"time"

	"github.com/google/uuid"
)

type Admin struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"user_id"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// Local User struct to avoid import cycle
type User struct {
	ID        uuid.UUID  `db:"id"`
	Email     string     `db:"email"`
	Password  *string    `db:"password"`
	FirstName string     `db:"first_name"`
	LastName  string     `db:"last_name"`
	Phone     *string    `db:"phone"`
	Role      string     `db:"role"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type RqCreateAdmin struct {
	Email     string  `json:"email" validate:"required,email"`
	FirstName string  `json:"first_name" validate:"required"`
	LastName  string  `json:"last_name" validate:"required"`
	Password  string  `json:"password" validate:"required,min=8"`
	Phone     *string `json:"phone"      validate:"omitempty,e164"`
}

type RsAdmin struct {
	ID     uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
}

// User details for the nested response
type RsUserDetail struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Phone     *string   `json:"phone"`
}

// The final nested response structure
type RsAdminDetail struct {
	ID   uuid.UUID    `json:"id"`
	User RsUserDetail `json:"user"`
}
