package auth

import "time"

type User struct {
	ID           string     `db:"id"`
	Email        string     `db:"email"`
	Password     string     `db:"password"`
	FirstName    string     `db:"first_name"`
	LastName     string     `db:"last_name"`
	Phone        *string    `db:"phone"`
	IsSuperadmin *bool      `db:"is_superadmin"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

type RqUser struct {
	Email        string  `json:"email" validate:"required,email"`
	Password     string  `json:"password" validate:"required,min=8"`
	FirstName    string  `json:"first_name" validate:"required"`
	LastName     string  `json:"last_name" validate:"required"`
	Phone        *string `json:"phone" validate:"omitempty,e164"`
	IsSuperadmin *bool   `json:"is_superadmin" validate:"omitempty"`
}

func (r *RqUser) ToDBModel() *User {
	return &User{
		Email:        r.Email,
		Password:     r.Password,
		FirstName:    r.FirstName,
		LastName:     r.LastName,
		Phone:        r.Phone,
		IsSuperadmin: r.IsSuperadmin,
	}
}

type RqLogin struct {
	Email        string `json:"email"    validate:"required,email"`
	Password     string `json:"password" validate:"required"`
	IsSuperadmin *bool  `json:"is_superadmin" validate:"omitempty"`
}

type RsToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IsSuperadmin *bool  `json:"is_superadmin"`
}

func (u *User) ToRsUser() *RsUser {
	return &RsUser{
		ID:           u.ID,
		Email:        u.Email,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		Phone:        u.Phone,
		IsSuperadmin: u.IsSuperadmin,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

type RsUser struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Phone        *string   `json:"phone,omitempty"`
	IsSuperadmin *bool     `json:"is_superadmin"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
