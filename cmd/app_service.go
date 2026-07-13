package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
	tterrors "github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
	"github.com/sglavoie/tt/internal/workspace"
)

// cliRepository is the production app.Repository backing CLI mutations. Each
// Snapshot loads every catalog project fresh so cross-project dependency
// resolution and cross-project moves see the whole workspace; Commit applies the
// changesets through the transactional commit service (with the workspace undo
// ledger attached) when the workspace lives on the real filesystem, and falls
// back to the legacy raw-save path when the storage repository is a test double.
// This mirrors the dual path SaveProject/projectCommitService already use, so a
// mutation routed through the application layer stays atomic, revision-checked,
// and undoable in production while remaining observable to the in-memory
// repository the CLI command tests inject.
type cliRepository struct {
	cfg     *config.Config
	dataDir string
	svc     *storage.CommitService
}

// newCLIRepository builds the storage-backed application repository for the
// active workspace, sizing the undo ledger from the configured history depth.
func newCLIRepository(cfg *config.Config) *cliRepository {
	dataDir := cfg.DataDir
	depth := config.DefaultUndoHistoryDepth
	if cfg.UndoHistoryDepth != nil {
		depth = *cfg.UndoHistoryDepth
	}
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), depth, []string{dataDir})
	svc := storage.NewCommitServiceWithLedger(config.TransactionsDir(dataDir), []string{dataDir}, ledger)
	return &cliRepository{cfg: cfg, dataDir: dataDir, svc: svc}
}

// useCommitService reports whether the workspace is backed by the real
// filesystem repository, in which case mutations flow through the durable
// transaction journal. Test doubles fall back to the raw-save path.
func (r *cliRepository) useCommitService() bool {
	_, ok := storage.DefaultRepository.(*storage.FileSystemRepository)
	return ok
}

// Snapshot loads a revision-consistent read model over every loadable project so
// the application layer can resolve tasks and cross-project references anywhere
// in the workspace. Projects that fail to load are skipped with a warning rather
// than failing the whole mutation, matching FindTaskGloballyByID's tolerance.
func (r *cliRepository) Snapshot() (app.Snapshot, error) {
	projects := r.cfg.GetAllLoadableProjects()
	loaded := make([]*models.Project, 0, len(projects))
	revisions := make(map[string]string, len(projects))
	for _, p := range projects {
		if p == nil {
			continue
		}
		if err := storage.LoadProject(p); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", p.Name, err)
			continue
		}
		loaded = append(loaded, p)
		revisions[p.ID] = projectDiskRevision(p)
	}
	return app.NewSnapshot(loaded, revisions), nil
}

// projectDiskRevision returns the canonical revision of a project's on-disk
// tasks.md — the exact value the commit service compares ExpectedRevision against.
// It deliberately does NOT recompute the revision from the loaded in-memory
// project: storage.LoadProject applies EnsureDefaultSections (and other load-time
// normalization), so ToProjectDocument(loaded) can diverge from the raw bytes a
// fresh `tt project new` wrote — which would fail the very first optimistic commit
// on a new project with a phantom stale-revision conflict. Reading the disk
// revision keeps the snapshot's expectation aligned with the committer, matching
// the migration contract that a recorded revision comes from the loaded/disk
// document, never an ad-hoc in-memory one. Test-double repositories keep projects
// in memory with no tasks.md on disk, and their commit fallback ignores
// ExpectedRevision, so there the in-memory canonical revision is both correct and
// sufficient.
func projectDiskRevision(p *models.Project) string {
	if _, ok := storage.DefaultRepository.(*storage.FileSystemRepository); !ok {
		return storage.ToProjectDocument(p, nil).Revision
	}
	doc, err := storage.LoadProjectDocument(p)
	if err != nil {
		return storage.ToProjectDocument(p, nil).Revision
	}
	return doc.Revision
}

