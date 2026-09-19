package rbac

import (
	"testing"
)

func TestRBACEvaluation(t *testing.T) {
	yamlData := []byte(`
version: 1
default_category: aarot
roles:
  sr:
    display_name: Sales Representative
    permissions:
      - order:create
      - order:read
      - collection:create
  manager:
    display_name: Ops Manager
    inherits:
      - sr
    permissions:
      - pricing:manage
      - master_po:compile
`)

	engine, err := LoadEngineFromYAML(yamlData)
	if err != nil {
		t.Fatalf("Failed to load engine: %v", err)
	}

	if !engine.HasPermission("sr", "order:create") {
		t.Errorf("Expected sr to have order:create")
	}

	if engine.HasPermission("sr", "pricing:manage") {
		t.Errorf("Did not expect sr to have pricing:manage")
	}

	// Manager inherits sr
	if !engine.HasPermission("manager", "order:create") {
		t.Errorf("Expected manager to inherit order:create from sr")
	}

	if !engine.HasPermission("manager", "pricing:manage") {
		t.Errorf("Expected manager to have pricing:manage")
	}

	// Super Admin always passes
	if !engine.HasPermission("super_admin", "any:random:permission") {
		t.Errorf("Expected super_admin to pass any permission")
	}
}
