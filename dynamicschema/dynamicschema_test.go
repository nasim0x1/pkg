package dynamicschema_test

import (
	"context"
	"testing"

	"github.com/nasim0x1/pkg/dynamicschema"
)

type mockRepo struct {
	fields map[string]dynamicschema.FieldSchema
}

func newMockRepo() *mockRepo {
	return &mockRepo{fields: make(map[string]dynamicschema.FieldSchema)}
}

func (m *mockRepo) GetFieldSchemas(ctx context.Context, entityType string, tenantID *string) ([]dynamicschema.FieldSchema, error) {
	var res []dynamicschema.FieldSchema
	for _, f := range m.fields {
		if f.EntityType == entityType {
			res = append(res, f)
		}
	}
	return res, nil
}

func (m *mockRepo) UpsertFieldSchema(ctx context.Context, fs dynamicschema.FieldSchema) error {
	m.fields[fs.FieldKey] = fs
	return nil
}

func (m *mockRepo) DeleteFieldSchema(ctx context.Context, id string) error {
	delete(m.fields, id)
	return nil
}

func TestValidation_Required(t *testing.T) {
	schema := &dynamicschema.Schema{
		EntityType: "user_profile",
		Fields: map[string]dynamicschema.FieldSchema{
			"nid": {
				FieldKey:  "nid",
				FieldType: dynamicschema.TypeString,
				Required:  true,
				IsActive:  true,
			},
		},
	}

	payload := map[string]interface{}{}
	res, err := schema.Validate(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected validation failure for missing required field")
	}
	if len(res.Errors) != 1 || res.Errors[0].Rule != "required" {
		t.Fatalf("expected required error rule, got: %+v", res.Errors)
	}
}

func TestValidation_TypesAndSemantics(t *testing.T) {
	minAge := 18.0
	maxAge := 100.0
	emailSemantic := "email"
	currencySemantic := "currency"

	schema := &dynamicschema.Schema{
		EntityType: "user_profile",
		Fields: map[string]dynamicschema.FieldSchema{
			"email": {
				FieldKey:     "email",
				FieldType:    dynamicschema.TypeString,
				SemanticType: &emailSemantic,
				Required:     true,
				IsActive:     true,
			},
			"age": {
				FieldKey:  "age",
				FieldType: dynamicschema.TypeNumber,
				ValidationRules: dynamicschema.ValidationRules{
					Min: &minAge,
					Max: &maxAge,
				},
				IsActive: true,
			},
			"currency": {
				FieldKey:     "currency",
				FieldType:    dynamicschema.TypeString,
				SemanticType: &currencySemantic,
				IsActive:     true,
			},
			"role_level": {
				FieldKey:  "role_level",
				FieldType: dynamicschema.TypeEnum,
				Options:   []string{"junior", "mid", "senior"},
				IsActive:  true,
			},
		},
	}

	// Test valid payload
	validPayload := map[string]interface{}{
		"email":      "test@example.com",
		"age":        25,
		"currency":   "BDT",
		"role_level": "senior",
	}
	res, err := schema.Validate(validPayload)
	if err != nil || !res.Valid {
		t.Fatalf("expected valid payload, got errors: %+v, err: %v", res.Errors, err)
	}

	// Test invalid payload
	invalidPayload := map[string]interface{}{
		"email":      "not-an-email",
		"age":        12, // < 18
		"currency":   "invalid_currency_code",
		"role_level": "super_admin", // not in options
	}
	res, err = schema.Validate(invalidPayload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected validation failures")
	}
	if len(res.Errors) != 4 {
		t.Fatalf("expected 4 validation errors, got %d: %+v", len(res.Errors), res.Errors)
	}
}

func TestSanitize(t *testing.T) {
	schema := &dynamicschema.Schema{
		EntityType: "user_profile",
		Fields: map[string]dynamicschema.FieldSchema{
			"allowed_field": {
				FieldKey:  "allowed_field",
				FieldType: dynamicschema.TypeString,
				IsActive:  true,
			},
			"defaulted_field": {
				FieldKey:     "defaulted_field",
				FieldType:    dynamicschema.TypeString,
				DefaultValue: "active",
				IsActive:     true,
			},
			"inactive_field": {
				FieldKey:  "inactive_field",
				FieldType: dynamicschema.TypeString,
				IsActive:  false,
			},
		},
	}

	payload := map[string]interface{}{
		"allowed_field":  "value1",
		"inactive_field": "hidden",
		"extra_field":    "should_be_stripped",
	}

	sanitized := schema.Sanitize(payload)

	if _, ok := sanitized["extra_field"]; ok {
		t.Errorf("extra_field was not stripped")
	}
	if _, ok := sanitized["inactive_field"]; ok {
		t.Errorf("inactive_field was not stripped")
	}
	if val, ok := sanitized["allowed_field"]; !ok || val != "value1" {
		t.Errorf("expected allowed_field to remain 'value1', got: %v", val)
	}
	if val, ok := sanitized["defaulted_field"]; !ok || val != "active" {
		t.Errorf("expected defaulted_field to be populated with 'active', got: %v", val)
	}
}

func TestYAML_SyncAndEngine(t *testing.T) {
	repo := newMockRepo()
	engine := dynamicschema.NewEngine(repo)

	yamlContent := `
entity_type: tenant
version: 1
fields:
  - field_key: tax_id
    field_type: string
    label: Tax Identification Number
    required: true
  - field_key: plan
    field_type: enum
    label: Subscription Plan
    options: ["free", "pro", "enterprise"]
    default_value: "free"
`

	ctx := context.Background()
	def, err := engine.SyncFromYAML(ctx, []byte(yamlContent))
	if err != nil {
		t.Fatalf("SyncFromYAML failed: %v", err)
	}
	if def.EntityType != "tenant" || len(def.Fields) != 2 {
		t.Fatalf("unexpected YAML sync result: %+v", def)
	}

	// Validate via Engine
	payload := map[string]interface{}{
		"tax_id": "TX-998822",
		"plan":   "enterprise",
	}
	sanitized, res, err := engine.ValidateAndSanitize(ctx, "tenant", nil, payload)
	if err != nil || !res.Valid {
		t.Fatalf("validation failed: %+v, err: %v", res, err)
	}
	if sanitized["tax_id"] != "TX-998822" || sanitized["plan"] != "enterprise" {
		t.Fatalf("unexpected sanitized output: %+v", sanitized)
	}
}
