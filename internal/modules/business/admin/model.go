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
	Email     string  `json:"email" binding:"required,email"`
	FirstName string  `json:"first_name" binding:"required"`
	LastName  string  `json:"last_name" binding:"required"`
	Password  string  `json:"password" binding:"required,min=8"`
	Phone     *string `json:"phone" binding:"omitempty"`
}

type RsAdmin struct {
	ID     uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
}

type RsUserDetail struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	Phone     *string   `json:"phone" db:"phone"`
}

type RsAdminDetail struct {
	ID   uuid.UUID    `json:"id" db:"id"`
	User RsUserDetail `json:"user" db:"user"` // Enabled sqlx struct embedding support
}
