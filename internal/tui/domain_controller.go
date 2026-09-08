package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/util"
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

// This file provides the production application-service wiring for the TUI. The
// feature coordinators (feature_coordinator.go) drive presentation-neutral
// application use cases; this is the seam that binds those use cases to durable,
// transactional, undoable storage in production.

// tuiAppRepository is the production app.Repository backing TUI feature
// coordinators. Each Snapshot loads every loadable project fresh at its on-disk
// revision so the application layer resolves tasks and cross-project references
// anywhere in the workspace; Commit routes changesets through the transactional
// commit service (with the workspace undo ledger attached) on the real
// filesystem, and falls back to a raw save against a test-double storage
// repository.
//
// It depends on explicit domain-backed capabilities rather than the config.Config
// facade: the catalog domain (via RuntimeDependencies) supplies the loadable
// project set for Snapshot, while the durable data-dir path and explicitly
// injected ledger fix the commit service's transaction and history boundaries.
type tuiAppRepository struct {
	deps    *config.RuntimeDependencies
	dataDir string
	svc     *storage.CommitService
}

// newTUIAppRepository builds the storage-backed application repository for the
// active workspace from explicit domain-backed capabilities. The durable
// data-dir path fixes the transaction and approved-root boundaries, the caller
// owns the ledger's identity and retention policy, and the catalog domain
// supplies Snapshot's loadable project set.
func newTUIAppRepository(dataDir string, deps *config.RuntimeDependencies, ledger *storage.WorkspaceLedger) *tuiAppRepository {
	svc := storage.NewCommitServiceWithLedger(config.TransactionsDir(dataDir), []string{dataDir}, ledger).
		WithConflictStore(storage.NewConflictStore(config.ConflictsDir(dataDir)))
	return &tuiAppRepository{deps: deps, dataDir: dataDir, svc: svc}
}

// useCommitService reports whether the workspace is backed by the real
// filesystem repository, in which case mutations flow through the durable
// transaction journal. Test doubles fall back to the raw-save path.
func (r *tuiAppRepository) useCommitService() bool {
	_, ok := storage.DefaultRepository.(*storage.FileSystemRepository)
	return ok
}

// Snapshot loads a revision-consistent read model over every loadable project.
// Projects that fail to load are skipped with a warning rather than failing the
// whole mutation.
func (r *tuiAppRepository) Snapshot() (app.Snapshot, error) {
	projects, listErr := r.deps.AvailableProjects(r.dataDir)
	if listErr != nil {
		return nil, listErr
	}
	loaded := make([]*models.Project, 0, len(projects))
	revisions := make(map[string]string, len(projects))
	documents := make(map[string]*storage.ProjectDocument, len(projects))
	for _, p := range projects {
		if p == nil {
			continue
		}
		if err := storage.LoadProject(p); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", p.Name, err)
			continue
		}
		doc, err := tuiProjectDiskDocument(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping project %s document: %v\n", p.Name, err)
			continue
		}
		loaded = append(loaded, p)
		revisions[p.ID] = doc.Revision
		documents[p.ID] = doc
	}
	return app.NewSnapshot(loaded, revisions, documents), nil
}

// WorkspaceCensus implements app.WorkspaceCensusSource: it reads every
// unarchived project — parked ones included, in every task state — together
// with the catalog revision that enumerated them, and fails on the first
// project it cannot read.
//
// It deliberately does not reuse Snapshot or AvailableProjects. Snapshot's
// tolerance is right for a single-task edit and wrong for a whole-workspace
// rename: skipping an unreadable project there would quietly under-report what
// a rename covers and then commit that partial answer. Taking the catalog
// through one checked load is what pairs the enumerated project set with the
// revision the commit boundary guards it against.
func (r *tuiAppRepository) WorkspaceCensus() (app.WorkspaceCensus, error) {
	catalog, revision, err := config.NewDomainFileStore(r.dataDir).LoadCatalogChecked()
	if err != nil {
		return app.WorkspaceCensus{}, fmt.Errorf("reading workspace catalog for census: %w", err)
	}
	census := app.WorkspaceCensus{
		CatalogPath:     config.WorkspaceCatalogPath(r.dataDir),
		CatalogRevision: revision,
	}
	for _, cataloged := range catalog.Projects {
		if cataloged.Archived {
			continue
		}
		project := cataloged.ToModel(r.dataDir)
		if err := storage.LoadProject(project); err != nil {
			return app.WorkspaceCensus{}, fmt.Errorf("reading project %s for census: %w", project.Name, err)
		}
		doc, err := tuiProjectDiskDocument(project)
		if err != nil {
			return app.WorkspaceCensus{}, fmt.Errorf("reading project %s document for census: %w", project.Name, err)
		}
		census.Projects = append(census.Projects, app.CensusProject{
			Project:  project,
			Revision: doc.Revision,
			Document: doc,
		})
	}
	return census, nil
}