// Commit applies the changesets atomically. On the real filesystem it drives the
// transaction journal; against a test double it raw-saves each project and
// synthesizes a CommitResult carrying the recomputed revision so callers still
// observe a post-commit revision.
func (r *cliRepository) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	if r.useCommitService() {
		return r.svc.CommitMulti(changesets)
	}
	results := make([]*storage.CommitResult, len(changesets))
	for i, cs := range changesets {
		if cs.Project != nil {
			if err := storage.SaveProject(cs.Project); err != nil {
				return nil, err
			}
		}
		// Apply auxiliary artifacts (config store, notes, blockers) so a test-double
		// repository observes the same durable side effects the transactional commit
		// service would persist. Without this, an atomic add + template use-count
		// bump would drop its config write against the in-memory repository.
		for _, art := range cs.Artifacts {
			if err := applyArtifactChange(art); err != nil {
				return nil, err
			}
		}
		// Apply attachment side effects (currently the cross-project move) directly on
		// disk so a test-double repository relocates attachment bytes the same way the
		// transactional commit service would. The production path resolves these into
		// journaled ops instead; here we mirror only their observable filesystem
		// effect. Attachment files live on the real filesystem under the project path
		// even when the project metadata is held by an in-memory repository.
		for _, op := range cs.AttachmentOps {
			if err := applyAttachmentOp(op); err != nil {
				return nil, err
			}
		}
		revision := ""
		if cs.Project != nil {
			revision = workspace.NewLoader().Build([]*models.Project{cs.Project}).Revision(cs.Project.ID)
		}
		results[i] = &storage.CommitResult{Revision: revision}
	}
	return results, nil
}

