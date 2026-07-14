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
// transactional, undoable storage in production. It deliberately mirrors the
// CLI's cliRepository (cmd/app_service.go) so a mutation dispatched from the TUI
// commits through the same journaled path with the same optimistic-concurrency
// and undo-ledger semantics as the equivalent CLI command — the whole point of
// routing both adapters through one application layer.

// tuiAppRepository is the production app.Repository backing TUI feature
// coordinators. Each Snapshot loads every loadable project fresh at its on-disk
// revision so the application layer resolves tasks and cross-project references
// anywhere in the workspace; Commit routes changesets through the transactional
// commit service (with the workspace undo ledger attached) on the real
// filesystem, and falls back to a raw save against a test-double storage
// repository — matching the dual path SaveProject and cliRepository already use.
//
// It depends on explicit domain-backed capabilities rather than the config.Config
// facade: the catalog domain (via RuntimeDependencies) supplies the loadable
// project set for Snapshot, and the durable data-dir path plus the settings
// domain size the commit service and its undo ledger.
type tuiAppRepository struct {
	deps    *config.RuntimeDependencies
	dataDir string
	svc     *storage.CommitService
}

// newTUIAppRepository builds the storage-backed application repository for the
// active workspace from explicit domain-backed capabilities: the durable
// data-dir path fixes the transaction, undo-ledger, and approved-root
// boundaries, while the settings domain sizes the undo ledger's history depth
// and the catalog domain (both reachable through deps) supplies Snapshot's
// loadable project set.
func newTUIAppRepository(dataDir string, deps *config.RuntimeDependencies) *tuiAppRepository {
	depth := config.DefaultUndoHistoryDepth
	if settings, err := deps.Settings.LoadSettings(); err == nil && settings.UndoHistoryDepth != nil {
		depth = *settings.UndoHistoryDepth
	}
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), depth, []string{dataDir})
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
// whole mutation, matching FindTaskGloballyByID's and cliRepository's tolerance.
func (r *tuiAppRepository) Snapshot() (app.Snapshot, error) {
	projects, listErr := r.deps.AllLoadableProjects(r.dataDir)
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
// disk document (not the load-normalized in-memory
// project) for the same reason cliRepository.projectDiskDocument does: a fresh
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

// newTUIAppService constructs the application service the TUI feature
// coordinators drive. The service owns the storage-backed repository, the system
// clock, and the UUID generator; every primary-view mutation routes through one
// of its use cases instead of the view orchestrating load/mutate/save itself.
func newTUIAppService(dataDir string, deps *config.RuntimeDependencies) *app.Service {
	return app.NewService(newTUIAppRepository(dataDir, deps), nil, nil)
}
