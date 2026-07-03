package rendering

import (
	"encoding/json"
	"fmt"
)

// DataMapper handles conversion of structs to maps for template rendering
type DataMapper struct{}

// NewDataMapper creates a new DataMapper instance
func NewDataMapper() *DataMapper {
	return &DataMapper{}
}

// ToMap converts any struct to a map[string]interface{} using JSON marshaling
func (m *DataMapper) ToMap(data interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	return result, nil
}

// MergeData combines multiple maps into one, with later maps overriding earlier ones
func (m *DataMapper) MergeData(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
