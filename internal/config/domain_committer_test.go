package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// seedWorkspace writes the complete lean domain set so later domain-scoped
// commits operate against a loadable workspace.
func seedWorkspace(t *testing.T, domains *Domains) (*Manager, string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), ".tt.json")
	manager := NewManager(configPath)
	if err := saveDomainBundle(manager.domainStore(), domains); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return manager, configPath
}

func TestManager_CommitCatalog(t *testing.T) {
	domains := NewDomains()
	manager, configPath := seedWorkspace(t, domains)
	domains.Catalog.Projects = append(domains.Catalog.Projects, CatalogProject{
		ID: "proj-1", Slug: "proj", Name: "Proj",
	})
	if err := manager.CommitCatalog(domains.Catalog); err != nil {
		t.Fatalf("CommitCatalog: %v", err)
	}
	reloaded, err := NewManager(configPath).LoadDomains()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if project := NewRuntimeState(manager.workspaceDataDir(), reloaded).GetProject("proj-1"); project == nil {
		t.Error("catalog project not persisted through CommitCatalog")
	}
}

func TestManager_CommitPlanning(t *testing.T) {
	domains := NewDomains()
	manager, configPath := seedWorkspace(t, domains)
	domains.Planning.AddToToday("planned", "project")
	if err := manager.CommitPlanning(domains.Planning); err != nil {
		t.Fatalf("CommitPlanning: %v", err)
	}
	reloaded, err := NewManager(configPath).LoadDomains()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.Planning.Today; len(got) != 1 || got[0].TaskID != "planned" {
		t.Errorf("Today membership not persisted: %+v", got)
	}
}

func TestManager_CommitMachine(t *testing.T) {
	domains := NewDomains()
	manager, configPath := seedWorkspace(t, domains)
	domains.Machine.Theme = "light"
	if err := manager.CommitMachine(domains.Machine); err != nil {
		t.Fatalf("CommitMachine: %v", err)
	}
	reloaded, err := NewManager(configPath).LoadDomains()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Machine.Theme != "light" {
		t.Errorf("machine theme not persisted: %q", reloaded.Machine.Theme)
	}
	machinePath, err := MachineStatePath(domains.Catalog.WorkspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	if _, err := os.Stat(machinePath); err != nil {
		t.Fatalf("machine state not written at local path %q: %v", machinePath, err)
	}
}

func TestManager_CommitNilArgs(t *testing.T) {
	manager, _ := seedWorkspace(t, NewDomains())
	if err := manager.CommitCatalog(nil); err == nil {
		t.Error("expected error for nil catalog")
	}
	if err := manager.CommitPlanning(nil); err == nil {
		t.Error("expected error for nil planning")
	}
	if err := manager.CommitMachine(nil); err == nil {
		t.Error("expected error for nil machine state")
	}
}

func TestDomainCoordinator_CheckedCommitsRejectWorkspaceIDMismatch(t *testing.T) {
	manager, _ := seedWorkspace(t, NewDomains())
	existing, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("LoadDomains: %v", err)
	}
	foreign := NewWorkspaceCatalog()
	foreign.WorkspaceID = testWorkspaceIDB

	coordinator := manager.DomainCoordinator()
	loaded, err := coordinator.LoadDomains()
	if err != nil {
		t.Fatalf("coordinator LoadDomains: %v", err)
	}
	if _, artifactErr := coordinator.CheckedDomainArtifacts(foreign, loaded.Planning); !errors.Is(artifactErr, ErrWorkspaceIDMismatch) {
		t.Fatalf("CheckedDomainArtifacts mismatch error = %v, want ErrWorkspaceIDMismatch", artifactErr)
	}
	if commitErr := coordinator.CommitCatalog(foreign); !errors.Is(commitErr, ErrWorkspaceIDMismatch) {
		t.Fatalf("CommitCatalog mismatch error = %v, want ErrWorkspaceIDMismatch", commitErr)
	}
	reloaded, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("reload after rejected commits: %v", err)
	}
	if reloaded.Catalog.WorkspaceID != existing.Catalog.WorkspaceID {
		t.Fatalf("rejected commit changed catalog workspace ID to %q", reloaded.Catalog.WorkspaceID)
	}
}

func TestDomainCoordinator_CheckedNonCatalogCommitDoesNotMintWorkspaceID(t *testing.T) {
	dataDir := t.TempDir()
	coordinator := NewDomainCoordinator(dataDir)
	if err := coordinator.CommitPlanning(NewPlanningState()); !errors.Is(err, ErrWorkspaceIDRequired) {
		t.Fatalf("CommitPlanning without catalog error = %v, want ErrWorkspaceIDRequired", err)
	}
	if _, err := os.Stat(PlanningStatePath(dataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("checked planning commit without catalog wrote an artifact, stat err = %v", err)
	}
}
