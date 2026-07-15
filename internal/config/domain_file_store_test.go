package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
)

const (
	testWorkspaceID  = "11111111-1111-4111-8111-111111111111"
	testWorkspaceIDA = "22222222-2222-4222-8222-222222222222"
	testWorkspaceIDB = "33333333-3333-4333-8333-333333333333"
)

// newIsolatedStore roots a DomainFileStore at a fresh temp data dir with HOME
// pointed at another temp dir so machine-local runtime state is isolated too.
func newIsolatedStore(t *testing.T) *DomainFileStore {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	store := NewDomainFileStoreWithWorkspace(dataDir, testWorkspaceID)
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
	if gotCatalog.WorkspaceID != testWorkspaceID || len(gotCatalog.Projects) != 1 || gotCatalog.SystemInboxProjectID != "p1" {
		t.Fatalf("catalog round-trip mismatch: %+v", gotCatalog)
	}
	// The store adopts the workspace ID from the catalog it read.
	if s2.WorkspaceID() != testWorkspaceID {
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
	if env.WorkspaceID != testWorkspaceID {
		t.Fatalf("workspace id = %q, want %q", env.WorkspaceID, testWorkspaceID)
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

	storeA := NewDomainFileStoreWithWorkspace(dirA, testWorkspaceIDA)
	storeB := NewDomainFileStoreWithWorkspace(dirB, testWorkspaceIDB)
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
	catalog := NewWorkspaceCatalog()
	catalog.WorkspaceID = testWorkspaceIDA
	if err := s.SaveCatalog(catalog); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}

	if err := s.SaveMachine(NewMachineState()); err != nil {
		t.Fatalf("SaveMachine: %v", err)
	}

	machinePath, err := MachineStatePath(testWorkspaceIDA)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
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

func TestDomainFileStore_MachineStateRequiresCatalogWorkspaceID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	store := NewDomainFileStore(filepath.Join(t.TempDir(), "workspace"))

	if _, err := store.LoadMachine(); !errors.Is(err, ErrWorkspaceIDRequired) {
		t.Fatalf("LoadMachine without catalog error = %v, want ErrWorkspaceIDRequired", err)
	}
	if err := store.SaveMachine(NewMachineState()); !errors.Is(err, ErrWorkspaceIDRequired) {
		t.Fatalf("SaveMachine without catalog error = %v, want ErrWorkspaceIDRequired", err)
	}
	if _, err := os.Stat(filepath.Join(LocalDataRoot(), runtimeDirName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("machine access without a catalog must not construct runtime paths, stat err = %v", err)
	}
}

func TestDomainFileStore_RejectsInvalidAndMismatchedWorkspaceIDs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")

	invalid := NewDomainFileStoreWithWorkspace(dataDir, "../not-a-workspace-uuid")
	if err := invalid.SaveSettings(NewSettings()); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("SaveSettings invalid workspace ID error = %v, want ErrInvalidWorkspaceID", err)
	}

	seed := NewDomainFileStore(dataDir)
	catalog := NewWorkspaceCatalog()
	catalog.WorkspaceID = testWorkspaceIDA
	if err := seed.SaveCatalog(catalog); err != nil {
		t.Fatalf("seed SaveCatalog: %v", err)
	}
	if err := seed.SavePlanning(NewPlanningState()); err != nil {
		t.Fatalf("seed SavePlanning: %v", err)
	}

	foreign := NewDomainFileStoreWithWorkspace(dataDir, testWorkspaceIDB)
	if err := foreign.SaveMachine(NewMachineState()); !errors.Is(err, ErrWorkspaceIDMismatch) {
		t.Fatalf("SaveMachine mismatched bound ID error = %v, want ErrWorkspaceIDMismatch", err)
	}
	if err := foreign.SavePlanning(NewPlanningState()); !errors.Is(err, ErrWorkspaceIDMismatch) {
		t.Fatalf("SavePlanning mismatched bound ID error = %v, want ErrWorkspaceIDMismatch", err)
	}

	planningPath := PlanningStatePath(dataDir)
	data, err := os.ReadFile(planningPath)
	if err != nil {
		t.Fatalf("read planning envelope: %v", err)
	}
	var env domainEnvelope
	if decodeErr := json.Unmarshal(data, &env); decodeErr != nil {
		t.Fatalf("decode planning envelope: %v", decodeErr)
	}
	env.WorkspaceID = testWorkspaceIDB
	tampered, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("encode tampered planning envelope: %v", err)
	}
	if writeErr := os.WriteFile(planningPath, tampered, FilePermissions); writeErr != nil {
		t.Fatalf("write tampered planning envelope: %v", writeErr)
	}

	reloaded := NewDomainFileStore(dataDir)
	if _, loadErr := reloaded.LoadCatalog(); loadErr != nil {
		t.Fatalf("LoadCatalog: %v", loadErr)
	}
	if _, loadErr := reloaded.LoadPlanning(); !errors.Is(loadErr, ErrWorkspaceIDMismatch) {
		t.Fatalf("LoadPlanning mismatched envelope error = %v, want ErrWorkspaceIDMismatch", loadErr)
	}

	catalogPath := WorkspaceCatalogPath(dataDir)
	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read catalog envelope: %v", err)
	}
	var catalogEnv domainEnvelope
	if decodeErr := json.Unmarshal(catalogData, &catalogEnv); decodeErr != nil {
		t.Fatalf("decode catalog envelope: %v", decodeErr)
	}
	var catalogPayload WorkspaceCatalog
	if decodeErr := json.Unmarshal(catalogEnv.Data, &catalogPayload); decodeErr != nil {
		t.Fatalf("decode catalog payload: %v", decodeErr)
	}
	catalogPayload.WorkspaceID = testWorkspaceIDB
	catalogEnv.Data, err = json.Marshal(catalogPayload)
	if err != nil {
		t.Fatalf("encode mismatched catalog payload: %v", err)
	}
	tamperedCatalog, err := json.MarshalIndent(catalogEnv, "", "  ")
	if err != nil {
		t.Fatalf("encode mismatched catalog envelope: %v", err)
	}
	if err := os.WriteFile(catalogPath, tamperedCatalog, FilePermissions); err != nil {
		t.Fatalf("write mismatched catalog payload: %v", err)
	}
	if _, err := NewDomainFileStore(dataDir).LoadCatalog(); !errors.Is(err, ErrWorkspaceIDMismatch) {
		t.Fatalf("LoadCatalog mismatched payload error = %v, want ErrWorkspaceIDMismatch", err)
	}
}

