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

type AuthProvider struct {
	ID             string     `db:"id"`
	UserID         string     `db:"user_id"`
	Provider       string     `db:"provider"`
	AccessToken    *string    `db:"access_token"`
	RefreshToken   *string    `db:"refresh_token"`
	TokenExpiresAt *time.Time `db:"token_expires_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

type Session struct {
	ID           string     `db:"id"`
	UserID       string     `db:"user_id"`
	RefreshToken string     `db:"refresh_token"`
	UserAgent    *string    `db:"user_agent"`
	IPAddress    *string    `db:"ip_address"`
	ExpiresAt    time.Time  `db:"expires_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

type RqUser struct {
	Email        string  `json:"email"         validate:"required,email"`
	Password     string  `json:"password"      validate:"required,min=8"`
	FirstName    string  `json:"first_name"    validate:"required"`
	LastName     string  `json:"last_name"     validate:"required"`
	Phone        *string `json:"phone"         validate:"omitempty,e164"`
	IsSuperadmin *bool   `json:"is_superadmin" validate:"omitempty"`
}

func (r *RqUser) ToDBModel() *User {
	return &User{
		Email:        r.Email,
		FirstName:    r.FirstName,
		LastName:     r.LastName,
		Phone:        r.Phone,
		IsSuperadmin: r.IsSuperadmin,
	}
}

type RqLogin struct {
	Email        string `json:"email"         validate:"required,email"`
	Password     string `json:"password"      validate:"required"`
	IsSuperadmin *bool  `json:"is_superadmin" validate:"omitempty"`
}

// ── Response models ───────────────────────────────────────────────────────────

type RsToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IsSuperadmin *bool  `json:"is_superadmin"`
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

type RsGoogleAuthURL struct {
	URL string `json:"url"`
}

// GoogleUserInfo is the response from the Google userinfo endpoint.
type GoogleUserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"given_name"`
	LastName  string `json:"family_name"`
}
