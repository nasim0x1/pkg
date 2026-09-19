package dynamicschema

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
)

// YAMLSchemaDefinition maps the top-level structure of a schema YAML file.
type YAMLSchemaDefinition struct {
	EntityType string        `yaml:"entity_type"`
	TenantID   *string       `yaml:"tenant_id,omitempty"`
	Version    int           `yaml:"version"`
	Fields     []FieldSchema `yaml:"fields"`
}

// ParseYAML parses raw YAML bytes into a YAMLSchemaDefinition.
func ParseYAML(data []byte) (*YAMLSchemaDefinition, error) {
	var def YAMLSchemaDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse schema YAML: %w", err)
	}

	if def.EntityType == "" {
		return nil, fmt.Errorf("schema YAML missing required 'entity_type'")
	}

	if def.Version <= 0 {
		def.Version = 1
	}

	// Propagate entity_type and version to individual fields if not set
	for i := range def.Fields {
		if def.Fields[i].EntityType == "" {
			def.Fields[i].EntityType = def.EntityType
		}
		if def.Fields[i].TenantID == nil && def.TenantID != nil {
			def.Fields[i].TenantID = def.TenantID
		}
		if def.Fields[i].SchemaVersion <= 0 {
			def.Fields[i].SchemaVersion = def.Version
		}
		// Default IsActive to true if not explicitly false
		// (In YAML, omitted boolean is false, but fields in YAML are assumed active unless specified)
	}

	return &def, nil
}

// SyncFromYAML parses YAML content, upserts all field definitions into the repository, and refreshes the cache.
func (e *Engine) SyncFromYAML(ctx context.Context, yamlContent []byte) (*YAMLSchemaDefinition, error) {
	def, err := ParseYAML(yamlContent)
	if err != nil {
		return nil, err
	}

	for _, field := range def.Fields {
		// Ensure active is set
		field.IsActive = true
		field.EntityType = def.EntityType
		field.TenantID = def.TenantID
		field.SchemaVersion = def.Version

		if err := e.repo.UpsertFieldSchema(ctx, field); err != nil {
			return nil, fmt.Errorf("failed to upsert field schema '%s': %w", field.FieldKey, err)
		}
	}

	// Invalidate cache for this entity
	e.InvalidateCache(def.EntityType, def.TenantID)

	return def, nil
}
