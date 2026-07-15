package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/workflow"
)

// seedWorkspace writes a complete synced-domain set (plus machine.json) so a
// later domain-scoped commit leaves a loadable, non-partial workspace and every
// domain the commit does not own already exists on disk.
func seedWorkspace(t *testing.T, cfg *Domains) (*Manager, string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), ".tt.json")
	mgr := NewManager(configPath)
	if err := saveDomainBundle(mgr.domainStore(), cfg); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return mgr, configPath
}

// TestManager_CommitSettings_WritesOnlyOwnedDomains verifies that a settings
// commit persists the settings + machine-local domains without rewriting (or
// clobbering) the planning and automation domains it does not own.
func TestManager_CommitSettings_WritesOnlyOwnedDomains(t *testing.T) {
	cfg := NewDomains()
	// Seed a value in a domain the settings commit must NOT touch.
	cfg.Automation.WorkflowRules = []workflow.WorkflowRule{{ID: "rule-1", Name: "seed", Enabled: true}}
	mgr, configPath := seedWorkspace(t, cfg)

	// Mutate a settings-domain field (theme name) and a machine-local field
	// (editor), then commit only those domains.
	cfg.Settings.ActiveTheme = "light"
	cfg.Machine.Editor = "hx"
	d := cfg
	if err := mgr.CommitSettings(d.Settings, d.Machine); err != nil {
		t.Fatalf("CommitSettings: %v", err)
	}

	reloaded, err := NewManager(configPath).LoadDomains()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Machine.Editor != "hx" {
		t.Errorf("machine-local editor = %q, want hx", reloaded.Machine.Editor)
	}
	if reloaded.Settings.ActiveTheme != "light" {
		t.Errorf("settings ActiveTheme = %q, want light", reloaded.Settings.ActiveTheme)
	}
	// The automation domain the commit did not own must survive untouched.
	if rules := reloaded.Automation.WorkflowRules; len(rules) != 1 || rules[0].ID != "rule-1" {
		t.Errorf("automation domain clobbered by settings commit: %+v", rules)
	}
}

