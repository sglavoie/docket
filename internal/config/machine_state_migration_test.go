package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	return writeLegacyEnvelope(t, path, domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   workspaceID,
		Kind:          domainMachine.name(),
		Data:          payload,
	})
}

func writeLegacyEnvelope(t *testing.T, path string, env domainEnvelope) []byte {
	t.Helper()
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy envelope: %v", err)
	}
	writeLegacyBytes(t, path, data)
	return data
}

func writeLegacyBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatalf("mkdir legacy machine dir: %v", err)
	}
	if err := os.WriteFile(path, data, FilePermissions); err != nil {
		t.Fatalf("write legacy machine bytes: %v", err)
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s changed: got %q, want %q", path, got, want)
	}
}

type invalidLegacyMachineEnvelopeCase struct {
	name         string
	envelope     domainEnvelope
	wantError    string
	wantMismatch bool
}

func invalidLegacyMachineEnvelopeCases() []invalidLegacyMachineEnvelopeCase {
	return []invalidLegacyMachineEnvelopeCase{
		{
			name: "future schema",
			envelope: domainEnvelope{
				SchemaVersion: CurrentDomainSchemaVersion + 1,
				WorkspaceID:   testWorkspaceIDA,
				Kind:          domainMachine.name(),
				Data:          json.RawMessage(`{}`),
			},
			wantError: "schema version",
		},
		{
			name: "wrong kind",
			envelope: domainEnvelope{
				SchemaVersion: CurrentDomainSchemaVersion,
				WorkspaceID:   testWorkspaceIDA,
				Kind:          domainPlanning.name(),
				Data:          json.RawMessage(`{}`),
			},
			wantError: "artifact kind",
		},
		{
			name: "foreign workspace",
			envelope: domainEnvelope{
				SchemaVersion: CurrentDomainSchemaVersion,
				WorkspaceID:   testWorkspaceIDB,
				Kind:          domainMachine.name(),
				Data:          json.RawMessage(`{}`),
			},
			wantError:    ErrWorkspaceIDMismatch.Error(),
			wantMismatch: true,
		},
		{
			name: "malformed payload",
			envelope: domainEnvelope{
				SchemaVersion: CurrentDomainSchemaVersion,
				WorkspaceID:   testWorkspaceIDA,
				Kind:          domainMachine.name(),
				Data:          json.RawMessage(`"not-an-object"`),
			},
			wantError: "machine-state payload",
		},
	}
}

func assertLegacyMachineMigrationError(t *testing.T, err error, tc invalidLegacyMachineEnvelopeCase) {
	t.Helper()
	if err == nil {
		t.Fatalf("LoadRuntimeState succeeded, want %s error", tc.name)
	}
	if !strings.Contains(err.Error(), tc.wantError) {
		t.Fatalf("LoadRuntimeState error = %v, want text %q", err, tc.wantError)
	}
	if tc.wantMismatch && !errors.Is(err, ErrWorkspaceIDMismatch) {
		t.Fatalf("LoadRuntimeState error = %v, want ErrWorkspaceIDMismatch", err)
	}
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
	if string(canonicalBytes) == string(currentBytes) {
		t.Fatal("canonical machine state did not receive the load-time TUI migration")
	}
	var canonical domainEnvelope
	if decodeErr := json.Unmarshal(canonicalBytes, &canonical); decodeErr != nil {
		t.Fatalf("decode canonical envelope: %v", decodeErr)
	}
	var canonicalMachine MachineState
	if decodeErr := json.Unmarshal(canonical.Data, &canonicalMachine); decodeErr != nil {
		t.Fatalf("decode canonical payload: %v", decodeErr)
	}
	if canonicalMachine.TUIStateVersion != CurrentTUIStateVersion {
		t.Fatalf("canonical TUI state = version %d, want %d", canonicalMachine.TUIStateVersion, CurrentTUIStateVersion)
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
	if decodeErr := json.Unmarshal(canonicalBytes, &canonical); decodeErr != nil {
		t.Fatalf("decode canonical machine state: %v", decodeErr)
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

func TestMachineStateMigrationRecoversCorruptPrimaryFromBackup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	seedSyncedWorkspaceWithoutMachine(t, dataDir, testWorkspaceIDA)
	legacyPaths := legacyMachineStatePaths(dataDir)
	if len(legacyPaths) != 2 {
		t.Fatalf("legacy machine paths = %v, want current and historical paths", legacyPaths)
	}

	primaryBytes := []byte("{not json")
	writeLegacyBytes(t, legacyPaths[0], primaryBytes)
	backupBytes := writeLegacyMachineEnvelope(t, legacyPaths[0]+".bak", "", &MachineState{Editor: "backup"})
	lowerPriorityBytes := writeLegacyMachineEnvelope(t, legacyPaths[1], testWorkspaceIDA, &MachineState{Editor: "lower-priority"})

	runtime, err := LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if got := runtime.Machine().Editor; got != "backup" {
		t.Fatalf("migrated editor = %q, want backup", got)
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
	if decodeErr := json.Unmarshal(canonicalBytes, &canonical); decodeErr != nil {
		t.Fatalf("decode canonical machine state: %v", decodeErr)
	}
	if canonical.WorkspaceID != testWorkspaceIDA {
		t.Fatalf("canonical workspace ID = %q, want %q", canonical.WorkspaceID, testWorkspaceIDA)
	}
	assertFileBytes(t, legacyPaths[0], primaryBytes)
	assertFileBytes(t, legacyPaths[0]+".bak", backupBytes)
	assertFileBytes(t, legacyPaths[1], lowerPriorityBytes)

	restarted, err := LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("idempotent restart: %v", err)
	}
	if got := restarted.Machine().Editor; got != "backup" {
		t.Fatalf("restart editor = %q, want canonical backup", got)
	}
	assertFileBytes(t, legacyPaths[0], primaryBytes)
	assertFileBytes(t, legacyPaths[0]+".bak", backupBytes)
	assertFileBytes(t, legacyPaths[1], lowerPriorityBytes)
}

func TestMachineStateMigrationRejectsMissingOrCorruptBackupWithoutFallback(t *testing.T) {
	tests := []struct {
		name       string
		backupData []byte
		wantError  string
	}{
		{name: "missing", wantError: "is unavailable"},
		{name: "corrupt", backupData: []byte("{also not json"), wantError: "is also corrupted"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			dataDir := filepath.Join(t.TempDir(), "workspace")
			seedSyncedWorkspaceWithoutMachine(t, dataDir, testWorkspaceIDA)
			legacyPaths := legacyMachineStatePaths(dataDir)
			primaryBytes := []byte("{not json")
			writeLegacyBytes(t, legacyPaths[0], primaryBytes)
			if tc.backupData != nil {
				writeLegacyBytes(t, legacyPaths[0]+".bak", tc.backupData)
			}
			lowerPriorityBytes := writeLegacyMachineEnvelope(t, legacyPaths[1], testWorkspaceIDA, &MachineState{Editor: "must-not-win"})

			if _, err := LoadRuntimeState(dataDir); err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("LoadRuntimeState error = %v, want text %q", err, tc.wantError)
			}
			canonicalPath, err := MachineStatePath(testWorkspaceIDA)
			if err != nil {
				t.Fatalf("MachineStatePath: %v", err)
			}
			if _, err := os.Stat(canonicalPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unusable backup created canonical state, stat err = %v", err)
			}
			assertFileBytes(t, legacyPaths[0], primaryBytes)
			if tc.backupData != nil {
				assertFileBytes(t, legacyPaths[0]+".bak", tc.backupData)
			}
			assertFileBytes(t, legacyPaths[1], lowerPriorityBytes)
		})
	}
}

