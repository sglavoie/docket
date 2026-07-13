package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newIsolatedStore roots a DomainFileStore at a fresh temp data dir with HOME
// pointed at another temp dir so machine-local runtime state is isolated too.
func newIsolatedStore(t *testing.T) *DomainFileStore {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	return NewDomainFileStoreWithWorkspace(dataDir, "ws-test")
}

func TestDomainFileStore_RoundTripAllDomains(t *testing.T) {
	s := newIsolatedStore(t)

	catalog := NewWorkspaceCatalog()
	catalog.WorkspaceID = "ws-round-trip"
	catalog.Projects = append(catalog.Projects, CatalogProject{ID: "p1", Slug: "proj", Name: "Proj"})
	catalog.SystemInboxProjectID = "p1"
	if err := s.SaveCatalog(catalog); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}

	planning := NewPlanningState()
	planning.StreakCount = 4
	if err := s.SavePlanning(planning); err != nil {
		t.Fatalf("SavePlanning: %v", err)
	}

	settings := NewSettings()
	settings.DefaultContext = "work"
	if err := s.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	automation := NewAutomationConfig()
	automation.DefaultRulesInitialized = true
	if err := s.SaveAutomation(automation); err != nil {
		t.Fatalf("SaveAutomation: %v", err)
	}

	machine := NewMachineState()
	machine.CurrentProject = "p1"
	if err := s.SaveMachine(machine); err != nil {
		t.Fatalf("SaveMachine: %v", err)
	}

	// Reload through a fresh store to prove persistence, not in-memory reuse.
	s2 := NewDomainFileStore(s.DataDir())

	gotCatalog, err := s2.LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if gotCatalog.WorkspaceID != "ws-round-trip" || len(gotCatalog.Projects) != 1 || gotCatalog.SystemInboxProjectID != "p1" {
		t.Fatalf("catalog round-trip mismatch: %+v", gotCatalog)
	}
	// The store adopts the workspace ID from the catalog it read.
	if s2.WorkspaceID() != "ws-round-trip" {
		t.Fatalf("store did not adopt workspace id from catalog: %q", s2.WorkspaceID())
	}

	gotPlanning, err := s2.LoadPlanning()
	if err != nil {
		t.Fatalf("LoadPlanning: %v", err)
	}
	if gotPlanning.StreakCount != 4 {
		t.Fatalf("planning round-trip mismatch: %+v", gotPlanning)
	}

	gotSettings, err := s2.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if gotSettings.DefaultContext != "work" {
		t.Fatalf("settings round-trip mismatch: %+v", gotSettings)
	}

	gotAutomation, err := s2.LoadAutomation()
	if err != nil {
		t.Fatalf("LoadAutomation: %v", err)
	}
	if !gotAutomation.DefaultRulesInitialized {
		t.Fatalf("automation round-trip mismatch: %+v", gotAutomation)
	}

	gotMachine, err := s2.LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if gotMachine.CurrentProject != "p1" {
		t.Fatalf("machine round-trip mismatch: %+v", gotMachine)
	}
}

func TestDomainFileStore_StampsSchemaAndWorkspaceID(t *testing.T) {
	s := newIsolatedStore(t)
	if err := s.SaveSettings(NewSettings()); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	data, err := os.ReadFile(SettingsPath(s.DataDir()))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var env domainEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.SchemaVersion != CurrentDomainSchemaVersion {
		t.Fatalf("schema version = %d, want %d", env.SchemaVersion, CurrentDomainSchemaVersion)
	}
	if env.WorkspaceID != "ws-test" {
		t.Fatalf("workspace id = %q, want ws-test", env.WorkspaceID)
	}
	if env.Kind != "settings" {
		t.Fatalf("kind = %q, want settings", env.Kind)
	}
}

func TestDomainFileStore_LoadDefaultsWhenAbsent(t *testing.T) {
	s := newIsolatedStore(t)
	settings, err := s.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings on empty store: %v", err)
	}
	// NewSettings defaults, not a zero value.
	if !settings.FzfEnabled || settings.UndoHistoryDepth == nil {
		t.Fatalf("absent settings should fall back to defaults, got %+v", settings)
	}
}

func TestDomainFileStore_CustomPathIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dirA := filepath.Join(t.TempDir(), "a")
	dirB := filepath.Join(t.TempDir(), "b")

	storeA := NewDomainFileStoreWithWorkspace(dirA, "ws-a")
	storeB := NewDomainFileStoreWithWorkspace(dirB, "ws-b")

	planA := NewPlanningState()
	planA.StreakCount = 11
	if err := storeA.SavePlanning(planA); err != nil {
		t.Fatalf("SavePlanning A: %v", err)
	}

	// B was never written; it must not see A's data.
	planB, err := storeB.LoadPlanning()
	if err != nil {
		t.Fatalf("LoadPlanning B: %v", err)
	}
	if planB.StreakCount != 0 {
		t.Fatalf("store B leaked data from store A: %+v", planB)
	}

	// A's synced artifacts live under its own data dir, never under B's.
	if _, err := os.Stat(PlanningStatePath(dirA)); err != nil {
		t.Fatalf("store A planning.json missing: %v", err)
	}
	if _, err := os.Stat(PlanningStatePath(dirB)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("store B planning.json should not exist, stat err = %v", err)
	}
}

