package dynamicschema

import (
	"context"
	"fmt"
	"sync"
)

// SchemaRepository defines data persistence operations for dynamic schema fields.
type SchemaRepository interface {
	GetFieldSchemas(ctx context.Context, entityType string, tenantID *string) ([]FieldSchema, error)
	UpsertFieldSchema(ctx context.Context, fs FieldSchema) error
	DeleteFieldSchema(ctx context.Context, id string) error
}

// Engine coordinates schema caching, loading from repository, validation, and sanitization.
type Engine struct {
	repo  SchemaRepository
	mu    sync.RWMutex
	cache map[string]*Schema
}

// NewEngine creates a new Dynamic Schema Engine instance.
func NewEngine(repo SchemaRepository) *Engine {
	return &Engine{
		repo:  repo,
		cache: make(map[string]*Schema),
	}
}

func cacheKey(entityType string, tenantID *string) string {
	if tenantID != nil && *tenantID != "" {
		return fmt.Sprintf("%s:%s", entityType, *tenantID)
	}
	return entityType
}

// GetSchema loads the schema from memory cache, or fetches from repository if not cached.
func (e *Engine) GetSchema(ctx context.Context, entityType string, tenantID *string) (*Schema, error) {
	key := cacheKey(entityType, tenantID)

	e.mu.RLock()
	cached, found := e.cache[key]
	e.mu.RUnlock()

	if found && cached != nil {
		return cached, nil
	}

	// Fallback to loading from repository
	fields, err := e.repo.GetFieldSchemas(ctx, entityType, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schema fields for %s: %w", entityType, err)
	}

	// If tenant-specific schema returned nothing, optionally fallback to global (tenantID == nil)
	if len(fields) == 0 && tenantID != nil {
		fields, err = e.repo.GetFieldSchemas(ctx, entityType, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch fallback global schema fields for %s: %w", entityType, err)
		}
	}

	fieldMap := make(map[string]FieldSchema, len(fields))
	maxVersion := 1
	for _, f := range fields {
		if f.IsActive {
			fieldMap[f.FieldKey] = f
		}
		if f.SchemaVersion > maxVersion {
			maxVersion = f.SchemaVersion
		}
	}

	schema := &Schema{
		EntityType: entityType,
		TenantID:   tenantID,
		Version:    maxVersion,
		Fields:     fieldMap,
	}

	e.mu.Lock()
	e.cache[key] = schema
	e.mu.Unlock()

	return schema, nil
}

// InvalidateCache clears the cached schema for a specific entity type and tenant.
func (e *Engine) InvalidateCache(entityType string, tenantID *string) {
	key := cacheKey(entityType, tenantID)
	e.mu.Lock()
	delete(e.cache, key)
	e.mu.Unlock()
}

// InvalidateAll clears all cached schemas.
func (e *Engine) InvalidateAll() {
	e.mu.Lock()
	e.cache = make(map[string]*Schema)
	e.mu.Unlock()
}

// ValidateAndSanitize fetches the active schema, validates payload, and if valid returns the sanitized payload.
func (e *Engine) ValidateAndSanitize(ctx context.Context, entityType string, tenantID *string, payload map[string]interface{}) (map[string]interface{}, *ValidationResult, error) {
	schema, err := e.GetSchema(ctx, entityType, tenantID)
	if err != nil {
		return nil, nil, err
	}

	res, err := schema.Validate(payload)
	if err != nil {
		return nil, nil, err
	}

	if !res.Valid {
		return nil, res, nil
	}

	sanitized := schema.Sanitize(payload)
	return sanitized, res, nil
}
