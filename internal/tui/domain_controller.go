package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// domainController is the shell's domain commit and configuration cluster: the
// startup-loaded snapshot of the three configuration domains, the manager and
// application service every commit against them travels through, the workspace
// history handle those commits record into, and the session-stable project cache
// the loader fills beside them.
//
// It is one cluster rather than two because the six pieces answer one question —
// what durable state is this session operating on, and through what does a
// change to it become durable. A catalog rewrite needs the snapshot for its
// expected revision, the service for the transaction, the manager to decide
// whether the workspace is writable at all, and the cache refreshed afterwards;
// splitting them would mean four clusters that are never used apart.
//
// It owns the state, not the commits. commitCatalogProjectMutation,
// commitRuntimeDomainsWithUndo, ensureSystemUnfiledProject, syncProjectCache and
// every other committer stay AppModel methods, because each one also touches the
// shell state a commit is raised from — the current project, the feedback line, a
// confirmation. What lives here is the answer to "which snapshot, which service,
// which ledger", so a single reader can see the whole composition root of the
// durable side of the shell.
//
// Machine-local state is deliberately not a field of its own. It is read and
// written through runtime alone (Machine, CommitMachine, SetMachineInMemory), so
// the four machine-local keys stay behind the one snapshot that validates them
// and the cluster can add no fifth.
type domainController struct {
	// runtime is the startup-loaded snapshot of the three configuration domains
	// (catalog, planning, machine), their load-time raw revisions, and the
	// workspace data dir. Views read domain data through its narrow accessors
	// without render-time disk loads, config-bearing mutations commit only the
	// changed domain(s) through its revision-checked commit seams, and each commit
	// refreshes the snapshot so the views observe the committed state without a
	// further disk load.
	runtime *config.RuntimeState

	// manager is the configuration manager. It is what a commit path reads to
	// decide the workspace is writable at all: a nil manager, like an empty data
	// dir, routes a domain mutation to its in-memory branch so a test that never
	// rooted a workspace neither attempts nor fails a disk write.
	manager *config.Manager

	// service is the transactional application service the feature coordinators
	// wrap, retained here for the AppModel-owned overlays that mutate
	// synchronously in Update() and need the typed result inline. Routing through
	// it keeps those mutations atomic and ledger-undoable without a direct raw
	// save.
	service *app.Service

	// ledger is the shared workspace history handle the commit service records
	// into and already-open history-aware views read their depth from. Isolated
	// tests may leave it nil to model unavailable history, which is why every
	// reader goes through historyLedger rather than the field.
	ledger *storage.WorkspaceLedger

	// persister loads a project's task tree from storage. It is injectable so a
	// test can drive the shell against an in-memory double.
	persister coordinator.Persister

	// projects preserves the loaded models.Project pointers used by open views.
	// RuntimeState deliberately rehydrates fresh catalog models on every read;
	// this shell-local cache keeps task trees stable while RuntimeState remains
	// the only owner of persisted catalog metadata.
	projects map[string]*models.Project
}

// newDomainController assembles the cluster at composition time from the
// startup-loaded snapshot and the manager rooted at the same workspace. The
// service and the ledger are bound later, in the one place they are built.
func newDomainController(runtime *config.RuntimeState, manager *config.Manager) domainController {
	return domainController{
		runtime:   runtime,
		manager:   manager,
		persister: coordinator.NewProjectService(),
		projects:  make(map[string]*models.Project),
	}
}

// historyLedger returns the shared history handle when this composition makes
// one available. A nil cluster is valid in isolated view tests, and so is a nil
// ledger: both spell "this session has no history to offer".
func (c *domainController) historyLedger() *storage.WorkspaceLedger {
	if c == nil {
		return nil
	}
	return c.ledger
}

// writable reports whether a domain mutation should be committed to disk rather
// than applied in memory. All three parts are required: a snapshot to stamp the
// expected revision with, a data dir to write under, and the service and manager
// the transaction is opened through. A composition missing any of them is a test
// double, and its caller's in-memory state stays authoritative.
func (c *domainController) writable() bool {
	if c == nil || c.runtime == nil {
		return false
	}
	return c.runtime.DataDir() != "" && c.service != nil && c.manager != nil
}

// machine returns the machine-local snapshot, or nil when this session has no
// runtime to read it from. It is the one read of machine-local state the cluster
// offers, and it offers no write: a machine key is committed through
// runtime.CommitMachine beside the workspace transaction it follows, never here.
func (c *domainController) machine() *config.MachineState {
	if c == nil || c.runtime == nil {
		return nil
	}
	return c.runtime.Machine()
}