// TestManager_CommitSettings_MachineStaysLocal verifies the machine-local file is
// written to the runtime root outside the synced workspace directory.
func TestManager_CommitSettings_MachineStaysLocal(t *testing.T) {
	cfg := NewDomains()
	mgr, _ := seedWorkspace(t, cfg)
	dataDir := mgr.workspaceDataDir()

	cfg.Machine.Editor = "nvim"
	d := cfg
	if err := mgr.CommitSettings(d.Settings, d.Machine); err != nil {
		t.Fatalf("CommitSettings: %v", err)
	}

	machinePath, err := MachineStatePath(cfg.Catalog.WorkspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	if _, err := os.Stat(machinePath); err != nil {
		t.Fatalf("machine.json not written at machine-local path %q: %v", machinePath, err)
	}
	// The machine file must live outside the synced <data>/.tt/ directory.
	if dir := filepath.Dir(machinePath); dir == InternalDir(dataDir) {
		t.Errorf("machine.json placed inside synced internal dir %q", dir)
	}
}

// TestManager_CommitAutomation persists only the automation domain and leaves the
// settings/machine domains untouched.
func TestManager_CommitAutomation(t *testing.T) {
	cfg := NewDomains()
	cfg.Machine.Editor = "seed-editor"
	mgr, configPath := seedWorkspace(t, cfg)

	cfg.Automation.WorkflowRules = append(cfg.Automation.WorkflowRules, workflow.WorkflowRule{ID: "rule-9", Name: "added", Enabled: true})
	if err := mgr.CommitAutomation(cfg.Automation); err != nil {
		t.Fatalf("CommitAutomation: %v", err)
	}

	reloaded, err := NewManager(configPath).LoadDomains()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if rules := reloaded.Automation.WorkflowRules; len(rules) != 1 || rules[0].ID != "rule-9" {
		t.Errorf("automation not persisted: %+v", rules)
	}
	if reloaded.Machine.Editor != "seed-editor" {
		t.Errorf("machine domain clobbered by automation commit: editor = %q", reloaded.Machine.Editor)
	}
}

// TestManager_CommitSettings_NilArgs guards the nil-payload contract.
func TestManager_CommitSettings_NilArgs(t *testing.T) {
	mgr, _ := seedWorkspace(t, NewDomains())
	if err := mgr.CommitSettings(nil, NewMachineState()); err == nil {
		t.Error("expected error for nil settings")
	}
	if err := mgr.CommitSettings(NewSettings(), nil); err == nil {
		t.Error("expected error for nil machine state")
	}
}

// TestManager_CommitCatalog persists only the catalog domain and leaves the
// automation domain it does not own untouched.
func TestManager_CommitCatalog(t *testing.T) {
	cfg := NewDomains()
	cfg.Automation.WorkflowRules = []workflow.WorkflowRule{{ID: "rule-1", Name: "seed", Enabled: true}}
	mgr, configPath := seedWorkspace(t, cfg)

	d := cfg
	d.Catalog.Projects = append(d.Catalog.Projects, CatalogProject{ID: "proj-1", Slug: "proj", Name: "Proj"})
	if err := mgr.CommitCatalog(d.Catalog); err != nil {
		t.Fatalf("CommitCatalog: %v", err)
	}

	reloaded, err := NewManager(configPath).LoadDomains()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if p := NewRuntimeState(mgr.workspaceDataDir(), reloaded).GetProject("proj-1"); p == nil {
		t.Errorf("catalog project not persisted through CommitCatalog")
	}
	if rules := reloaded.Automation.WorkflowRules; len(rules) != 1 || rules[0].ID != "rule-1" {
		t.Errorf("automation domain clobbered by catalog commit: %+v", rules)
	}
}

// TestManager_CommitPlanning persists only the planning domain and leaves the
// machine domain it does not own untouched.
func TestManager_CommitPlanning(t *testing.T) {
	cfg := NewDomains()
	cfg.Machine.Editor = "seed-editor"
	mgr, configPath := seedWorkspace(t, cfg)

	d := cfg
	d.Planning.StreakCount = 5
	if err := mgr.CommitPlanning(d.Planning); err != nil {
		t.Fatalf("CommitPlanning: %v", err)
	}

	reloaded, err := NewManager(configPath).LoadDomains()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Planning.StreakCount != 5 {
		t.Errorf("planning StreakCount not persisted: %d", reloaded.Planning.StreakCount)
	}
	if reloaded.Machine.Editor != "seed-editor" {
		t.Errorf("machine domain clobbered by planning commit: editor = %q", reloaded.Machine.Editor)
	}
}

// TestManager_CommitMachine persists only the machine-local domain and leaves the
// synced automation domain it does not own untouched.
func TestManager_CommitMachine(t *testing.T) {
	cfg := NewDomains()
	cfg.Automation.WorkflowRules = []workflow.WorkflowRule{{ID: "rule-7", Name: "seed", Enabled: true}}
	mgr, configPath := seedWorkspace(t, cfg)

	d := cfg
	d.Machine.Editor = "nvim"
	if err := mgr.CommitMachine(d.Machine); err != nil {
		t.Fatalf("CommitMachine: %v", err)
	}

	reloaded, err := NewManager(configPath).LoadDomains()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Machine.Editor != "nvim" {
		t.Errorf("machine editor not persisted: %q", reloaded.Machine.Editor)
	}
	if rules := reloaded.Automation.WorkflowRules; len(rules) != 1 || rules[0].ID != "rule-7" {
		t.Errorf("automation domain clobbered by machine commit: %+v", rules)
	}
}

// TestManager_CommitCatalog_NilArg guards the nil-payload contract.
func TestManager_CommitCatalog_NilArg(t *testing.T) {
	mgr, _ := seedWorkspace(t, NewDomains())
	if err := mgr.CommitCatalog(nil); err == nil {
		t.Error("expected error for nil catalog")
	}
	if err := mgr.CommitPlanning(nil); err == nil {
		t.Error("expected error for nil planning")
	}
	if err := mgr.CommitMachine(nil); err == nil {
		t.Error("expected error for nil machine state")
	}
}

func TestDomainCoordinator_CheckedCommitsRejectWorkspaceIDMismatch(t *testing.T) {
	mgr, _ := seedWorkspace(t, NewDomains())
	existing, err := mgr.LoadDomains()
	if err != nil {
		t.Fatalf("LoadDomains: %v", err)
	}
	originalID := existing.Catalog.WorkspaceID
	foreign := NewWorkspaceCatalog()
	foreign.WorkspaceID = testWorkspaceIDB

	coord := mgr.DomainCoordinator()
	loaded, err := coord.LoadDomains()
	if err != nil {
		t.Fatalf("coordinator LoadDomains: %v", err)
	}
	if _, artifactsErr := coord.CheckedDomainArtifacts(foreign, loaded.Planning, loaded.Settings, loaded.Automation); !errors.Is(artifactsErr, ErrWorkspaceIDMismatch) {
		t.Fatalf("CheckedDomainArtifacts mismatch error = %v, want ErrWorkspaceIDMismatch", artifactsErr)
	}
	if commitErr := coord.CommitCatalog(foreign); !errors.Is(commitErr, ErrWorkspaceIDMismatch) {
		t.Fatalf("CommitCatalog mismatch error = %v, want ErrWorkspaceIDMismatch", commitErr)
	}

	reloaded, err := mgr.LoadDomains()
	if err != nil {
		t.Fatalf("reload after rejected commits: %v", err)
	}
	if reloaded.Catalog.WorkspaceID != originalID {
		t.Fatalf("rejected commit changed catalog workspace ID to %q, want %q", reloaded.Catalog.WorkspaceID, originalID)
	}
}

func TestDomainCoordinator_CheckedNonCatalogCommitDoesNotMintWorkspaceID(t *testing.T) {
	dataDir := t.TempDir()
	coord := NewDomainCoordinator(dataDir)

	if err := coord.CommitPlanning(NewPlanningState()); !errors.Is(err, ErrWorkspaceIDRequired) {
		t.Fatalf("CommitPlanning without catalog error = %v, want ErrWorkspaceIDRequired", err)
	}
	if _, err := os.Stat(PlanningStatePath(dataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("checked planning commit without catalog wrote an artifact, stat err = %v", err)
	}
}