// tuiProjectDiskDocument returns the exact lossless on-disk document whose
// revision the commit service compares against ExpectedRevision. It reads the
// disk document rather than the load-normalized in-memory project: a fresh
// project's raw bytes can diverge from ToProjectDocument(loaded) after load-time
// normalization, which would otherwise fail the first optimistic commit with a
// phantom stale-revision conflict. Test doubles keep projects in memory with no
// tasks.md, so the in-memory canonical revision is both correct and sufficient.
func tuiProjectDiskDocument(p *models.Project) (*storage.ProjectDocument, error) {
	if _, ok := storage.DefaultRepository.(*storage.FileSystemRepository); !ok {
		return storage.CanonicalProjectDocument(p, nil)
	}
	doc, err := storage.LoadProjectDocument(p)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// Commit applies the changesets atomically. On the real filesystem it drives the
// transaction journal; against a test double it raw-saves each project and
// synthesizes a CommitResult carrying the recomputed revision. The fallback
// fails closed on auxiliary artifacts, attachment side effects, and read-only
// guards rather than silently dropping them — the primary feature mutations this
// repository serves (capture, state transition, priority, completion) carry
// none of them, so the fallback stays observably faithful for the paths that
// actually use it. A dropped guard would be worse than a dropped write: the
// commit would appear to succeed while proving nothing about the state it was
// derived from.
func (r *tuiAppRepository) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	if r.useCommitService() {
		return r.svc.CommitMulti(changesets)
	}
	results := make([]*storage.CommitResult, len(changesets))
	for i, cs := range changesets {
		if len(cs.Artifacts) > 0 || len(cs.AttachmentOps) > 0 {
			return nil, fmt.Errorf("tui fallback commit cannot apply auxiliary artifacts or attachment ops")
		}
		if len(cs.ProjectPreconditions) > 0 || len(cs.ArtifactPreconditions) > 0 {
			return nil, fmt.Errorf("tui fallback commit cannot validate read-only preconditions")
		}
		if cs.Project != nil {
			if err := storage.SaveProject(cs.Project); err != nil {
				return nil, err
			}
		}
		revision := ""
		if cs.Project != nil {
			doc, docErr := storage.CanonicalProjectDocument(cs.Project, cs.Base)
			if docErr != nil {
				return nil, docErr
			}
			revision = doc.Revision
		}
		results[i] = &storage.CommitResult{Revision: revision}
	}
	return results, nil
}

// ReferenceDocuments implements app.ReferenceDocumentSource: it enumerates every
// cataloged project, archived ones included, so a move or delete can repair the
// soft task references held anywhere in the workspace. It deliberately does NOT
// go through Snapshot — widening reference maintenance to the whole catalog must
// never widen the population a picker, a move destination, or a task projection
// can name, which is exactly why AvailableProjects still filters archived
// projects there.
//
// A project whose document cannot be read fails the whole enumeration. Snapshot
// may skip an unloadable project with a warning because a mutation elsewhere is
// still correct without it; reference maintenance may not, because a document
// nobody could inspect is a document whose stale locators nobody could repair.
func (r *tuiAppRepository) ReferenceDocuments() ([]app.ReferenceDocument, error) {
	projects, err := r.deps.AllProjects(r.dataDir)
	if err != nil {
		return nil, err
	}
	documents := make([]app.ReferenceDocument, 0, len(projects))
	for _, p := range projects {
		if p == nil {
			continue
		}
		if err := storage.LoadProject(p); err != nil {
			return nil, fmt.Errorf("load project %s for reference maintenance: %w", p.Name, err)
		}
		doc, rawRevision, err := tuiProjectReferenceDocument(p)
		if err != nil {
			return nil, fmt.Errorf("read project %s for reference maintenance: %w", p.Name, err)
		}
		documents = append(documents, app.ReferenceDocument{
			Project:     p,
			Base:        doc,
			Revision:    doc.Revision,
			RawRevision: rawRevision,
			TasksPath:   storage.TasksFilePath(p.Path),
		})
	}
	return documents, nil
}

