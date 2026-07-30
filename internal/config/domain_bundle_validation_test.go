package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomainBundleLoadsAllowRepairableOutOfRangeColorSlots(t *testing.T) {
	manager, dataDir := seededManager(t)
	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("load seeded domains: %v", err)
	}
	domains.Catalog.Projects = []CatalogProject{{
		ID:        "repairable",
		Slug:      "repairable",
		ColorSlot: ProjectColorSlotCount + 1,
	}}
	writeRawDomainPayload(t, dataDir, domains.Catalog.WorkspaceID, domainCatalog, domains.Catalog)

	loaders := []struct {
		name string
		load func() (*WorkspaceCatalog, error)
	}{
		{
			name: "manager",
			load: func() (*WorkspaceCatalog, error) {
				loaded, loadErr := manager.LoadDomains()
				if loadErr != nil {
					return nil, loadErr
				}
				return loaded.Catalog, nil
			},
		},
		{
			name: "coordinator",
			load: func() (*WorkspaceCatalog, error) {
				loaded, loadErr := manager.DomainCoordinator().LoadDomains()
				if loadErr != nil {
					return nil, loadErr
				}
				return loaded.Catalog, nil
			},
		},
		{
			name: "runtime",
			load: func() (*WorkspaceCatalog, error) {
				loaded, loadErr := manager.LoadRuntimeState()
				if loadErr != nil {
					return nil, loadErr
				}
				return loaded.Catalog(), nil
			},
		},
	}

	for _, loader := range loaders {
		t.Run(loader.name, func(t *testing.T) {
			catalog, loadErr := loader.load()
			if loadErr != nil {
				t.Fatalf("load rejected repairable color slot: %v", loadErr)
			}
			if got := catalog.Projects[0].ColorSlot; got != ProjectColorSlotCount+1 {
				t.Fatalf("load repaired color slot to %d, want persisted value %d", got, ProjectColorSlotCount+1)
			}
			if !catalog.EnsureProjectColorSlots() {
				t.Fatal("EnsureProjectColorSlots did not repair the loaded catalog")
			}
			if err := catalog.Validate(); err != nil {
				t.Fatalf("repaired catalog does not validate: %v", err)
			}
		})
	}
}

func TestStrictCatalogPersistenceRejectsOutOfRangeColorSlots(t *testing.T) {
	manager, dataDir := seededManager(t)
	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("load seeded domains: %v", err)
	}
	catalogPath := WorkspaceCatalogPath(dataDir)
	before, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read seeded catalog: %v", err)
	}
	domains.Catalog.Projects = []CatalogProject{{
		ID:        "invalid",
		Slug:      "invalid",
		ColorSlot: ProjectColorSlotCount + 1,
	}}

	err = manager.domainStore().SaveCatalog(domains.Catalog)
	if err == nil || !strings.Contains(err.Error(), "invalid color slot") {
		t.Fatalf("SaveCatalog error = %v, want invalid color slot", err)
	}
	after, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read catalog after rejected save: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("rejected strict catalog save changed persisted bytes")
	}
}

func writeRawDomainPayload(t *testing.T, dataDir, workspaceID string, kind domainKind, payload any) string {
	t.Helper()
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal %s payload: %v", kind.name(), err)
	}
	raw, err := json.MarshalIndent(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   workspaceID,
		Kind:          kind.name(),
		Data:          rawPayload,
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s envelope: %v", kind.name(), err)
	}

	var path string
	switch kind {
	case domainCatalog:
		path = WorkspaceCatalogPath(dataDir)
	case domainPlanning:
		path = PlanningStatePath(dataDir)
	case domainMachine:
		path, err = MachineStatePath(workspaceID)
		if err != nil {
			t.Fatalf("resolve machine path: %v", err)
		}
	default:
		t.Fatalf("unsupported domain kind %v", kind)
	}
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatalf("create %s directory: %v", kind.name(), err)
	}
	if err := os.WriteFile(path, raw, FilePermissions); err != nil {
		t.Fatalf("write raw %s: %v", kind.name(), err)
	}
	return path
}
