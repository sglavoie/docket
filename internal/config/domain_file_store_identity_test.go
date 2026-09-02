package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// One workspace ID keys every artifact the store touches, which is what keeps a
// synced bundle and a machine-local runtime directory from ever being read as
// each other's. This file covers where machine state lives, that it refuses to
// exist without a catalog to key it, and every way a bound or stored ID can
// disagree. The fixtures are in domain_file_store_test.go.

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

	invalid := NewDomainFileStore(dataDir)
	if err := invalid.BindWorkspaceID("../not-a-workspace-uuid"); !errors.Is(err, ErrInvalidWorkspaceID) {
		t.Fatalf("BindWorkspaceID invalid workspace ID error = %v, want ErrInvalidWorkspaceID", err)
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

	foreign := newBoundTestDomainFileStore(t, dataDir, testWorkspaceIDB)
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
