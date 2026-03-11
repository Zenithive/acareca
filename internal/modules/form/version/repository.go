package version

import "github.com/jmoiron/sqlx"

type IRepository interface {
	Create(formVersion *FormVersion) error
	Get(id string) (*FormVersion, error)
	Update(formVersion *FormVersion) error
	Delete(id string) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &repository{db: db}
}

// Create implements [IRepository].
func (r *repository) Create(formVersion *FormVersion) error {
	panic("unimplemented")
}

// Delete implements [IRepository].
func (r *repository) Delete(id string) error {
	panic("unimplemented")
}

// Get implements [IRepository].
func (r *repository) Get(id string) (*FormVersion, error) {
	panic("unimplemented")
}

// Update implements [IRepository].
func (r *repository) Update(formVersion *FormVersion) error {
	panic("unimplemented")
}
