package dynamicschema

import (
	"encoding/json"
	"fmt"
	"math"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var phoneRegex = regexp.MustCompile(`^\+?[0-9]{7,15}$`)

// Validate performs hand-written, stdlib-only validation against the defined fields.
func (s *Schema) Validate(payload map[string]interface{}) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:  true,
		Errors: make([]ValidationError, 0),
	}

	if s == nil || len(s.Fields) == 0 {
		return result, nil
	}

	for key, field := range s.Fields {
		if !field.IsActive {
			continue
		}

		val, exists := payload[key]

		// 1. Required Check
		if !exists || val == nil {
			if field.Required {
				result.AddError(key, "required", fmt.Sprintf("field '%s' is required", key))
			}
			continue
		}

		// Handle empty string as non-provided if required
		if strVal, isStr := val.(string); isStr && strings.TrimSpace(strVal) == "" {
			if field.Required {
				result.AddError(key, "required", fmt.Sprintf("field '%s' cannot be blank", key))
			}
			continue
		}

		// 2. Type Checking & Rule Validation
		switch field.FieldType {
		case TypeString:
			str, ok := val.(string)
			if !ok {
				result.AddError(key, "type", fmt.Sprintf("field '%s' must be a string", key))
				continue
			}
			s.validateStringRules(key, str, field, result)

		case TypeNumber:
			num, ok := toFloat64(val)
			if !ok {
				result.AddError(key, "type", fmt.Sprintf("field '%s' must be a valid number", key))
				continue
			}
			s.validateNumberRules(key, num, field, result)

		case TypeBoolean:
			_, ok := val.(bool)
			if !ok {
				result.AddError(key, "type", fmt.Sprintf("field '%s' must be a boolean", key))
			}

		case TypeDate:
			if !isValidDate(val) {
				result.AddError(key, "date_format", fmt.Sprintf("field '%s' must be a valid date (YYYY-MM-DD or RFC3339 timestamp)", key))
			}

		case TypeEnum:
			str, ok := val.(string)
			if !ok {
				result.AddError(key, "type", fmt.Sprintf("field '%s' must be a string enum value", key))
				continue
			}
			if !containsString(field.Options, str) {
				result.AddError(key, "enum_options", fmt.Sprintf("field '%s' must be one of: %s", key, strings.Join(field.Options, ", ")))
			}

		case TypeArray:
			arr, ok := val.([]interface{})
			if !ok {
				// Handle []string if passed in Go maps
				if strArr, okStr := val.([]string); okStr {
					arr = make([]interface{}, len(strArr))
					for i, v := range strArr {
						arr[i] = v
					}
				} else {
					result.AddError(key, "type", fmt.Sprintf("field '%s' must be an array", key))
					continue
				}
			}

			if field.ValidationRules.MinLength != nil && len(arr) < *field.ValidationRules.MinLength {
				result.AddError(key, "min_items", fmt.Sprintf("field '%s' must have at least %d items", key, *field.ValidationRules.MinLength))
			}
			if field.ValidationRules.MaxLength != nil && len(arr) > *field.ValidationRules.MaxLength {
				result.AddError(key, "max_items", fmt.Sprintf("field '%s' must have at most %d items", key, *field.ValidationRules.MaxLength))
			}

			// Validate enum array or item types
			if len(field.Options) > 0 {
				for i, item := range arr {
					itemStr, ok := item.(string)
					if !ok || !containsString(field.Options, itemStr) {
						result.AddError(fmt.Sprintf("%s[%d]", key, i), "enum_options", fmt.Sprintf("array item must be one of: %s", strings.Join(field.Options, ", ")))
					}
				}
			}

		case TypeReference:
			str, ok := val.(string)
			if !ok || strings.TrimSpace(str) == "" {
				result.AddError(key, "reference", fmt.Sprintf("field '%s' must be a valid reference identifier string", key))
			}
		}

		// 3. Semantic Type Validation
		if field.SemanticType != nil && *field.SemanticType != "" {
			s.validateSemanticType(key, val, *field.SemanticType, result)
		}
	}

	return result, nil
}

