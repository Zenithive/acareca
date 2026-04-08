package seed

// Request models
type RqSeedData struct {
	PractitionerID *string `json:"practitioner_id,omitempty"`
	NumClinics     int     `json:"num_clinics" validate:"required,min=1,max=100"`
	NumForms       int     `json:"num_forms" validate:"required,min=1,max=50"`
	NumFields      *int    `json:"num_fields,omitempty" validate:"omitempty,min=1,max=10"`
	Verbose        bool    `json:"verbose,omitempty"`
}

type RqCleanupData struct {
	PractitionerID string `json:"practitioner_id" validate:"required,uuid"`
}

// Response models
type RsSeedData struct {
	PractitionerID string         `json:"practitioner_id"`
	ClinicsCreated int            `json:"clinics_created"`
	FormsCreated   int            `json:"forms_created"`
	FieldsCreated  int            `json:"fields_created"`
	FormulasCreated int           `json:"formulas_created"`
	Duration       string         `json:"duration"`
	Details        []ClinicDetail `json:"details,omitempty"`
}

type ClinicDetail struct {
	ClinicID   string       `json:"clinic_id"`
	ClinicName string       `json:"clinic_name"`
	Forms      []FormDetail `json:"forms"`
}

type FormDetail struct {
	FormID   string `json:"form_id"`
	FormName string `json:"form_name"`
	Fields   int    `json:"fields"`
	Formulas int    `json:"formulas"`
}

type RsCleanupData struct {
	PractitionerID      string `json:"practitioner_id"`
	ClinicsDeleted      int    `json:"clinics_deleted"`
	FormsDeleted        int    `json:"forms_deleted"`
	FieldsDeleted       int    `json:"fields_deleted"`
	AddressesDeleted    int    `json:"addresses_deleted"`
	ContactsDeleted     int    `json:"contacts_deleted"`
	FormVersionsDeleted int    `json:"form_versions_deleted"`
	Duration            string `json:"duration"`
}
