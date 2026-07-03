package domain

// Constants for size limits
const (
	MaxTemplateSizeBytes = 5 * 1024 * 1024  // 5MB per template
	MaxTotalSizeBytes    = 10 * 1024 * 1024 // 10MB total
	MaxTemplateCount     = 10                // Max templates per request
)

// TemplateMethod defines invoice method configurations
type TemplateMethod struct {
	Name          string
	TemplateNames []string
	PageOrder     map[string]int
}

var (
	// MethodSFAClinicCollects - Method A: SFA Clinic Collects
	MethodSFAClinicCollects = TemplateMethod{
		Name: "SFA_CLINIC_COLLECTS",
		TemplateNames: []string{
			"Calculation Statement",
			"Tax Invoice",
			"Remittance Advice",
		},
		PageOrder: map[string]int{
			"Calculation Statement": 1,
			"Tax Invoice":           2,
			"Remittance Advice":     3,
		},
	}

	// MethodSFADentistCollects - Method B: SFA Dentist Collects
	MethodSFADentistCollects = TemplateMethod{
		Name: "SFA_DENTIST_COLLECTS",
		TemplateNames: []string{
			"Calculation Statement",
			"Tax Invoice",
		},
		PageOrder: map[string]int{
			"Calculation Statement": 1,
			"Tax Invoice":           2,
		},
	}

	// MethodIndependentContractor - Method C: Independent Contractor
	MethodIndependentContractor = TemplateMethod{
		Name: "INDEPENDENT_CONTRACTOR",
		TemplateNames: []string{
			"Calculation Statement",
			"Recipient Created Tax Invoice",
			"Remittance Advice",
		},
		PageOrder: map[string]int{
			"Calculation Statement":         1,
			"Recipient Created Tax Invoice": 2,
			"Remittance Advice":             3,
		},
	}


	// MethodRegistry maps method names to configurations
	MethodRegistry = map[string]TemplateMethod{
		"SFA_CLINIC_COLLECTS":    MethodSFAClinicCollects,
		"SFA_DENTIST_COLLECTS":   MethodSFADentistCollects,
		"INDEPENDENT_CONTRACTOR": MethodIndependentContractor,
	}
)

// GetMethod returns method configuration by name
func GetMethod(name string) (TemplateMethod, bool) {
	method, ok := MethodRegistry[name]
	return method, ok
}

// GetPageOrder returns page order for a method (with fallback to default)
func GetPageOrder(methodName string) map[string]int {
	method, ok := GetMethod(methodName)
	if !ok {
		// Fallback to default method
		return MethodSFAClinicCollects.PageOrder
	}
	return method.PageOrder
}

// GetTemplateNames returns template names for a method
func GetTemplateNames(methodName string) []string {
	method, ok := GetMethod(methodName)
	if !ok {
		return nil
	}
	return method.TemplateNames
}

// AllMethods returns all available method names
func AllMethods() []string {
	methods := make([]string, 0, len(MethodRegistry))
	for name := range MethodRegistry {
		methods = append(methods, name)
	}
	return methods
}
