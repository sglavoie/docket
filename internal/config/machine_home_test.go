package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Two machines sharing one workspace are two homes over one data directory:
// machine.json lives at <home>/.tt/runtime/<workspace-id>/machine.json, so the
// home is the machine identity. The adoption tests express that through $HOME
// because they run one machine at a time; these tests inject the home instead,
// which is the only way two machines can be live at once — a process has exactly
// one $HOME, and mutating it from a goroutine is a data race, not a second
// machine.

// stageSharedWorkspaceOn mints a workspace as one named machine and returns the
// data directory the other machines will join through.
func stageSharedWorkspaceOn(t *testing.T, home string) (string, *Manager) {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "workspace")
	manager := NewManagerOnMachine(ConfigPathForDataDir(dataDir), home)
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatalf("bootstrap workspace on %s: %v", home, err)
	}
	return dataDir, manager
}

// joinWorkspaceOn opens an existing workspace as another machine, running the
// same startup path a real launch runs.
func joinWorkspaceOn(t *testing.T, dataDir, home string) *Manager {
	t.Helper()
	manager := NewManagerOnMachine(ConfigPathForDataDir(dataDir), home)
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatalf("join workspace on %s: %v", home, err)
	}
	return manager
}

func TestMachineHomeDefaultsToProcessHome(t *testing.T) {
	home := t.TempDir()
	useMachine(t, home)

	// An empty machine home is not a third mode: it has to resolve to exactly
	// what the environment-driven helpers resolve to, or every existing caller
	// would quietly change meaning.
	catalogPath, err := MachineStatePath("11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	injected, err := machineStatePathOn("", "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatalf("machineStatePathOn: %v", err)
	}
	if catalogPath != injected {
		t.Fatalf("default machine home = %q, want %q", injected, catalogPath)
	}
	if !strings.HasPrefix(catalogPath, home) {
		t.Fatalf("machine state path %q should resolve under $HOME %q", catalogPath, home)
	}

	if got := NewManager("").machineHome; got != "" {
		t.Fatalf("NewManager machine home = %q, want the process home", got)
	}
	if got := NewDomainFileStore(t.TempDir()).MachineHome(); got != "" {
		t.Fatalf("NewDomainFileStore machine home = %q, want the process home", got)
	}
}

func TestMachineHomeOverridesTheEnvironment(t *testing.T) {
	processHome, namedHome := t.TempDir(), t.TempDir()
	useMachine(t, processHome)

	dataDir := filepath.Join(t.TempDir(), "workspace")
	manager := NewManagerOnMachine(ConfigPathForDataDir(dataDir), namedHome)
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatalf("bootstrap workspace: %v", err)
	}

	store := manager.domainStore()
	if _, _, err := store.LoadMachineChecked(); err != nil {
		t.Fatalf("bind workspace identity: %v", err)
	}
	path, err := store.pathFor(domainMachine)
	if err != nil {
		t.Fatalf("resolve machine state path: %v", err)
	}
	if !strings.HasPrefix(path, namedHome) {
		t.Fatalf("machine state landed at %q, want it under the named home %q", path, namedHome)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("named machine home should hold machine.json: %v", err)
	}
	// The process home must be untouched: a named machine that still wrote there
	// would collide with every other machine in the same process.
	if _, err := os.Stat(filepath.Join(processHome, dataDirName, runtimeDirName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("process home runtime dir stat = %v, want it never created", err)
	}
}

// TestTwoMachineHomesCommitConcurrently is the -race proof that the injected
// home is a real seam: two managers with distinct homes drive one data directory
// at the same time. Nothing here touches the environment, which is the whole
// point — the same test written against $HOME would be a data race on the
// process environment rather than a test of two machines.
func TestTwoMachineHomesCommitConcurrently(t *testing.T) {
	homeA, homeB := t.TempDir(), t.TempDir()
	dataDir, managerA := stageSharedWorkspaceOn(t, homeA)
	managerB := joinWorkspaceOn(t, dataDir, homeB)

	const rounds = 12
	var wg sync.WaitGroup
	errs := make(chan error, 2*rounds)

	commit := func(manager *Manager, theme string) {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			domains, err := manager.LoadOrCreateDomains()
			if err != nil {
				errs <- err
				return
			}
			domains.Machine.Theme = theme
			// A stale synced write is the expected outcome of two machines racing
			// one file; it is refused, not corrupted, and refusal is not a failure
			// of this test. Anything else is.
			if err := manager.CommitMachine(domains.Machine); err != nil && !errors.Is(err, ErrStaleObject) {
				errs <- err
				return
			}
			if err := manager.CommitPlanning(domains.Planning); err != nil && !errors.Is(err, ErrStaleObject) {
				errs <- err
				return
			}
		}
	}

	wg.Add(2)
	go commit(managerA, "dark")
	go commit(managerB, "light")
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent two-machine commit: %v", err)
	}

	// Each machine kept its own machine-local state throughout; a shared home
	// would have left one theme.
	machineA, _, err := managerA.domainStore().LoadMachineChecked()
	if err != nil {
		t.Fatalf("load machine A: %v", err)
	}
	machineB, _, err := managerB.domainStore().LoadMachineChecked()
	if err != nil {
		t.Fatalf("load machine B: %v", err)
	}
	if machineA.Theme != "dark" || machineB.Theme != "light" {
		t.Fatalf("themes = %q/%q, want dark/light", machineA.Theme, machineB.Theme)
	}
}