func TestDomainFileStore_FreshBundleStampsOneWorkspaceID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := filepath.Join(t.TempDir(), "fresh-workspace")
	manager := NewManager(ConfigPathForDataDir(dataDir))

	domains, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatalf("LoadOrCreateDomains: %v", err)
	}
	workspaceID := domains.Catalog.WorkspaceID
	if validateErr := validateWorkspaceID(workspaceID); validateErr != nil {
		t.Fatalf("fresh workspace ID %q is invalid: %v", workspaceID, validateErr)
	}
	machinePath, err := MachineStatePath(workspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}

	paths := []string{
		WorkspaceCatalogPath(dataDir),
		PlanningStatePath(dataDir),
		SettingsPath(dataDir),
		AutomationPath(dataDir),
		machinePath,
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var env domainEnvelope
		if decodeErr := json.Unmarshal(data, &env); decodeErr != nil {
			t.Fatalf("decode %s: %v", path, decodeErr)
		}
		if env.WorkspaceID != workspaceID {
			t.Errorf("%s workspace ID = %q, want %q", filepath.Base(path), env.WorkspaceID, workspaceID)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != os.FileMode(FilePermissions) {
			t.Errorf("%s permissions = %o, want %o", filepath.Base(path), got, FilePermissions)
		}
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

func TestDomainFileStore_RecoversFromBackupWithRepairedRevision(t *testing.T) {
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
	backupData, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read planning backup: %v", err)
	}
	if writeErr := os.WriteFile(path, []byte("{not json"), FilePermissions); writeErr != nil {
		t.Fatalf("corrupt primary: %v", writeErr)
	}

	recovered, recoveredRevision, err := NewDomainFileStore(s.DataDir()).LoadPlanningChecked()
	if err != nil {
		t.Fatalf("LoadPlanningChecked after corruption: %v", err)
	}
	if recovered.StreakCount != 7 {
		t.Fatalf("recovered streak = %d, want 7 from backup", recovered.StreakCount)
	}
	repairedData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read repaired primary: %v", err)
	}
	if !bytes.Equal(repairedData, backupData) {
		t.Fatal("repaired primary bytes differ from the recovered backup")
	}
	if want := revisionOf(repairedData); recoveredRevision != want {
		t.Fatalf("recovered revision = %q, want repaired-byte revision %q", recoveredRevision, want)
	}

	// The revision returned by the recovering load must be immediately usable as
	// a checked transaction baseline; hashing the corrupt pre-recovery bytes would
	// make this commit fail stale.
	recovered.StreakCount = 9
	updated, err := encodeDomainEnvelope(domainPlanning, testWorkspaceID, recovered)
	if err != nil {
		t.Fatalf("encode recovered planning update: %v", err)
	}
	if _, commitErr := storage.NewCommitService(TransactionsDir(s.DataDir()), []string{s.DataDir()}).CommitArtifacts([]storage.ArtifactChange{{
		Path:             path,
		Data:             updated,
		Mode:             FilePermissions,
		ExpectedRevision: &recoveredRevision,
	}}); commitErr != nil {
		t.Fatalf("commit checked artifact after recovery: %v", commitErr)
	}
	fresh, err := NewDomainFileStore(s.DataDir()).LoadPlanning()
	if err != nil {
		t.Fatalf("reload committed planning: %v", err)
	}
	if fresh.StreakCount != 9 {
		t.Fatalf("committed streak = %d, want 9", fresh.StreakCount)
	}
}