// Sanitize strips any keys from payload that are not defined or not active in the schema.
func (s *Schema) Sanitize(payload map[string]interface{}) map[string]interface{} {
	if s == nil || len(s.Fields) == 0 {
		return make(map[string]interface{})
	}

	sanitized := make(map[string]interface{})
	for k, v := range payload {
		field, exists := s.Fields[k]
		if exists && field.IsActive {
			sanitized[k] = v
		}
	}

	// Apply default values for missing fields
	for k, field := range s.Fields {
		if field.IsActive && field.DefaultValue != nil {
			if _, exists := sanitized[k]; !exists {
				sanitized[k] = field.DefaultValue
			}
		}
	}

	return sanitized
}

func (s *Schema) validateStringRules(key, val string, field FieldSchema, res *ValidationResult) {
	length := len(val)
	if field.ValidationRules.MinLength != nil && length < *field.ValidationRules.MinLength {
		res.AddError(key, "min_length", fmt.Sprintf("field '%s' must be at least %d characters", key, *field.ValidationRules.MinLength))
	}
	if field.ValidationRules.MaxLength != nil && length > *field.ValidationRules.MaxLength {
		res.AddError(key, "max_length", fmt.Sprintf("field '%s' must not exceed %d characters", key, *field.ValidationRules.MaxLength))
	}
	if field.ValidationRules.Pattern != "" {
		matched, err := regexp.MatchString(field.ValidationRules.Pattern, val)
		if err != nil || !matched {
			res.AddError(key, "pattern", fmt.Sprintf("field '%s' does not match required pattern: %s", key, field.ValidationRules.Pattern))
		}
	}
}

func (s *Schema) validateNumberRules(key string, val float64, field FieldSchema, res *ValidationResult) {
	if field.ValidationRules.Min != nil && val < *field.ValidationRules.Min {
		res.AddError(key, "min_value", fmt.Sprintf("field '%s' must be greater than or equal to %g", key, *field.ValidationRules.Min))
	}
	if field.ValidationRules.Max != nil && val > *field.ValidationRules.Max {
		res.AddError(key, "max_value", fmt.Sprintf("field '%s' must be less than or equal to %g", key, *field.ValidationRules.Max))
	}
}

func (s *Schema) validateSemanticType(key string, val interface{}, semType string, res *ValidationResult) {
	strVal, isStr := val.(string)
	if !isStr {
		return
	}

	switch SemanticType(semType) {
	case SemanticEmail:
		_, err := mail.ParseAddress(strVal)
		if err != nil || !strings.Contains(strVal, "@") || !strings.Contains(strVal, ".") {
			res.AddError(key, "semantic_email", fmt.Sprintf("field '%s' must be a valid email address", key))
		}

	case SemanticPhone:
		cleaned := strings.ReplaceAll(strVal, " ", "")
		cleaned = strings.ReplaceAll(cleaned, "-", "")
		if !phoneRegex.MatchString(cleaned) {
			res.AddError(key, "semantic_phone", fmt.Sprintf("field '%s' must be a valid phone number (7-15 digits)", key))
		}

	case SemanticURL:
		parsed, err := url.ParseRequestURI(strVal)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			res.AddError(key, "semantic_url", fmt.Sprintf("field '%s' must be a valid HTTP or HTTPS URL", key))
		}

	case SemanticCurrency:
		// ISO currency code (3 uppercase letters, e.g. BDT, USD, EUR)
		if len(strVal) != 3 || strings.ToUpper(strVal) != strVal {
			res.AddError(key, "semantic_currency", fmt.Sprintf("field '%s' must be a 3-letter uppercase currency code (e.g. BDT, USD)", key))
		}
	}
}

func toFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func isValidDate(val interface{}) bool {
	switch v := val.(type) {
	case time.Time:
		return true
	case string:
		formats := []string{
			time.RFC3339,
			"2006-01-02",
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02 15:04:05",
		}
		for _, layout := range formats {
			if _, err := time.Parse(layout, v); err == nil {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func containsString(slice []string, val string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, val) {
			return true
		}
	}
	return false
}
