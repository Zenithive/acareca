package rendering

import (
	"encoding/json"
	"fmt"
)

type DataMapper struct{}

func NewDataMapper() *DataMapper {
	return &DataMapper{}
}

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

func (m *DataMapper) MergeData(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// ToMapReflect converts struct to map using reflection (alternative approach)
// func (m *DataMapper) ToMapReflect(data interface{}) (map[string]interface{}, error) {
// 	result := make(map[string]interface{})

// 	v := reflect.ValueOf(data)
// 	if v.Kind() == reflect.Ptr {
// 		v = v.Elem()
// 	}

// 	if v.Kind() != reflect.Struct {
// 		return nil, fmt.Errorf("data must be a struct, got %s", v.Kind())
// 	}

// 	t := v.Type()
// 	for i := 0; i < v.NumField(); i++ {
// 		field := t.Field(i)
// 		value := v.Field(i)

// 		// Get json tag or use field name
// 		key := field.Name
// 		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
// 			key = jsonTag
// 		}

// 		// Skip unexported fields
// 		if !field.IsExported() {
// 			continue
// 		}

// 		result[key] = value.Interface()
// 	}

// 	return result, nil
// }

// // ValidateData checks if required fields are present in the data map
// func (m *DataMapper) ValidateData(data map[string]interface{}, requiredFields []string) error {
// 	for _, field := range requiredFields {
// 		if _, exists := data[field]; !exists {
// 			return fmt.Errorf("required field %q not found in data", field)
// 		}
// 	}
// 	return nil
// }

// // SanitizeData removes nil values and empty strings from the map
// func (m *DataMapper) SanitizeData(data map[string]interface{}) map[string]interface{} {
// 	result := make(map[string]interface{})
// 	for k, v := range data {
// 		if v == nil {
// 			continue
// 		}
// 		if str, ok := v.(string); ok && str == "" {
// 			continue
// 		}
// 		result[k] = v
// 	}
// 	return result
// }