func TestMachineStateMigrationRejectsSemanticPrimaryDespiteValidBackup(t *testing.T) {
	for _, tc := range invalidLegacyMachineEnvelopeCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			dataDir := filepath.Join(t.TempDir(), "workspace")
			seedSyncedWorkspaceWithoutMachine(t, dataDir, testWorkspaceIDA)
			legacyPaths := legacyMachineStatePaths(dataDir)
			primaryBytes := writeLegacyEnvelope(t, legacyPaths[0], tc.envelope)
			backupBytes := writeLegacyMachineEnvelope(t, legacyPaths[0]+".bak", testWorkspaceIDA, &MachineState{Editor: "backup-must-not-win"})
			lowerPriorityBytes := writeLegacyMachineEnvelope(t, legacyPaths[1], testWorkspaceIDA, &MachineState{Editor: "lower-must-not-win"})

			_, err := LoadRuntimeState(dataDir)
			assertLegacyMachineMigrationError(t, err, tc)
			canonicalPath, pathErr := MachineStatePath(testWorkspaceIDA)
			if pathErr != nil {
				t.Fatalf("MachineStatePath: %v", pathErr)
			}
			if _, statErr := os.Stat(canonicalPath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("semantic primary error created canonical state, stat err = %v", statErr)
			}
			assertFileBytes(t, legacyPaths[0], primaryBytes)
			assertFileBytes(t, legacyPaths[0]+".bak", backupBytes)
			assertFileBytes(t, legacyPaths[1], lowerPriorityBytes)
		})
	}
}

func TestMachineStateMigrationFullyValidatesBackupBeforeImport(t *testing.T) {
	for _, tc := range invalidLegacyMachineEnvelopeCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			dataDir := filepath.Join(t.TempDir(), "workspace")
			seedSyncedWorkspaceWithoutMachine(t, dataDir, testWorkspaceIDA)
			legacyPaths := legacyMachineStatePaths(dataDir)
			primaryBytes := []byte("{not json")
			writeLegacyBytes(t, legacyPaths[0], primaryBytes)
			backupBytes := writeLegacyEnvelope(t, legacyPaths[0]+".bak", tc.envelope)
			lowerPriorityBytes := writeLegacyMachineEnvelope(t, legacyPaths[1], testWorkspaceIDA, &MachineState{Editor: "lower-must-not-win"})

			_, err := LoadRuntimeState(dataDir)
			assertLegacyMachineMigrationError(t, err, tc)
			canonicalPath, pathErr := MachineStatePath(testWorkspaceIDA)
			if pathErr != nil {
				t.Fatalf("MachineStatePath: %v", pathErr)
			}
			if _, statErr := os.Stat(canonicalPath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("invalid backup created canonical state, stat err = %v", statErr)
			}
			assertFileBytes(t, legacyPaths[0], primaryBytes)
			assertFileBytes(t, legacyPaths[0]+".bak", backupBytes)
			assertFileBytes(t, legacyPaths[1], lowerPriorityBytes)
		})
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