func TestDomainFileStore_RecoversTransactionalDomainBackup(t *testing.T) {
	s := newIsolatedStore(t)
	planning := NewPlanningState()
	planning.StreakCount = 7
	if err := s.SavePlanning(planning); err != nil {
		t.Fatalf("seed planning: %v", err)
	}
	path := PlanningStatePath(s.DataDir())
	before, revision, err := s.LoadPlanningChecked()
	if err != nil {
		t.Fatalf("LoadPlanningChecked: %v", err)
	}
	before.StreakCount = 8
	updated, err := encodeDomainEnvelope(domainPlanning, testWorkspaceID, before)
	if err != nil {
		t.Fatalf("encode planning update: %v", err)
	}
	change := (DomainArtifact{Path: path, Data: updated, ExpectedRevision: &revision}).StorageChange()
	if _, commitErr := storage.NewCommitService(TransactionsDir(s.DataDir()), []string{s.DataDir()}).CommitArtifacts([]storage.ArtifactChange{change}); commitErr != nil {
		t.Fatalf("CommitArtifacts: %v", commitErr)
	}

	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read transactional backup: %v", err)
	}
	if writeErr := os.WriteFile(path, []byte("{not json"), FilePermissions); writeErr != nil {
		t.Fatalf("corrupt primary: %v", writeErr)
	}
	recovered, _, err := NewDomainFileStore(s.DataDir()).LoadPlanningChecked()
	if err != nil {
		t.Fatalf("LoadPlanningChecked after corruption: %v", err)
	}
	if recovered.StreakCount != 7 {
		t.Fatalf("recovered streak = %d, want 7", recovered.StreakCount)
	}
	if repaired, err := os.ReadFile(path); err != nil {
		t.Fatalf("read repaired primary: %v", err)
	} else if !bytes.Equal(repaired, backup) {
		t.Fatal("repaired primary differs from transactional backup")
	}
}

func TestDomainFileStore_BackupDoesNotOverrideSemanticPrimaryErrors(t *testing.T) {
	goodBackup, err := encodeDomainEnvelope(domainPlanning, testWorkspaceIDA, &PlanningState{StreakCount: 7})
	if err != nil {
		t.Fatalf("encode backup: %v", err)
	}

	var unsupported domainEnvelope
	if decodeErr := json.Unmarshal(goodBackup, &unsupported); decodeErr != nil {
		t.Fatalf("decode backup fixture: %v", decodeErr)
	}
	unsupported.SchemaVersion = CurrentDomainSchemaVersion + 1
	unsupportedData, err := json.MarshalIndent(unsupported, "", "  ")
	if err != nil {
		t.Fatalf("encode unsupported primary: %v", err)
	}
	mismatchedData, err := encodeDomainEnvelope(domainPlanning, testWorkspaceIDB, &PlanningState{StreakCount: 8})
	if err != nil {
		t.Fatalf("encode mismatched primary: %v", err)
	}
	invalidPayloadData, err := json.MarshalIndent(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   testWorkspaceIDA,
		Kind:          domainPlanning.name(),
		Data:          json.RawMessage(`"not a planning object"`),
	}, "", "  ")
	if err != nil {
		t.Fatalf("encode invalid-payload primary: %v", err)
	}

	tests := []struct {
		name    string
		primary []byte
		check   func(error) bool
	}{
		{name: "unsupported-schema", primary: unsupportedData, check: func(err error) bool { return strings.Contains(err.Error(), "not supported") }},
		{name: "workspace-mismatch", primary: mismatchedData, check: func(err error) bool { return errors.Is(err, ErrWorkspaceIDMismatch) }},
		{name: "invalid-payload", primary: invalidPayloadData, check: func(err error) bool { return strings.Contains(err.Error(), "failed to parse planning.json payload") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := filepath.Join(t.TempDir(), "workspace")
			path := PlanningStatePath(dataDir)
			if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
				t.Fatalf("mkdir domain dir: %v", err)
			}
			if err := os.WriteFile(path, tc.primary, FilePermissions); err != nil {
				t.Fatalf("write primary: %v", err)
			}
			if err := os.WriteFile(path+".bak", goodBackup, FilePermissions); err != nil {
				t.Fatalf("write backup: %v", err)
			}

			_, _, loadErr := NewDomainFileStoreWithWorkspace(dataDir, testWorkspaceIDA).LoadPlanningChecked()
			if loadErr == nil || !tc.check(loadErr) {
				t.Fatalf("LoadPlanningChecked error = %v, want %s primary failure", loadErr, tc.name)
			}
			primaryAfter, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read primary after load: %v", err)
			}
			if !bytes.Equal(primaryAfter, tc.primary) {
				t.Fatal("semantic primary error incorrectly triggered backup replacement")
			}
		})
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
