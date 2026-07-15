package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func seedSyncedWorkspaceWithoutMachine(t *testing.T, dataDir, workspaceID string) {
	t.Helper()
	store := NewDomainFileStoreWithWorkspace(dataDir, workspaceID)
	catalog := NewWorkspaceCatalog()
	catalog.WorkspaceID = workspaceID
	if err := store.SaveCatalog(catalog); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}
	if err := store.SavePlanning(NewPlanningState()); err != nil {
		t.Fatalf("SavePlanning: %v", err)
	}
	if err := store.SaveSettings(NewSettings()); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if err := store.SaveAutomation(NewAutomationConfig()); err != nil {
		t.Fatalf("SaveAutomation: %v", err)
	}
}

func writeLegacyMachineEnvelope(t *testing.T, path, workspaceID string, machine *MachineState) []byte {
	t.Helper()
	payload, err := json.Marshal(machine)
	if err != nil {
		t.Fatalf("marshal machine payload: %v", err)
	}
	data, err := json.MarshalIndent(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   workspaceID,
		Kind:          domainMachine.name(),
		Data:          payload,
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal machine envelope: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatalf("mkdir legacy machine dir: %v", err)
	}
	if err := os.WriteFile(path, data, FilePermissions); err != nil {
		t.Fatalf("write legacy machine envelope: %v", err)
	}
	return data
}

func TestMachineStateMigrationImportsCurrentPathBeforeHistoricalDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	seedSyncedWorkspaceWithoutMachine(t, dataDir, testWorkspaceIDA)

	legacyPaths := legacyMachineStatePaths(dataDir)
	if len(legacyPaths) != 2 {
		t.Fatalf("legacy machine paths = %v, want current and historical paths", legacyPaths)
	}
	currentBytes := writeLegacyMachineEnvelope(t, legacyPaths[0], testWorkspaceIDA, &MachineState{Editor: "current-path"})
	historicalBytes := writeLegacyMachineEnvelope(t, legacyPaths[1], testWorkspaceIDA, &MachineState{Editor: "historical-default"})

	runtime, err := LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if got := runtime.Machine().Editor; got != "current-path" {
		t.Fatalf("migrated editor = %q, want current-path", got)
	}
	canonicalPath, err := MachineStatePath(testWorkspaceIDA)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	canonicalBytes, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("read canonical machine state: %v", err)
	}
	if string(canonicalBytes) != string(currentBytes) {
		t.Fatal("matching legacy envelope was not copied byte-for-byte")
	}
	if got, readErr := os.ReadFile(legacyPaths[0]); readErr != nil || string(got) != string(currentBytes) {
		t.Fatalf("current-path rollback source changed: bytes=%q err=%v", got, readErr)
	}
	if got, readErr := os.ReadFile(legacyPaths[1]); readErr != nil || string(got) != string(historicalBytes) {
		t.Fatalf("historical rollback source changed: bytes=%q err=%v", got, readErr)
	}

	// Once canonical state exists, restart must ignore even a now-mismatched
	// higher-priority legacy copy.
	writeLegacyMachineEnvelope(t, legacyPaths[0], testWorkspaceIDB, &MachineState{Editor: "foreign"})
	restarted, err := LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("idempotent restart: %v", err)
	}
	if got := restarted.Machine().Editor; got != "current-path" {
		t.Fatalf("restart editor = %q, want canonical current-path", got)
	}
}

func TestMachineStateMigrationImportsHistoricalDefaultAndClaimsEmptyIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	seedSyncedWorkspaceWithoutMachine(t, dataDir, testWorkspaceIDA)

	legacyPaths := legacyMachineStatePaths(dataDir)
	legacyBytes := writeLegacyMachineEnvelope(t, legacyPaths[len(legacyPaths)-1], "", &MachineState{Editor: "historical"})
	runtime, err := LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if got := runtime.Machine().Editor; got != "historical" {
		t.Fatalf("migrated editor = %q, want historical", got)
	}

	canonicalPath, err := MachineStatePath(testWorkspaceIDA)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	canonicalBytes, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("read canonical machine state: %v", err)
	}
	var canonical domainEnvelope
	if err := json.Unmarshal(canonicalBytes, &canonical); err != nil {
		t.Fatalf("decode canonical machine state: %v", err)
	}
	if canonical.WorkspaceID != testWorkspaceIDA {
		t.Fatalf("canonical workspace ID = %q, want %q", canonical.WorkspaceID, testWorkspaceIDA)
	}
	if got, err := os.ReadFile(legacyPaths[len(legacyPaths)-1]); err != nil || string(got) != string(legacyBytes) {
		t.Fatalf("historical rollback source changed: bytes=%q err=%v", got, err)
	}
}

func TestMachineStateMigrationRejectsIdentityMismatchWithoutFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	seedSyncedWorkspaceWithoutMachine(t, dataDir, testWorkspaceIDA)
	legacyPaths := legacyMachineStatePaths(dataDir)
	writeLegacyMachineEnvelope(t, legacyPaths[0], testWorkspaceIDB, &MachineState{Editor: "foreign"})
	writeLegacyMachineEnvelope(t, legacyPaths[1], testWorkspaceIDA, &MachineState{Editor: "fallback-must-not-win"})

	if _, err := LoadRuntimeState(dataDir); !errors.Is(err, ErrWorkspaceIDMismatch) {
		t.Fatalf("LoadRuntimeState mismatch error = %v, want ErrWorkspaceIDMismatch", err)
	}
	canonicalPath, err := MachineStatePath(testWorkspaceIDA)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	if _, err := os.Stat(canonicalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mismatch created canonical state, stat err = %v", err)
	}
}

func TestRuntimeDependenciesMachineStateMigrationBindsCatalogFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	seedSyncedWorkspaceWithoutMachine(t, dataDir, testWorkspaceIDA)
	writeLegacyMachineEnvelope(t, legacyMachineStatePaths(dataDir)[0], testWorkspaceIDA, &MachineState{Editor: "runtime-dependency"})

	deps := NewManager(ConfigPathForDataDir(dataDir)).RuntimeDependencies()
	machine, err := deps.Machine.LoadMachine()
	if err != nil {
		t.Fatalf("RuntimeDependencies Machine.LoadMachine: %v", err)
	}
	if machine.Editor != "runtime-dependency" {
		t.Fatalf("runtime dependency editor = %q, want runtime-dependency", machine.Editor)
	}
}

func TestMachineStateNewWorkspaceIDUsesIsolatedDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDirA := filepath.Join(t.TempDir(), "workspace-a")
	dataDirB := filepath.Join(t.TempDir(), "workspace-b")
	seedSyncedWorkspaceWithoutMachine(t, dataDirA, testWorkspaceIDA)
	seedSyncedWorkspaceWithoutMachine(t, dataDirB, testWorkspaceIDB)
	storeA := NewDomainFileStore(dataDirA)
	if err := storeA.SaveMachine(&MachineState{Editor: "workspace-a"}); err != nil {
		t.Fatalf("SaveMachine A: %v", err)
	}

	runtimeB, err := LoadRuntimeState(dataDirB)
	if err != nil {
		t.Fatalf("LoadRuntimeState B: %v", err)
	}
	if got, want := runtimeB.Machine().Editor, NewMachineState().Editor; got != want {
		t.Fatalf("new workspace ID editor = %q, want isolated default %q", got, want)
	}
	pathA, _ := MachineStatePath(testWorkspaceIDA)
	pathB, _ := MachineStatePath(testWorkspaceIDB)
	if pathA == pathB {
		t.Fatalf("distinct workspace IDs share machine path %q", pathA)
	}
}
