package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// This file holds the store fixtures the suite shares and the plain persistence
// cases: what a round trip preserves, what an absent artifact falls back to,
// that two data dirs never see each other, and how a written file is stamped and
// permissioned. The refusals are in domain_file_store_validation_test.go, the
// workspace-ID rules in domain_file_store_identity_test.go, and the checked
// revision and backup paths in domain_file_store_recovery_test.go.

const (
	testWorkspaceID  = "11111111-1111-4111-8111-111111111111"
	testWorkspaceIDA = "22222222-2222-4222-8222-222222222222"
	testWorkspaceIDB = "33333333-3333-4333-8333-333333333333"
)

func newBoundTestDomainFileStore(t *testing.T, dataDir, workspaceID string) *DomainFileStore {
	t.Helper()
	store := NewDomainFileStore(dataDir)
	if err := store.BindWorkspaceID(workspaceID); err != nil {
		t.Fatalf("BindWorkspaceID(%q): %v", workspaceID, err)
	}
	return store
}

// newIsolatedStore roots a DomainFileStore at a fresh temp data dir with HOME
// pointed at another temp dir so machine-local runtime state is isolated too.
func newIsolatedStore(t *testing.T) *DomainFileStore {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	store := newBoundTestDomainFileStore(t, dataDir, testWorkspaceID)
	catalog := NewWorkspaceCatalog()
	catalog.WorkspaceID = testWorkspaceID
	if err := store.SaveCatalog(catalog); err != nil {
		t.Fatalf("seed workspace catalog: %v", err)
	}
	return store
}

func TestDomainFileStore_RoundTripAllDomains(t *testing.T) {
	s := newIsolatedStore(t)

	catalog := NewWorkspaceCatalog()
	catalog.WorkspaceID = testWorkspaceID
	catalog.Projects = append(catalog.Projects, CatalogProject{ID: "p1", Slug: "proj", Name: "Proj", ColorSlot: 31})
	catalog.SystemUnfiledProjectID = "p1"
	if err := s.SaveCatalog(catalog); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}

	planning := NewPlanningState()
	planning.AddToToday("today", "p1")
	if err := s.SavePlanning(planning); err != nil {
		t.Fatalf("SavePlanning: %v", err)
	}

	machine := NewMachineState()
	machine.Theme = "light"
	if err := s.SaveMachine(machine); err != nil {
		t.Fatalf("SaveMachine: %v", err)
	}

	// Reload through a fresh store to prove persistence, not in-memory reuse.
	s2 := NewDomainFileStore(s.DataDir())

	gotCatalog, err := s2.LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if gotCatalog.WorkspaceID != testWorkspaceID || len(gotCatalog.Projects) != 1 || gotCatalog.SystemUnfiledProjectID != "p1" {
		t.Fatalf("catalog round-trip mismatch: %+v", gotCatalog)
	}
	if gotCatalog.Projects[0].ColorSlot != 31 {
		t.Fatalf("catalog ColorSlot = %d, want 31", gotCatalog.Projects[0].ColorSlot)
	}
	// The store adopts the workspace ID from the catalog it read.
	if s2.WorkspaceID() != testWorkspaceID {
		t.Fatalf("store did not adopt workspace id from catalog: %q", s2.WorkspaceID())
	}

	gotPlanning, err := s2.LoadPlanning()
	if err != nil {
		t.Fatalf("LoadPlanning: %v", err)
	}
	if len(gotPlanning.Today) != 1 || gotPlanning.Today[0].TaskID != "today" {
		t.Fatalf("planning round-trip mismatch: %+v", gotPlanning)
	}

	gotMachine, err := s2.LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if gotMachine.Theme != "light" {
		t.Fatalf("machine round-trip mismatch: %+v", gotMachine)
	}
}

func TestDomainFileStore_StampsSchemaAndWorkspaceID(t *testing.T) {
	s := newIsolatedStore(t)
	if err := s.SavePlanning(NewPlanningState()); err != nil {
		t.Fatalf("SavePlanning: %v", err)
	}

	data, err := os.ReadFile(PlanningStatePath(s.DataDir()))
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
	if env.WorkspaceID != testWorkspaceID {
		t.Fatalf("workspace id = %q, want %q", env.WorkspaceID, testWorkspaceID)
	}
	if env.Kind != "planning" {
		t.Fatalf("kind = %q, want planning", env.Kind)
	}
}

func TestDomainFileStore_LoadDefaultsWhenAbsent(t *testing.T) {
	s := newIsolatedStore(t)
	planning, err := s.LoadPlanning()
	if err != nil {
		t.Fatalf("LoadPlanning on empty store: %v", err)
	}
	if planning == nil || len(planning.Today) != 0 || len(planning.Next) != 0 {
		t.Fatalf("absent planning should fall back to defaults, got %+v", planning)
	}
}

func TestDomainFileStore_CustomPathIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dirA := filepath.Join(t.TempDir(), "a")
	dirB := filepath.Join(t.TempDir(), "b")

	storeA := newBoundTestDomainFileStore(t, dirA, testWorkspaceIDA)
	storeB := newBoundTestDomainFileStore(t, dirB, testWorkspaceIDB)
	catalogA := NewWorkspaceCatalog()
	catalogA.WorkspaceID = testWorkspaceIDA
	if err := storeA.SaveCatalog(catalogA); err != nil {
		t.Fatalf("SaveCatalog A: %v", err)
	}
	catalogB := NewWorkspaceCatalog()
	catalogB.WorkspaceID = testWorkspaceIDB
	if err := storeB.SaveCatalog(catalogB); err != nil {
		t.Fatalf("SaveCatalog B: %v", err)
	}

	planA := NewPlanningState()
	planA.AddToToday("workspace-a", "project-a")
	if err := storeA.SavePlanning(planA); err != nil {
		t.Fatalf("SavePlanning A: %v", err)
	}

	// B was never written; it must not see A's data.
	planB, err := storeB.LoadPlanning()
	if err != nil {
		t.Fatalf("LoadPlanning B: %v", err)
	}
	if len(planB.Today) != 0 {
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

func TestDomainFileStore_WritesPrivatePermissions(t *testing.T) {
	s := newIsolatedStore(t)
	if err := s.SavePlanning(NewPlanningState()); err != nil {
		t.Fatalf("SavePlanning: %v", err)
	}
	info, err := os.Stat(PlanningStatePath(s.DataDir()))
	if err != nil {
		t.Fatalf("stat settings.json: %v", err)
	}
	if got := info.Mode().Perm(); got != os.FileMode(FilePermissions) {
		t.Fatalf("planning.json perms = %o, want %o", got, FilePermissions)
	}
}
