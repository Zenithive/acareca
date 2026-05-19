package auth

type RqClinic struct {
	DocumentId *string `json:"document_id"`
	Name       string  `json:"name" validate:"required, min=3, max=100"`
	Email      string  `json:"email" validate:"required,email"`
	Password   string  `json:"password" validate:"required,min=8"`
}

type RsClinic struct {
	ID         string  `json:"id"`
	DocumentId *string `json:"document_id,omitempty"`
	Name       string  `json:"name"`
	Email      string  `json:"email"`
}
