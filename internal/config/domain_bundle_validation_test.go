package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

func TestDomainBundleLoadsRejectInvalidAIPreferences(t *testing.T) {
	tests := []struct {
		name   string
		kind   domainKind
		mutate func(*Domains)
		want   string
	}{
		{
			name: "unknown harness",
			kind: domainMachine,
			mutate: func(domains *Domains) {
				domains.Machine.AIHarness = "gemini"
			},
			want: "invalid machine domain",
		},
		{
			name: "unknown consent provider",
			kind: domainSettings,
			mutate: func(domains *Domains) {
				domains.Settings.AIConsent.Providers = map[string]AIProviderConsent{"gemini": {Granted: true}}
			},
			want: "invalid settings domain",
		},
		{
			name: "empty config override",
			kind: domainMachine,
			mutate: func(domains *Domains) {
				domains.Machine.AIConfigDirs = map[string]string{"codex": ""}
			},
			want: "must contain an explicit path",
		},
		{
			name: "relative config override",
			kind: domainMachine,
			mutate: func(domains *Domains) {
				domains.Machine.AIConfigDirs = map[string]string{"codex": "configs/codex"}
			},
			want: "must contain an absolute path",
		},
	}
	loaders := []struct {
		name string
		load func(*Manager) error
	}{
		{
			name: "manager",
			load: func(manager *Manager) error {
				_, err := manager.LoadDomains()
				return err
			},
		},
		{
			name: "coordinator",
			load: func(manager *Manager) error {
				_, err := manager.DomainCoordinator().LoadDomains()
				return err
			},
		},
		{
			name: "runtime",
			load: func(manager *Manager) error {
				_, err := manager.LoadRuntimeState()
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, loader := range loaders {
				t.Run(loader.name, func(t *testing.T) {
					manager, dataDir := seededManager(t)
					domains, err := manager.LoadDomains()
					if err != nil {
						t.Fatalf("load seeded domains: %v", err)
					}
					tt.mutate(domains)
					writeRawDomainPayload(t, dataDir, domains.Catalog.WorkspaceID, tt.kind, payloadForDomain(domains, tt.kind))

					err = loader.load(manager)
					if err == nil || !strings.Contains(err.Error(), tt.want) {
						t.Fatalf("load error = %v, want containing %q", err, tt.want)
					}
				})
			}
		})
	}
}

func TestRuntimeStateRefreshRejectsInvalidBundleWithoutAdoptingState(t *testing.T) {
	manager, dataDir := seededManager(t)
	runtimeState, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	previousDomains := runtimeState.domains
	previousRuntimeRevs := cloneDomainRevisions(runtimeState.revs)
	previousCoordinatorRevs := cloneDomainRevisions(runtimeState.coord.revs)

	corrupted := NewSettings()
	corrupted.DefaultContext = "corrupted"
	corrupted.AIConsent.Providers = map[string]AIProviderConsent{"gemini": {Granted: true}}
	writeRawDomainPayload(t, dataDir, runtimeState.Catalog().WorkspaceID, domainSettings, corrupted)

	err = runtimeState.RefreshAfterCommit()
	if err == nil || !strings.Contains(err.Error(), "invalid settings domain") {
		t.Fatalf("RefreshAfterCommit error = %v, want invalid settings domain", err)
	}
	if runtimeState.domains != previousDomains {
		t.Fatal("failed refresh replaced the last valid domain snapshot")
	}
	if got := runtimeState.Settings().DefaultContext; got != "personal" {
		t.Fatalf("failed refresh adopted corrupted DefaultContext %q", got)
	}
	if !reflect.DeepEqual(runtimeState.revs, previousRuntimeRevs) {
		t.Fatalf("failed refresh changed runtime revisions: got %v want %v", runtimeState.revs, previousRuntimeRevs)
	}
	if !reflect.DeepEqual(runtimeState.coord.revs, previousCoordinatorRevs) {
		t.Fatalf("failed refresh changed coordinator revisions: got %v want %v", runtimeState.coord.revs, previousCoordinatorRevs)
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
	case domainSettings:
		path = SettingsPath(dataDir)
	case domainAutomation:
		path = AutomationPath(dataDir)
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

func payloadForDomain(domains *Domains, kind domainKind) any {
	switch kind {
	case domainCatalog:
		return domains.Catalog
	case domainPlanning:
		return domains.Planning
	case domainSettings:
		return domains.Settings
	case domainAutomation:
		return domains.Automation
	case domainMachine:
		return domains.Machine
	default:
		return nil
	}
}

func cloneDomainRevisions(revisions map[domainKind]string) map[domainKind]string {
	cloned := make(map[domainKind]string, len(revisions))
	for kind, revision := range revisions {
		cloned[kind] = revision
	}
	return cloned
}
