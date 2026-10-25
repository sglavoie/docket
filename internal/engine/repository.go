package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
	"github.com/sglavoie/docket/internal/util"
	"github.com/sglavoie/docket/internal/workspace"
)

// Repository is the production app.Repository every frontend's application
// service runs over. Each Snapshot loads every loadable project fresh at its on-disk
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
type Repository struct {
	deps    *config.RuntimeDependencies
	dataDir string
	svc     *storage.CommitService

	// activityLoader retains the completion-activity read across calls, so an
	// unchanged project is not re-read and re-parsed after every commit. The
	// loader is not safe for concurrent use and the repository is reachable from
	// command goroutines, so activityMu owns it.
	activityMu     sync.Mutex
	activityLoader *workspace.Loader
}

// NewRepository builds the storage-backed application repository for the
// active workspace from explicit domain-backed capabilities. The durable
// data-dir path fixes the transaction and approved-root boundaries, the caller
// owns the ledger's identity and retention policy, the catalog domain supplies
// Snapshot's loadable project set, and owner is the transaction owner stamped
// on every manifest — the empty string meaning the running process's machine.
func NewRepository(dataDir string, deps *config.RuntimeDependencies, ledger *storage.WorkspaceLedger, owner string) *Repository {
	svc := storage.NewCommitServiceWithLedger(config.TransactionsDir(dataDir), []string{dataDir}, ledger).
		WithConflictStore(storage.NewConflictStore(config.ConflictsDir(dataDir))).
		OnMachine(owner)
	return &Repository{deps: deps, dataDir: dataDir, svc: svc}
}

// useCommitService reports whether the workspace is backed by the real
// filesystem repository, in which case mutations flow through the durable
// transaction journal. Test doubles fall back to the raw-save path.
func (r *Repository) useCommitService() bool {
	_, ok := storage.DefaultRepository.(*storage.FileSystemRepository)
	return ok
}

// Snapshot loads a revision-consistent read model over every loadable project.
// Projects that fail to load are skipped with a warning rather than failing the
// whole mutation, and are named unavailable beside the ones with no tasks.md on
// disk, so a caller reconciling other state against this read can tell a task
// it could not see from a task that is gone.
func (r *Repository) Snapshot() (app.Snapshot, error) {
	projects, listErr := r.deps.AvailableProjects(r.dataDir)
	if listErr != nil {
		return nil, listErr
	}
	loaded := make([]*models.Project, 0, len(projects))
	revisions := make(map[string]string, len(projects))
	documents := make(map[string]*storage.ProjectDocument, len(projects))
	var unavailable []string
	for _, p := range projects {
		if p == nil {
			continue
		}
		// One read yields both the tasks and the guarded document, so a
		// concurrent save is either wholly visible or wholly absent rather than
		// pairing one version's tasks with the next version's revision. The
		// warning goes to the log: stderr may be the terminal a frontend is drawing on.
		doc, err := storage.LoadProjectWithDocument(p)
		if err != nil {
			logging.Warn("skipping project %s: %v", p.Name, err)
			unavailable = append(unavailable, p.ID)
			continue
		}
		if storage.ProjectTasksFileMissing(p) {
			unavailable = append(unavailable, p.ID)
		}
		loaded = append(loaded, p)
		revisions[p.ID] = doc.Revision
		documents[p.ID] = doc
	}
	return app.NewPartialSnapshot(loaded, revisions, documents, unavailable), nil
}