// applyArtifactChange writes or deletes a single ChangeSet artifact directly on
// the filesystem. It backs the test-double fallback in cliRepository.Commit; the
// production path routes artifacts through the transactional commit service
// instead. It mirrors the commit service's semantics: a delete tolerates a missing
// target, and a write creates parent directories and defaults the file mode.
func applyArtifactChange(art storage.ArtifactChange) error {
	if art.Path == "" {
		return nil
	}
	if art.Delete {
		if err := os.Remove(art.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(art.Path), config.DirPermissions); err != nil {
		return err
	}
	mode := art.Mode
	if mode == 0 {
		mode = config.FilePermissions
	}
	return util.AtomicWriteFile(art.Path, art.Data, mode)
}

// applyAttachmentOp mirrors a single ChangeSet attachment side effect on the
// filesystem for the test-double fallback in cliRepository.Commit; the production
// path routes attachment ops through the transactional commit service instead. The
// application layer emits the copy kind (file attach/ingest), the move kind
// (cross-project move), and the delete kind (single-attachment removal when a
// filename is set, whole-task deletion otherwise). An unrecognized kind is an error
// rather than a silent drop, so a future op kind cannot slip through unhandled.
func applyAttachmentOp(op storage.AttachmentOp) error {
	switch op.Kind {
	case storage.AttachmentCopy:
		if op.SourcePath == "" {
			return fmt.Errorf("attachment copy op missing source path in fallback commit")
		}
		_, err := storage.CopyAttachment(op.DestProject, op.Task, op.SourcePath)
		return err
	case storage.AttachmentMove:
		return storage.MoveTaskAttachments(op.SourceProject, op.DestProject, op.Task)
	case storage.AttachmentDelete:
		project := op.SourceProject
		if project == nil {
			project = op.DestProject
		}
		if op.Filename != "" {
			return storage.RemoveAttachmentFile(project, op.Task, op.Filename)
		}
		return storage.CleanupTaskAttachments(project, op.Task)
	default:
		return fmt.Errorf("unsupported attachment op kind %d in fallback commit", op.Kind)
	}
}

// encodeConfigArtifacts renders the four synced configuration domains
// (workspace/planning/settings/automation.json) into ChangeSet artifacts so a
// config mutation (e.g. an incremented template use-count, a focus-queue/daily-
// plan cleanup, or a project-catalog change) rides the same transaction as the
// task write it accompanies, keeping the pair atomic. Each artifact is
// byte-compatible with what config.Manager.Save writes and targets a path under
// the workspace data directory (the commit service's approved root), so a
// domain-file reload observes the mutation.
//
// machine.json is deliberately excluded: it is machine-local, lives outside the
// approved root, and its refs are repaired separately post-commit. This replaces
// the pre-cutover single-blob .tt.json artifact, which a domain-file reload would
// no longer read.
func encodeConfigArtifacts(mgr *config.Manager, cfg *config.Config) ([]storage.ArtifactChange, error) {
	domainArts, err := mgr.SyncedDomainArtifacts(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode config artifacts: %w", err)
	}
	arts := make([]storage.ArtifactChange, 0, len(domainArts))
	for _, da := range domainArts {
		arts = append(arts, storage.ArtifactChange{Path: da.Path, Data: da.Data})
	}
	return arts, nil
}

// RawSaveProject implements app.MaintenanceRepository: it rewrites a project's
// tasks.md outside the transactional commit boundary. It is the deliberate
// repair-only escape hatch tt doctor uses, kept distinct from the mutation path.
func (r *cliRepository) RawSaveProject(project *models.Project) error {
	return storage.SaveProject(project)
}

// CommitOps implements app.OpsCommitter: it commits a set of raw file operations —
// a project directory move, a tombstone write, a config rewrite, a recursive delete
// — as ONE atomic journaled transaction on the real filesystem, driving the project
// archive-location, trash, restore, and purge use cases. Against a test-double
// repository (no filesystem transaction journal) it applies the same operations
// directly on disk, mirroring the observable effect the transactional path would
// produce, so command tests exercise the same routing without a live journal.
func (r *cliRepository) CommitOps(ops []transaction.Op) error {
	if r.useCommitService() {
		return r.svc.CommitOps(ops)
	}
	for _, op := range ops {
		if err := applyRawOp(op); err != nil {
			return err
		}
	}
	return nil
}

// applyRawOp mirrors a single journal operation on the filesystem for the
// test-double fallback in cliRepository.CommitOps. A write creates parent
// directories and defaults the mode; a move renames source onto target after
// ensuring the destination parent exists; a delete tolerates a missing target.
func applyRawOp(op transaction.Op) error {
	switch op.Kind {
	case transaction.OpWrite:
		if err := os.MkdirAll(filepath.Dir(op.Target), config.DirPermissions); err != nil {
			return err
		}
		mode := op.Mode
		if mode == 0 {
			mode = config.FilePermissions
		}
		return util.AtomicWriteFile(op.Target, op.Data, mode)
	case transaction.OpMove:
		if err := os.MkdirAll(filepath.Dir(op.Target), config.DirPermissions); err != nil {
			return err
		}
		return os.Rename(op.Source, op.Target)
	case transaction.OpDelete:
		if err := os.Remove(op.Target); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported raw op kind %q in fallback commit", op.Kind)
	}
}

// newAppService constructs the application service the CLI adapters drive. The
// service owns the storage-backed repository, the system clock, and the UUID
// generator; every task/project mutation command routes through one of its use
// cases instead of orchestrating load/mutate/save itself.
func newAppService(cfg *config.Config) *app.Service {
	return app.NewService(newCLIRepository(cfg), nil, nil)
}

// resolveAppTaskID applies the strict qualified-reference rules at the adapter
// edge and returns the task's stable UUID, which the application use cases take
// as their target. It reuses FindTaskGloballyByID so a bare numeric short ID is
// accepted only when it is unique across the workspace, a cross-project target
// must be qualified (slug:#short-id / slug:uuid), and ambiguity is reported as an
// error listing candidates rather than silently acting on the wrong task.
func resolveAppTaskID(cfg *config.Config, ref string) (string, error) {
	task, _, err := FindTaskGloballyByID(cfg, ref)
	if err != nil {
		return "", err
	}
	if task == nil {
		return "", tterrors.NewTaskNotFoundError(ref)
	}
	return task.ID, nil
}

// translateAppError maps the application layer's presentation-neutral typed
// errors onto the CLI's error vocabulary so command output stays consistent with
// the legacy paths. Errors that carry no CLI-specific translation are returned
// unchanged (their messages are already user-facing).
func translateAppError(err error) error {
	if err == nil {
		return nil
	}
	var transErr *app.TransitionError
	var inputErr *app.InvalidInputError
	switch {
	case errors.Is(err, app.ErrTaskNotFound):
		return err
	case errors.Is(err, app.ErrProjectNotFound):
		return err
	case errors.Is(err, storage.ErrStaleRevision):
		return fmt.Errorf("the workspace changed since this command loaded it; please retry: %w", err)
	case errors.Is(err, app.ErrParentHasIncompleteChildren):
		// *app.ChildrenBlockedError already renders "task #N has M incomplete
		// children:" plus each blocking child, mirroring the legacy completion
		// invariant message; surface it verbatim (it carries no "app:" prefix).
		return err
	case errors.Is(err, app.ErrDependencyBlocked):
		return err
	case errors.As(err, &transErr):
		return errors.New(strings.TrimPrefix(transErr.Error(), "app: "))
	case errors.As(err, &inputErr):
		return errors.New(strings.TrimPrefix(inputErr.Error(), "app: "))
	default:
		return err
	}
}

// appMutationRetries bounds how many times a task mutation is retried when the
// commit races another writer and the storage layer reports a stale revision.
// Each attempt loads a fresh snapshot inside the use case, so a retry re-reads
// the latest workspace state before re-applying the mutation — closing the
// last-write-wins window the pre-application-layer CLI had (session 22 gate).
const appMutationRetries = 5

// withTaskRetry runs a task mutation, retrying on a stale-revision conflict up to
// appMutationRetries times. A non-conflict error (or success) returns
// immediately. The mutation closure re-invokes the use case, which re-snapshots
// the workspace each call, so retries converge instead of clobbering.
func withTaskRetry(mutate func() (app.TaskResult, error)) (app.TaskResult, error) {
	return withResultRetry(mutate)
}

// withResultRetry is the generic form of withTaskRetry for use cases whose result
// is a richer struct than app.TaskResult (recurrence and attachment mutations).
// It applies the same stale-revision retry policy: each attempt re-invokes the use
// case, which re-snapshots the workspace, so retries converge on the latest state
// instead of clobbering a concurrent writer.
func withResultRetry[T any](mutate func() (T, error)) (T, error) {
	var res T
	var err error
	for attempt := 0; attempt < appMutationRetries; attempt++ {
		res, err = mutate()
		if err == nil || !errors.Is(err, storage.ErrStaleRevision) {
			return res, err
		}
	}
	return res, err
}

// MutationResult is the standardized JSON envelope every task-mutation command
// emits under --json. It centers on the committed transaction outcome (the
// project's post-commit revision), the canonical qualified reference of the
// affected task, its owning project, and the task's final workflow state, so a
// scripting caller gets a stable, adapter-agnostic record of what changed.
type MutationResult struct {
	// Revision is the affected project's post-commit canonical revision — the
	// stable identifier of the committed transaction outcome for that project.
	Revision string `json:"revision"`
	// Task is the canonical "slug:#short-id" cross-project reference.
	Task string `json:"task"`
	// TaskID is the affected task's stable UUID.
	TaskID string `json:"task_id"`
	// Project is the owning project's slug.
	Project string `json:"project"`
	// State is the task's canonical workflow state after the mutation.
	State string `json:"state"`
	// Title is the affected task's current title.
	Title string `json:"title"`
}

// mutationResultFrom builds the standardized envelope from an application
// TaskResult.
func mutationResultFrom(res app.TaskResult) MutationResult {
	return MutationResult{
		Revision: res.Revision,
		Task:     res.QualifiedRef,
		TaskID:   res.TaskID,
		Project:  res.ProjectSlug,
		State:    string(res.State),
		Title:    res.Title,
	}
}

// emitMutation renders the outcome of a task mutation: JSON envelope under
// --json, nothing under --quiet, or a human-readable line otherwise.
func emitMutation(out MutateOutput, verb string, res app.TaskResult) error {
	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printMutationJSON(res)
	}
	fmt.Printf("%s %s: %s\n", verb, res.QualifiedRef, res.Title)
	return nil
}

// printMutationJSON renders the standardized mutation envelope for commands
// whose human output is bespoke (comments, epic baseline) but whose --json
// output follows the shared MutationResult convention.
func printMutationJSON(res app.TaskResult) error {
	data, err := json.MarshalIndent(mutationResultFrom(res), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