func TestDomainFileStore_MachineStateIsMachineLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// A data dir distinct from the local root so machine state is keyed under
	// the local runtime dir rather than sharing the data dir.
	dataDir := filepath.Join(t.TempDir(), "synced-workspace")
	s := NewDomainFileStore(dataDir)

	if err := s.SaveMachine(NewMachineState()); err != nil {
		t.Fatalf("SaveMachine: %v", err)
	}

	machinePath := MachineStatePath(dataDir)
	// Machine state must not live under the (potentially synced) data dir.
	if strings.HasPrefix(machinePath, filepath.Clean(dataDir)) {
		t.Fatalf("machine.json %q must not live under data dir %q", machinePath, dataDir)
	}
	// It must live under the local runtime root.
	if !strings.HasPrefix(machinePath, LocalDataRoot()) {
		t.Fatalf("machine.json %q must live under local root %q", machinePath, LocalDataRoot())
	}
	if _, err := os.Stat(machinePath); err != nil {
		t.Fatalf("machine.json not written at %q: %v", machinePath, err)
	}
	// No machine.json under the data dir's internal directory.
	if _, err := os.Stat(filepath.Join(InternalDir(dataDir), "machine.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("machine.json should not exist under data dir internal dir, stat err = %v", err)
	}
}

func TestDomainFileStore_ConcurrentStaleObjectDetection(t *testing.T) {
	s := newIsolatedStore(t)

	// Seed an initial catalog.
	if err := s.SaveCatalog(NewWorkspaceCatalog()); err != nil {
		t.Fatalf("seed SaveCatalog: %v", err)
	}

	// Reader A loads the current revision.
	catA, revA, err := s.LoadCatalogChecked()
	if err != nil {
		t.Fatalf("LoadCatalogChecked A: %v", err)
	}

	// Writer B advances the on-disk artifact behind A's back.
	catB := NewWorkspaceCatalog()
	catB.Projects = append(catB.Projects, CatalogProject{ID: "pB", Slug: "b", Name: "B"})
	if saveErr := s.SaveCatalog(catB); saveErr != nil {
		t.Fatalf("writer B SaveCatalog: %v", saveErr)
	}

	// A's checked save must now fail as stale rather than clobbering B.
	catA.Projects = append(catA.Projects, CatalogProject{ID: "pA", Slug: "a", Name: "A"})
	err = s.SaveCatalogChecked(catA, revA)
	if !errors.Is(err, ErrStaleObject) {
		t.Fatalf("stale checked save = %v, want ErrStaleObject", err)
	}

	// B's changes survived; A's did not.
	got, err := s.LoadCatalog()
	if err != nil {
		t.Fatalf("reload after stale save: %v", err)
	}
	if len(got.Projects) != 1 || got.Projects[0].ID != "pB" {
		t.Fatalf("stale save clobbered concurrent writer: %+v", got.Projects)
	}

	// Re-loading gives a fresh revision that saves cleanly.
	fresh, revFresh, err := s.LoadCatalogChecked()
	if err != nil {
		t.Fatalf("LoadCatalogChecked fresh: %v", err)
	}
	fresh.Projects = append(fresh.Projects, CatalogProject{ID: "pA", Slug: "a", Name: "A"})
	if err := s.SaveCatalogChecked(fresh, revFresh); err != nil {
		t.Fatalf("checked save with fresh revision: %v", err)
	}
}

func TestDomainFileStore_RecoversFromBackup(t *testing.T) {
	s := newIsolatedStore(t)

	planning := NewPlanningState()
	planning.StreakCount = 7
	if err := s.SavePlanning(planning); err != nil {
		t.Fatalf("first SavePlanning: %v", err)
	}
	// A second save produces a .bak of the first good file.
	planning.StreakCount = 8
	if err := s.SavePlanning(planning); err != nil {
		t.Fatalf("second SavePlanning: %v", err)
	}

	// Corrupt the primary; the .bak (streak 7) should be recovered.
	path := PlanningStatePath(s.DataDir())
	if err := os.WriteFile(path, []byte("{not json"), FilePermissions); err != nil {
		t.Fatalf("corrupt primary: %v", err)
	}

	recovered, err := NewDomainFileStore(s.DataDir()).LoadPlanning()
	if err != nil {
		t.Fatalf("LoadPlanning after corruption: %v", err)
	}
	if recovered.StreakCount != 7 {
		t.Fatalf("recovered streak = %d, want 7 from backup", recovered.StreakCount)
	}
	// Recovery re-persisted the good bytes so the primary parses again.
	if _, err := NewDomainFileStore(s.DataDir()).LoadPlanning(); err != nil {
		t.Fatalf("primary not repaired after recovery: %v", err)
	}
}

func TestDomainFileStore_WritesPrivatePermissions(t *testing.T) {
	s := newIsolatedStore(t)
	if err := s.SaveSettings(NewSettings()); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	info, err := os.Stat(SettingsPath(s.DataDir()))
	if err != nil {
		t.Fatalf("stat settings.json: %v", err)
	}
	if got := info.Mode().Perm(); got != os.FileMode(FilePermissions) {
		t.Fatalf("settings.json perms = %o, want %o", got, FilePermissions)
	}
}
