package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// TestDomainControllerWritableRequiresACompleteComposition pins the predicate the
// commit paths branch on. A domain mutation is committed to disk only when the
// whole composition is present; any missing piece is a test double, and its
// caller's in-memory state stays authoritative.
func TestDomainControllerWritableRequiresACompleteComposition(t *testing.T) {
	t.Parallel()

	manager := config.NewManager(filepath.Join(t.TempDir(), "tt.json"))
	service := &app.Service{}
	rooted := config.EmptyRuntimeState(t.TempDir())

	cases := []struct {
		name       string
		controller *domainController
		want       bool
	}{
		{"nil cluster", nil, false},
		{"no snapshot", &domainController{manager: manager, service: service}, false},
		{"no data dir", &domainController{
			runtime: config.EmptyRuntimeState(""), manager: manager, service: service,
		}, false},
		{"no service", &domainController{runtime: rooted, manager: manager}, false},
		{"no manager", &domainController{runtime: rooted, service: service}, false},
		{"complete", &domainController{runtime: rooted, manager: manager, service: service}, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.controller.writable(); got != testCase.want {
				t.Fatalf("writable() = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestDomainControllerReadsAreNilSafe pins that an isolated view test may leave
// the cluster empty: no history to offer and no machine-local state to read are
// both valid compositions, so neither reader may assume its field was bound.
func TestDomainControllerReadsAreNilSafe(t *testing.T) {
	t.Parallel()

	var absent *domainController
	if absent.historyLedger() != nil {
		t.Error("a nil cluster offers no history")
	}
	if absent.machine() != nil {
		t.Error("a nil cluster offers no machine-local state")
	}

	empty := &domainController{}
	if empty.historyLedger() != nil {
		t.Error("an unbound ledger spells no history, not a panic")
	}
	if empty.machine() != nil {
		t.Error("an unbound snapshot spells no machine-local state, not a panic")
	}

	dataDir := t.TempDir()
	bound := &domainController{
		runtime: config.EmptyRuntimeState(dataDir),
		ledger:  storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), []string{dataDir}),
	}
	if bound.historyLedger() == nil {
		t.Error("a bound ledger is what historyLedger returns")
	}
	if bound.machine() == nil {
		t.Error("a bound snapshot answers the machine-local read")
	}
}

// TestNewDomainControllerBindsWhatCompositionKnows pins the split between what
// the constructor can bind and what only the composition root can: the snapshot,
// the manager, the default persister, and an empty project cache are available at
// construction, while the service and the ledger are built later from the same
// workspace paths and bound there.
func TestNewDomainControllerBindsWhatCompositionKnows(t *testing.T) {
	t.Parallel()

	runtime := config.EmptyRuntimeState(t.TempDir())
	manager := config.NewManager(filepath.Join(t.TempDir(), "tt.json"))

	controller := newDomainController(runtime, manager)
	if controller.runtime != runtime || controller.manager != manager {
		t.Fatal("the constructor binds the snapshot and the manager it was given")
	}
	if controller.persister == nil {
		t.Error("the constructor binds the default storage-backed persister")
	}
	if controller.projects == nil {
		t.Error("the project cache is allocated so the loader can fill it")
	}
	if len(controller.projects) != 0 {
		t.Errorf("project cache holds %d entries at construction, want 0", len(controller.projects))
	}
	if controller.service != nil || controller.ledger != nil {
		t.Error("the service and the ledger are bound by the composition root, not here")
	}
}

// TestNewAppModelCompletesTheDomainCluster is the composition-root half of the
// same split, and re-homes what the retired shell dependency bundle proved: a
// launched shell carries the snapshot, the manager, the application service every
// commit travels through, the shared workspace ledger those commits record into,
// and a project cache — so a commit path finds a writable workspace rather than
// falling back to its in-memory branch.
func TestNewAppModelCompletesTheDomainCluster(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HOME", dataDir)
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))

	shell, err := NewAppModel(testRuntimeAt(dataDir, newTestConfigAt(dataDir)), manager, withPersister(newMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}

	if shell.domain.runtime == nil {
		t.Error("the launched shell carries the startup-loaded snapshot")
	}
	if shell.domain.manager != manager {
		t.Error("the launched shell carries the manager it was rooted with")
	}
	if shell.domain.service == nil {
		t.Error("the launched shell carries the transactional application service")
	}
	if shell.domain.historyLedger() == nil {
		t.Error("the launched shell carries the shared workspace ledger")
	}
	if shell.domain.projects == nil {
		t.Error("the launched shell carries the session project cache")
	}
	if !shell.domain.writable() {
		t.Error("a launched shell rooted at a workspace commits to disk")
	}
}

// TestDomainClusterHasOneHome pins the domain cluster to its single named
// AppModel field. The snapshot, the manager, the service, the ledger, the
// persister, and the project cache are one answer to what durable state this
// session operates on and through what a change to it becomes durable; a second
// home is how one commit path would come to read a different workspace than
// another.
func TestDomainClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"runtime":       true,
		"configManager": true,
		"appService":    true,
		"deps":          true,
		"persister":     true,
		"projects":      true,
	}
	assertAppModelClusterHasOneHome(t, "domain", clustered)
}

// TestDomainClusterKeepsMachineLocalStateBehindTheSnapshot pins the rule the
// cluster was shaped around: machine-local state has no field of its own, so the
// four machine-local keys stay behind the one snapshot that validates them and
// the cluster can add no fifth.
func TestDomainClusterKeepsMachineLocalStateBehindTheSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HOME", dataDir)
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	runtime := testRuntimeAt(dataDir, newTestConfigAt(dataDir))

	controller := newDomainController(runtime, manager)
	machine := controller.machine()
	if machine == nil {
		t.Fatal("the cluster reads machine-local state through the snapshot")
	}

	machine.AdvisoriesHidden = true
	if err := runtime.CommitMachine(machine); err != nil {
		t.Fatalf("commit machine state: %v", err)
	}
	if !controller.machine().AdvisoriesHidden {
		t.Fatal("a machine commit refreshes the snapshot the cluster reads through")
	}
}
