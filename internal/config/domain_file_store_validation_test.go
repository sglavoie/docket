package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// This file covers what the store refuses to read or write: a slug that would
// escape the workspace, a color slot the catalog can repair rather than reject,
// and an invalid catalog that must produce no filesystem effect at all. The
// fixtures are in domain_file_store_test.go.

func TestDomainFileStore_RejectsMalformedCatalogSlugs(t *testing.T) {
	for _, slug := range []string{"../escape", "Upper", "_sandbox"} {
		t.Run(slug, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			dataDir := filepath.Join(t.TempDir(), "workspace")
			path := WorkspaceCatalogPath(dataDir)
			if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(&WorkspaceCatalog{
				SchemaVersion: CurrentConfigVersion,
				WorkspaceID:   testWorkspaceID,
				Projects:      []CatalogProject{{ID: "unsafe", Slug: slug}},
			})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(domainEnvelope{
				SchemaVersion: CurrentDomainSchemaVersion,
				WorkspaceID:   testWorkspaceID,
				Kind:          domainCatalog.name(),
				Data:          payload,
			})
			if err != nil {
				t.Fatal(err)
			}
			if writeErr := os.WriteFile(path, raw, FilePermissions); writeErr != nil {
				t.Fatal(writeErr)
			}

			if _, loadErr := NewDomainFileStore(dataDir).LoadCatalog(); loadErr == nil {
				t.Fatalf("LoadCatalog accepted malformed slug %q", slug)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, raw) {
				t.Fatal("malformed catalog was rewritten during rejected load")
			}
		})
	}
}

func TestDomainFileStore_LoadCatalogAllowsRepairableOutOfRangeColorSlots(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	path := WorkspaceCatalogPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatal(err)
	}
	catalog := &WorkspaceCatalog{
		SchemaVersion: CurrentConfigVersion,
		WorkspaceID:   testWorkspaceID,
		Projects: []CatalogProject{
			{ID: "assigned", Slug: "assigned", ColorSlot: 1},
			{ID: "eligible", Slug: "eligible", ColorSlot: ProjectColorSlotCount + 1},
		},
	}
	payload, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   testWorkspaceID,
		Kind:          domainCatalog.name(),
		Data:          payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(path, raw, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	loaded, rev, err := NewDomainFileStore(dataDir).LoadCatalogChecked()
	if err != nil {
		t.Fatalf("LoadCatalogChecked rejected repairable color slots: %v", err)
	}
	if !loaded.EnsureProjectColorSlots() {
		t.Fatal("EnsureProjectColorSlots did not repair out-of-range slots")
	}
	want := map[string]int{"assigned": 1, "eligible": 2, "internal": 0}
	for _, project := range loaded.Projects {
		if project.ColorSlot != want[project.ID] {
			t.Errorf("project %s ColorSlot = %d, want %d", project.ID, project.ColorSlot, want[project.ID])
		}
	}
	if err := loaded.Validate(); err != nil {
		t.Fatalf("repaired catalog does not validate: %v", err)
	}
	if err := NewDomainFileStore(dataDir).SaveCatalogChecked(loaded, rev); err != nil {
		t.Fatalf("persist repaired catalog: %v", err)
	}
}

func TestDomainFileStore_RejectsInvalidCatalogBeforeSaveEffects(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "absent", "workspace")
	catalog := NewWorkspaceCatalog()
	catalog.Projects = []CatalogProject{{ID: "escape", Slug: "../escape"}}

	if err := NewDomainFileStore(dataDir).SaveCatalog(catalog); err == nil {
		t.Fatal("SaveCatalog accepted traversal slug")
	}
	if _, err := os.Lstat(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid catalog save created workspace effects: %v", err)
	}
}