// TestLensAddedOnSecondMachineSurvivesFirstMachineMutation is defect 1. Machine A
// loads, machine B publishes a lens, and machine A then commits a lens of its
// own against the snapshot it loaded before B wrote. The retry-overwrite this
// replaces re-read the revision per attempt, so A's write landed and B's lens
// was gone with no error reported anywhere.
func TestLensAddedOnSecondMachineSurvivesFirstMachineMutation(t *testing.T) {
	homeA, homeB := t.TempDir(), t.TempDir()
	dataDir, managerA := stageSharedWorkspaceOn(t, homeA)
	managerB := joinWorkspaceOn(t, dataDir, homeB)

	// Machine A's session snapshot, taken before B writes anything.
	coordA := managerA.DomainCoordinator()
	domainsA, err := coordA.LoadDomains()
	if err != nil {
		t.Fatalf("machine A load: %v", err)
	}
	baseline := domainsA.Machine.NamedLenses.Clone()

	// Machine B publishes a lens A's snapshot has never seen.
	coordB := managerB.DomainCoordinator()
	domainsB, err := coordB.LoadDomains()
	if err != nil {
		t.Fatalf("machine B load: %v", err)
	}
	domainsB.Machine.NamedLenses = append(domainsB.Machine.NamedLenses.Clone(), adoptionLens(adoptionLensGammaID, "Gamma", "#gamma"))
	if err := coordB.CommitMachine(domainsB.Machine); err != nil {
		t.Fatalf("machine B commit: %v", err)
	}

	// Machine A now adds its own lens on top of its stale snapshot and commits.
	domainsA.Machine.NamedLenses = append(baseline.Clone(), adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha"))
	if err := coordA.CommitMachine(domainsA.Machine); err != nil {
		t.Fatalf("machine A commit: %v", err)
	}

	published := workspaceLenses(t, managerA)
	names := strings.Join(lensNames(published), "|")
	if !strings.Contains(names, "Gamma") {
		t.Fatalf("published lenses = %v, want machine B's Gamma to survive", lensNames(published))
	}
	if !strings.Contains(names, "Alpha") {
		t.Fatalf("published lenses = %v, want machine A's Alpha to be written", lensNames(published))
	}
	// The machine copy has to carry the merge too, or the fallback beside
	// lenses.json would claim a collection the workspace no longer holds.
	if !strings.Contains(strings.Join(lensNames(domainsA.Machine.NamedLenses), "|"), "Gamma") {
		t.Fatalf("machine A in-memory lenses = %v, want the merged set", lensNames(domainsA.Machine.NamedLenses))
	}
}

// TestCommitCatalogRefusesStaleSessionBaseline is the other half of defect 1: the
// synced domains refuse rather than merge, because a catalog or planning commit
// rides a real transaction the caller can retry, and there is no order-stable
// join for them the way MergeNamedLenses is one for lenses.
func TestCommitCatalogRefusesStaleSessionBaseline(t *testing.T) {
	homeA, homeB := t.TempDir(), t.TempDir()
	dataDir, managerA := stageSharedWorkspaceOn(t, homeA)
	managerB := joinWorkspaceOn(t, dataDir, homeB)

	coordA := managerA.DomainCoordinator()
	domainsA, err := coordA.LoadDomains()
	if err != nil {
		t.Fatalf("machine A load: %v", err)
	}

	// B advances both synced domains after A's snapshot was taken.
	coordB := managerB.DomainCoordinator()
	domainsB, err := coordB.LoadDomains()
	if err != nil {
		t.Fatalf("machine B load: %v", err)
	}
	if len(domainsB.Catalog.Projects) == 0 {
		t.Fatal("bootstrap should have cataloged the system Unfiled project")
	}
	domainsB.Catalog.Projects[0].Description = "written by machine B"
	if err := coordB.CommitCatalog(domainsB.Catalog); err != nil {
		t.Fatalf("machine B catalog commit: %v", err)
	}
	// Planning has to actually change: an identical rewrite hashes to the same
	// revision, so it is not a concurrent update at all and A would be right to
	// accept it.
	domainsB.Planning.Today = []ProjectTaskLocator{{ProjectID: domainsB.Catalog.Projects[0].ID, TaskID: "b-task"}}
	if err := coordB.CommitPlanning(domainsB.Planning); err != nil {
		t.Fatalf("machine B planning commit: %v", err)
	}

	domainsA.Catalog.Projects[0].Description = "written by machine A"
	domainsA.Planning.Today = []ProjectTaskLocator{{ProjectID: domainsA.Catalog.Projects[0].ID, TaskID: "a-task"}}
	if err := coordA.CommitCatalog(domainsA.Catalog); !errors.Is(err, ErrStaleObject) {
		t.Fatalf("machine A catalog commit = %v, want ErrStaleObject", err)
	}
	if err := coordA.CommitPlanning(domainsA.Planning); !errors.Is(err, ErrStaleObject) {
		t.Fatalf("machine A planning commit = %v, want ErrStaleObject", err)
	}

	// The refusal has to be a refusal, not a partial write.
	catalog, _, err := managerA.domainStore().LoadCatalogChecked()
	if err != nil {
		t.Fatalf("reload catalog: %v", err)
	}
	if catalog.Projects[0].Description != "written by machine B" {
		t.Fatalf("catalog description = %q, want machine B's write intact", catalog.Projects[0].Description)
	}
}
