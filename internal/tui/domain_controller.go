package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
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

	// syncStatus reads the runtime's sync status for the status strip and the
	// conflict browser. Nil — every isolated test, and every workspace composed
	// without a runtime — reports nothing.
	syncStatus func() (*engine.SyncStatus, error)

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

// This file holds the shell-level helpers that commit one owning configuration
// domain through RuntimeState. Each helper mutates a defensive copy and commits
// it under the runtime's optimistic-concurrency check.

// commitPlanningToRuntime commits a planning mutation through a runtime snapshot
// without touching the legacy config.Config facade. It is the view-level
// counterpart to AppModel.commitPlanningMutation for the focus views, which hold
// their own runtime reference and drive planning-owned reads/writes off it. A nil
// or non-writable runtime is a silent no-op so tests that never root a writable
// workspace do not attempt (or fail) a disk commit; the caller's in-memory shell
// state remains authoritative.
// commitCatalogProjectMutation is the ledger-aware catalog boundary for
// user-facing project pin and restore changes. The catalog rewrite and its undo
// record are one application transaction.
func (m *AppModel) commitCatalogProjectMutation(
	action string,
	context string,
	projectIDs []string,
	mutate func(*config.WorkspaceCatalog) error,
) (string, error) {
	if m.domain.runtime == nil {
		return "", nil
	}
	catalog := coordinator.CloneWorkspaceCatalog(m.domain.runtime.Catalog())
	if err := mutate(catalog); err != nil {
		return "", err
	}
	if !m.domain.writable() {
		m.domain.runtime.SetCatalogInMemory(catalog)
		m.syncProjectCache()
		return "", nil
	}
	artifacts, err := coordinator.CatalogDomainArtifact(m.domain.runtime, catalog)
	if err != nil {
		return "", err
	}
	transactionID, err := m.domain.service.CommitProjectArtifactsResult(storage.ArtifactCommitRequest{
		Artifacts:   artifacts,
		ProjectIDs:  projectIDs,
		UndoAction:  action,
		UndoContext: context,
	})
	if err != nil {
		return "", err
	}
	if err := m.domain.runtime.RefreshAfterCommit(); err != nil {
		return "", err
	}
	m.syncProjectCache()
	return transactionID, nil
}

// commitCatalogToRuntime commits a catalog mutation through a runtime snapshot
// without touching the legacy config.Config facade. It is the view-level
// counterpart to AppModel.commitCatalogMutation for views (project creator/
// editor/cycle manager) that hold their own runtime reference and drive
// catalog-owned writes off it. A nil or non-writable runtime applies the
// mutation in memory so runtime.Catalog() observes it; the caller's in-memory
// shell state remains authoritative.
func cloneRuntimeDomains(runtime *config.RuntimeState) *config.Domains {
	if runtime == nil {
		return nil
	}
	return &config.Domains{
		Catalog:  coordinator.CloneWorkspaceCatalog(runtime.Catalog()),
		Planning: coordinator.ClonePlanningState(runtime.Planning()),
		Machine:  runtime.Machine().Clone(),
	}
}

// runtimeDomainsCommitResult distinguishes the authoritative synced workspace
// transaction from the machine-local follow-up, which cannot share the same
// transaction root. A caller must publish the workspace result when
// WorkspaceCommitted is true even if MachineErr is non-nil, then queue the
// idempotent machine repair for retry.
type runtimeDomainsCommitResult struct {
	MachineErr    error
	TransactionID string
}

func runtimeDomainsMachineFailure(result runtimeDomainsCommitResult, machineErr error) (runtimeDomainsCommitResult, error) {
	result.MachineErr = machineErr
	return result, nil
}

func (m *AppModel) commitRuntimeDomainsWithUndo(
	domains *config.Domains,
	action string,
	context string,
	projectIDs []string,
) (runtimeDomainsCommitResult, error) {
	if m == nil || m.domain.runtime == nil || domains == nil {
		return runtimeDomainsCommitResult{}, nil
	}
	if !m.domain.writable() {
		m.domain.runtime.SetDomainsInMemory(domains)
		m.syncProjectCache()
		return runtimeDomainsCommitResult{}, nil
	}
	arts, err := syncedDomainArtifacts(
		m.domain.runtime,
		domains.Catalog,
		domains.Planning,
	)
	if err != nil {
		return runtimeDomainsCommitResult{}, err
	}
	result := runtimeDomainsCommitResult{}
	if action == "" {
		if err := m.domain.service.CommitConfig(arts...); err != nil {
			return runtimeDomainsCommitResult{}, err
		}
	} else {
		transactionID, err := m.domain.service.CommitProjectArtifactsResult(storage.ArtifactCommitRequest{
			Artifacts:   arts,
			ProjectIDs:  projectIDs,
			UndoAction:  action,
			UndoContext: context,
		})
		if err != nil {
			return runtimeDomainsCommitResult{}, err
		}
		result.TransactionID = transactionID
	}
	if err := m.domain.runtime.CommitMachine(domains.Machine); err != nil {
		// CommitMachine may have failed before writing or only while refreshing
		// after a successful write. Either way, advance to the authoritative
		// synced transaction when possible; the queued repair is idempotent.
		_ = m.domain.runtime.RefreshAfterCommit()
		m.syncProjectCache()
		return runtimeDomainsMachineFailure(result, err)
	}
	m.syncProjectCache()
	return result, nil
}
