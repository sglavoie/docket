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
	"github.com/sglavoie/tt/internal/util"
)

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
// fails closed on auxiliary artifacts or attachment side effects rather than
// silently dropping them — the primary feature mutations this repository serves
// (capture, state transition, priority, completion) carry neither, so the
// fallback stays observably faithful for the paths that actually use it.
func (r *tuiAppRepository) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	if r.useCommitService() {
		return r.svc.CommitMulti(changesets)
	}
	results := make([]*storage.CommitResult, len(changesets))
	for i, cs := range changesets {
		if len(cs.Artifacts) > 0 || len(cs.AttachmentOps) > 0 {
			return nil, fmt.Errorf("tui fallback commit cannot apply auxiliary artifacts or attachment ops")
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