// tuiProjectReferenceDocument returns a project's retained document together
// with the raw byte revision of its tasks.md. Both are needed because the two
// commit boundaries that carry a reference rewrite check different things: a
// ChangeSet compares the canonical document revision, an ArtifactChange the raw
// file hash. A project with no tasks.md yet reports an empty raw revision, which
// is the "must be absent" precondition — and such a project owns no references
// to rewrite anyway.
func tuiProjectReferenceDocument(p *models.Project) (*storage.ProjectDocument, string, error) {
	if _, ok := storage.DefaultRepository.(*storage.FileSystemRepository); !ok {
		doc, err := storage.CanonicalProjectDocument(p, nil)
		if err != nil {
			return nil, "", err
		}
		content, err := doc.WriteCurrent()
		if err != nil {
			return nil, "", err
		}
		return doc, storage.RawArtifactRevision([]byte(content)), nil
	}
	content, err := os.ReadFile(storage.TasksFilePath(p.Path))
	if os.IsNotExist(err) {
		doc, docErr := storage.LoadProjectDocument(p)
		if docErr != nil {
			return nil, "", docErr
		}
		return doc, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	doc, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		return nil, "", err
	}
	return doc, storage.RawArtifactRevision(content), nil
}

// CommitArtifactTransaction implements app.ArtifactCommitter for note-only
// mutations. The real workspace path journals note bytes and the matching
// workspace-ledger record as one transaction; test-double repositories fail
// closed because a sequential fallback could violate note batch atomicity.
func (r *tuiAppRepository) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	if !r.useCommitService() {
		return "", fmt.Errorf("tui fallback commit cannot apply an artifact-only transaction")
	}
	return r.svc.CommitArtifactTransaction(request)
}

// CommitOps implements app.OpsCommitter so TUI-owned project lifecycle and
// bootstrap transactions can atomically combine catalog and project artifacts.
func (r *tuiAppRepository) CommitOps(ops []transaction.Op, artifacts []storage.ArtifactChange) error {
	if r.useCommitService() {
		return r.svc.CommitOpsWithArtifacts(ops, artifacts)
	}
	for _, op := range ops {
		switch op.Kind {
		case transaction.OpWrite:
			if err := os.MkdirAll(filepath.Dir(op.Target), config.DirPermissions); err != nil {
				return err
			}
			mode := op.Mode
			if mode == 0 {
				mode = config.FilePermissions
			}
			if err := util.AtomicWriteFile(op.Target, op.Data, mode); err != nil {
				return err
			}
		case transaction.OpMove:
			if err := os.MkdirAll(filepath.Dir(op.Target), config.DirPermissions); err != nil {
				return err
			}
			if err := os.Rename(op.Source, op.Target); err != nil {
				return err
			}
		case transaction.OpDelete:
			if err := os.RemoveAll(op.Target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported transaction operation %q", op.Kind)
		}
	}
	for _, artifact := range artifacts {
		if artifact.Delete {
			if err := os.Remove(artifact.Path); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(artifact.Path), config.DirPermissions); err != nil {
			return err
		}
		mode := artifact.Mode
		if mode == 0 {
			mode = config.FilePermissions
		}
		if err := util.AtomicWriteFile(artifact.Path, artifact.Data, mode); err != nil {
			return err
		}
	}
	return nil
}

func (r *tuiAppRepository) CommitOpsTransaction(request storage.OpsCommitRequest) (string, error) {
	if r.useCommitService() {
		return r.svc.CommitOpsTransaction(request)
	}
	if err := r.CommitOps(request.Ops, request.Artifacts); err != nil {
		return "", err
	}
	return "", nil
}

// newTUIAppService constructs the application service the TUI feature
// coordinators drive. The service owns the storage-backed repository, the system
// clock, and the UUID generator; every primary-view mutation routes through one
// of its use cases instead of the view orchestrating load/mutate/save itself.
func newTUIAppService(dataDir string, deps *config.RuntimeDependencies, ledger *storage.WorkspaceLedger) *app.Service {
	return app.NewServiceWithArtifacts(
		newTUIAppRepository(dataDir, deps, ledger),
		nil,
		nil,
		app.NewStorageWorkspaceNotePort(dataDir),
	)
}
