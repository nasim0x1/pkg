package rbac

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

const DefaultCatalogYAML = `
version: 1
default_category: general
default_role: user
role_levels:
  system:
    display_name: System Administrator
  management:
    display_name: Management
  general:
    display_name: General Staff
  self:
    display_name: End User
roles:
  super_admin:
    display_name: Super Administrator
    level: system
    permissions:
      - "*"
  admin:
    display_name: Administrator
    level: system
    permissions:
      - "user:read"
      - "user:manage_roles"
      - "tenant:manage"
      - "schema:manage"
      - "profile:manage"
  manager:
    display_name: Manager
    level: management
    permissions:
      - "user:read"
      - "profile:manage"
  member:
    display_name: Organization Member
    level: general
    permissions:
      - "profile:manage"
  user:
    display_name: User
    level: self
    permissions:
      - "profile:manage"
`

type RoleDefinition struct {
	DisplayName string   `yaml:"display_name"`
	Description string   `yaml:"description"`
	Level       string   `yaml:"level"` // system, management, general, self
	Permissions []string `yaml:"permissions"`
	Inherits    []string `yaml:"inherits,omitempty"`
}

type Catalog struct {
	Version         int                       `yaml:"version"`
	DefaultCategory string                    `yaml:"default_category"`
	DefaultRole     string                    `yaml:"default_role"` // e.g. "user"
	Roles           map[string]RoleDefinition `yaml:"roles"`
}

type Engine struct {
	mu              sync.RWMutex
	catalog         *Catalog
	rolePermissions map[string]map[string]bool
	defaultRole     string
}

func NewEngine() *Engine {
	engine := &Engine{
		rolePermissions: make(map[string]map[string]bool),
		defaultRole:     "user",
	}
	_ = engine.UpdateCatalog([]byte(DefaultCatalogYAML))
	return engine
}

func LoadEngineFromYAML(yamlContent []byte) (*Engine, error) {
	var catalog Catalog
	if err := yaml.Unmarshal(yamlContent, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse rbac catalog yaml: %w", err)
	}

	engine := NewEngine()
	engine.catalog = &catalog
	if catalog.DefaultRole != "" {
		engine.defaultRole = catalog.DefaultRole
	} else {
		for rName, rDef := range catalog.Roles {
			if rDef.Level == "self" {
				engine.defaultRole = rName
				break
			}
		}
	}
	engine.rebuildPermissions()
	return engine, nil
}

func LoadEngineFromFile(filePath string) (*Engine, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rbac catalog file %s: %w", filePath, err)
	}
	return LoadEngineFromYAML(data)
}

func (e *Engine) rebuildPermissions() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.rolePermissions = make(map[string]map[string]bool)
	if e.catalog == nil {
		return
	}

	if e.catalog.DefaultRole != "" {
		e.defaultRole = e.catalog.DefaultRole
	}

	for roleName, def := range e.catalog.Roles {
		permSet := make(map[string]bool)
		for _, p := range def.Permissions {
			permSet[p] = true
		}

		// Handle inherited roles
		for _, inheritedRole := range def.Inherits {
			if parentDef, ok := e.catalog.Roles[inheritedRole]; ok {
				for _, p := range parentDef.Permissions {
					permSet[p] = true
				}
			}
		}

		e.rolePermissions[roleName] = permSet
	}
}

func (e *Engine) GetDefaultRole() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.defaultRole != "" {
		return e.defaultRole
	}
	return "user"
}

func (e *Engine) HasPermission(role, permission string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Super Admin wildcard
	if role == "super_admin" || role == "admin" {
		return true
	}

	perms, ok := e.rolePermissions[role]
	if !ok {
		return false
	}

	return perms[permission] || perms["*"]
}

func (e *Engine) GetRolePermissions(role string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	perms, ok := e.rolePermissions[role]
	if !ok {
		return nil
	}

	result := make([]string, 0, len(perms))
	for p := range perms {
		result = append(result, p)
	}
	return result
}

func (e *Engine) GetAllRoles() map[string]RoleDefinition {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.catalog == nil {
		return nil
	}
	return e.catalog.Roles
}

func (e *Engine) GetAllPermissions() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.catalog == nil {
		return nil
	}
	permMap := make(map[string]bool)
	for _, def := range e.catalog.Roles {
		for _, p := range def.Permissions {
			permMap[p] = true
		}
	}
	res := make([]string, 0, len(permMap))
	for p := range permMap {
		res = append(res, p)
	}
	sort.Strings(res)
	return res
}

func (e *Engine) UpdateCatalog(yamlContent []byte) error {
	var catalog Catalog
	if err := yaml.Unmarshal(yamlContent, &catalog); err != nil {
		return fmt.Errorf("failed to parse updated rbac catalog: %w", err)
	}

	e.catalog = &catalog
	e.rebuildPermissions()
	return nil
}
