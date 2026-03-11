package version

type IService interface {
	Create(formVersion *FormVersion) error
	Get(id string) (*RsFormVersion, error)
	Update(formVersion *FormVersion) error
	Delete(id string) error
}

type service struct {
	repo IRepository
}

func NewService(repo IRepository) IService {
	return &service{repo: repo}
}

// Create implements [IService].
func (s *service) Create(formVersion *FormVersion) error {
	panic("unimplemented")
}

// Delete implements [IService].
func (s *service) Delete(id string) error {
	panic("unimplemented")
}

// Get implements [IService].
func (s *service) Get(id string) (*RsFormVersion, error) {
	panic("unimplemented")
}

// Update implements [IService].
func (s *service) Update(formVersion *FormVersion) error {
	panic("unimplemented")
}
