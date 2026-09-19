package dynamicschema

import (
	"time"
)

type FieldType string

const (
	TypeString    FieldType = "string"
	TypeNumber    FieldType = "number"
	TypeBoolean   FieldType = "boolean"
	TypeDate      FieldType = "date"
	TypeEnum      FieldType = "enum"
	TypeArray     FieldType = "array"
	TypeReference FieldType = "reference"
)

type SemanticType string

const (
	SemanticEmail    SemanticType = "email"
	SemanticPhone    SemanticType = "phone"
	SemanticURL      SemanticType = "url"
	SemanticCurrency SemanticType = "currency"
)

type ValidationRules struct {
	Min       *float64 `json:"min,omitempty" yaml:"min,omitempty"`
	Max       *float64 `json:"max,omitempty" yaml:"max,omitempty"`
	Pattern   string   `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	MinLength *int     `json:"min_length,omitempty" yaml:"min_length,omitempty"`
	MaxLength *int     `json:"max_length,omitempty" yaml:"max_length,omitempty"`
	ItemType  string   `json:"item_type,omitempty" yaml:"item_type,omitempty"` // For arrays: string, number, etc.
}

type FieldSchema struct {
	ID              string          `json:"id" yaml:"id,omitempty"`
	EntityType      string          `json:"entity_type" yaml:"entity_type"`
	TenantID        *string         `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	FieldKey        string          `json:"field_key" yaml:"field_key"`
	FieldType       FieldType       `json:"field_type" yaml:"field_type"`
	SemanticType    *string         `json:"semantic_type,omitempty" yaml:"semantic_type,omitempty"`
	Label           string          `json:"label" yaml:"label"`
	Description     string          `json:"description,omitempty" yaml:"description,omitempty"`
	Required        bool            `json:"required" yaml:"required"`
	DefaultValue    interface{}     `json:"default_value,omitempty" yaml:"default_value,omitempty"`
	ValidationRules ValidationRules `json:"validation_rules,omitempty" yaml:"validation_rules,omitempty"`
	Options         []string        `json:"options,omitempty" yaml:"options,omitempty"`
	DisplayOrder    int             `json:"display_order" yaml:"display_order"`
	SchemaVersion   int             `json:"schema_version" yaml:"schema_version"`
	IsActive        bool            `json:"is_active" yaml:"is_active"`
	CreatedAt       time.Time       `json:"created_at,omitempty" yaml:"created_at,omitempty"`
}

type Schema struct {
	EntityType string                 `json:"entity_type"`
	TenantID   *string                `json:"tenant_id,omitempty"`
	Version    int                    `json:"version"`
	Fields     map[string]FieldSchema `json:"fields"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

func (vr *ValidationResult) AddError(field, rule, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, ValidationError{
		Field:   field,
		Rule:    rule,
		Message: message,
	})
}