// CompletionActivityProjects implements app.CompletionActivitySource: the base
// task workspace — neither archived nor parked, the population completion
// activity counts — read through a retained loader. Selection reads the fresh
// catalog before the loader is asked, because a cached project carries the
// catalog metadata of the read that parsed it. A project that fails to load is
// skipped with a warning, as Snapshot skips it. A test double's project has no
// tasks.md to fingerprint, so the loader simply never caches it.
func (r *Repository) CompletionActivityProjects() ([]*models.Project, error) {
	projects, err := r.deps.AvailableProjects(r.dataDir)
	if err != nil {
		return nil, err
	}
	selected := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project == nil || project.Archived || models.ProjectTagsParked(project.Tags) {
			continue
		}
		selected = append(selected, project)
	}
	r.activityMu.Lock()
	defer r.activityMu.Unlock()
	if r.activityLoader == nil {
		r.activityLoader = workspace.NewLoader()
	}
	snapshot := r.activityLoader.Build(selected)
	for _, diagnostic := range snapshot.Diagnostics() {
		logging.Warn("completion activity: skipping project %s: %s", diagnostic.ProjectName, diagnostic.Detail)
	}
	return snapshot.Projects(), nil
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
//
// Each project's tasks, base document, and guarded revision come from one read
// through storage.LoadProjectWithDocument rather than from a load followed by an
// independent document read. Two reads could straddle another process's save and
// hand the preview version A's tasks under version B's revision, which commits
// as an unmerged overwrite of B. One read makes a concurrent save either wholly
// visible or wholly absent, and an absent one expires the preview at the guard.
func (r *Repository) WorkspaceCensus() (app.WorkspaceCensus, error) {
	return r.readWorkspaceCensus(false)
}

// CheckInSetupCensus excludes parked owners before reading their documents.
// It uses a fresh catalog and fails closed on any unreadable active project.
func (r *Repository) CheckInSetupCensus() (app.WorkspaceCensus, error) {
	return r.readWorkspaceCensus(true)
}

func (r *Repository) readWorkspaceCensus(activeOnly bool) (app.WorkspaceCensus, error) {
	catalog, revision, err := config.NewDomainFileStore(r.dataDir).LoadCatalogChecked()
	if err != nil {
		return app.WorkspaceCensus{}, fmt.Errorf("reading workspace catalog for census: %w", err)
	}
	census := app.WorkspaceCensus{
		CatalogPath:     config.WorkspaceCatalogPath(r.dataDir),
		CatalogRevision: revision,
	}
	for _, cataloged := range catalog.Projects {
		if cataloged.Archived || (activeOnly && models.ProjectTagsParked(cataloged.Tags)) {
			continue
		}
		project := cataloged.ToModel(r.dataDir)
		doc, err := storage.LoadProjectWithDocument(project)
		if err != nil {
			return app.WorkspaceCensus{}, fmt.Errorf("reading project %s for census: %w", project.Name, err)
		}
		census.Projects = append(census.Projects, app.CensusProject{
			Project:  project,
			Revision: doc.Revision,
			Document: doc,
		})
	}
	return census, nil
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
func (r *Repository) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	if r.useCommitService() {
		return r.svc.CommitMulti(changesets)
	}
	results := make([]*storage.CommitResult, len(changesets))
	for i, cs := range changesets {
		if len(cs.Artifacts) > 0 || len(cs.AttachmentOps) > 0 {
			return nil, fmt.Errorf("fallback commit cannot apply auxiliary artifacts or attachment ops")
		}
		if len(cs.ProjectPreconditions) > 0 || len(cs.ArtifactPreconditions) > 0 {
			return nil, fmt.Errorf("fallback commit cannot validate read-only preconditions")
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
func (r *Repository) ReferenceDocuments() ([]app.ReferenceDocument, error) {
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
		doc, rawRevision, err := projectReferenceDocument(p)
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

// projectReferenceDocument returns a project's retained document together
// with the raw byte revision of its tasks.md. Both are needed because the two
// commit boundaries that carry a reference rewrite check different things: a
// ChangeSet compares the canonical document revision, an ArtifactChange the raw
// file hash. A project with no tasks.md yet reports an empty raw revision, which
// is the "must be absent" precondition — and such a project owns no references
// to rewrite anyway.
func projectReferenceDocument(p *models.Project) (*storage.ProjectDocument, string, error) {
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
func (r *Repository) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	if !r.useCommitService() {
		return "", fmt.Errorf("fallback commit cannot apply an artifact-only transaction")
	}
	return r.svc.CommitArtifactTransaction(request)
}

// CommitOps implements app.OpsCommitter so project lifecycle and bootstrap
// transactions can atomically combine catalog and project artifacts.
func (r *Repository) CommitOps(ops []transaction.Op, artifacts []storage.ArtifactChange) error {
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

func (r *Repository) CommitOpsTransaction(request storage.OpsCommitRequest) (string, error) {
	if r.useCommitService() {
		return r.svc.CommitOpsTransaction(request)
	}
	if err := r.CommitOps(request.Ops, request.Artifacts); err != nil {
		return "", err
	}
	return "", nil
}
