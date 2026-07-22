package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
	tterrors "github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

// cliRepository is the production app.Repository backing CLI mutations. Each
// Snapshot loads every catalog project fresh so cross-project dependency
// resolution and cross-project moves see the whole workspace; Commit applies the
// changesets through the transactional commit service (with the workspace undo
// ledger attached) when the workspace lives on the real filesystem, and falls
// back to the legacy raw-save path when the storage repository is a test double.
// A mutation routed through the application layer therefore stays atomic,
// revision-checked, and undoable in production while remaining observable to
// the in-memory repository the CLI command tests inject.
type cliRepository struct {
	deps    *config.RuntimeDependencies
	dataDir string
	svc     *storage.CommitService
}

// newCLIRepository builds the storage-backed application repository for the
// active workspace, sizing the undo ledger from the configured history depth.
func newCLIRepository(mgr *config.Manager, deps *config.RuntimeDependencies) *cliRepository {
	if deps == nil {
		deps = mgr.RuntimeDependencies()
	}
	dataDir := catalogWorkspaceDataDir(mgr)
	depth := config.DefaultUndoHistoryDepth
	if settings, err := deps.Settings.LoadSettings(); err == nil && settings.UndoHistoryDepth != nil {
		depth = *settings.UndoHistoryDepth
	}
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), depth, []string{dataDir})
	svc := storage.NewCommitServiceWithLedger(config.TransactionsDir(dataDir), []string{dataDir}, ledger).
		WithConflictStore(storage.NewConflictStore(config.ConflictsDir(dataDir)))
	return &cliRepository{deps: deps, dataDir: dataDir, svc: svc}
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
	projects, err := r.deps.AllLoadableProjects(r.dataDir)
	if err != nil {
		return nil, err
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
		doc, err := projectDiskDocument(p)
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

// projectDiskDocument returns the exact lossless on-disk document whose revision
// the commit service compares against ExpectedRevision and whose opaque blocks
// must survive the next user mutation.
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
func projectDiskDocument(p *models.Project) (*storage.ProjectDocument, error) {
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

// CommitArtifactTransaction implements app.ArtifactCommitter for note-only user
// mutations. It always uses the journal-backed service, even when project
// metadata is supplied by a test-double repository: note artifacts are real
// workspace files and do not require a synthetic tasks.md save.
func (r *cliRepository) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	return r.svc.CommitArtifactTransaction(request)
}

// applyArtifactChange writes or deletes a single ChangeSet artifact directly on
// the filesystem. It backs the test-double fallback in cliRepository.Commit; the
// production path routes artifacts through the transactional commit service
// instead. It mirrors the commit service's observable semantics: a marked
// change first copies an existing primary to its private backup, a delete
// tolerates a missing target, and a write creates parent directories and
// defaults the file mode.
func applyArtifactChange(art storage.ArtifactChange) error {
	if art.Path == "" {
		return nil
	}
	if art.BackupPath != "" {
		if filepath.Clean(art.BackupPath) == filepath.Clean(art.Path) {
			return fmt.Errorf("artifact backup path %q is self-referential", art.BackupPath)
		}
		current, readErr := os.ReadFile(art.Path)
		switch {
		case readErr == nil:
			if mkdirErr := os.MkdirAll(filepath.Dir(art.BackupPath), config.DirPermissions); mkdirErr != nil {
				return mkdirErr
			}
			if writeErr := util.AtomicWriteFile(art.BackupPath, current, config.FilePermissions); writeErr != nil {
				return writeErr
			}
		case os.IsNotExist(readErr):
			// First save: there is no preceding primary value to preserve.
		default:
			return readErr
		}
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

// encodeSyncedDomainArtifacts renders the four synced configuration domains
// (workspace/planning/settings/automation.json) into ChangeSet artifacts so a
// config mutation (e.g. an incremented template use-count, a focus-queue/daily-
// plan cleanup, or a project-catalog change) rides the same transaction as the
// task write it accompanies, keeping the pair atomic. Each artifact is
// byte-compatible with what config.Manager.Save writes and targets a path under
// the workspace data directory (the commit service's approved root), so a
// domain-file reload observes the mutation.
//
// It routes through RuntimeState's startup-loaded DomainCoordinator so every
// artifact carries an optimistic-concurrency revision check — the on-disk
// revision observed when the command loaded, or a require-absent contract when
// the domain did not yet exist. Retaining that original baseline catches a
// concurrent writer at any point after command load; the transaction validates
// it again under the artifact locks before staging.
//
// machine.json is deliberately excluded: it is machine-local, lives outside the
// approved root, and its refs are repaired separately post-commit.
func encodeSyncedDomainArtifacts(runtime *config.RuntimeState, domains *config.Domains) ([]storage.ArtifactChange, error) {
	if domains == nil {
		return nil, fmt.Errorf("configuration domains are required")
	}
	domainArts, err := runtime.CheckedSyncedArtifacts(domains)
	if err != nil {
		return nil, fmt.Errorf("failed to encode config artifacts: %w", err)
	}
	return storageArtifacts(domainArts), nil
}

// encodeCatalogArtifact renders only workspace.json for a project lifecycle
// transaction whose synced configuration effect is catalog-only.
func encodeCatalogArtifact(runtime *config.RuntimeState, catalog *config.WorkspaceCatalog) ([]storage.ArtifactChange, error) {
	artifact, err := runtime.CheckedCatalogArtifact(catalog)
	if err != nil {
		return nil, fmt.Errorf("failed to encode workspace catalog: %w", err)
	}
	return storageArtifacts([]config.DomainArtifact{artifact}), nil
}

// encodeAutomationArtifact renders only automation.json for a task transaction
// whose configuration effect is automation-scoped, such as incrementing a
// template use count. Keeping planning out of this artifact set lets workflow
// focus changes supply their own checked planning artifact without duplicate
// targets or stale unrelated baselines.
func encodeAutomationArtifact(runtime *config.RuntimeState, automation *config.AutomationConfig) ([]storage.ArtifactChange, error) {
	artifact, err := runtime.CheckedAutomationArtifact(automation)
	if err != nil {
		return nil, fmt.Errorf("failed to encode automation config: %w", err)
	}
	return storageArtifacts([]config.DomainArtifact{artifact}), nil
}

func storageArtifacts(domainArts []config.DomainArtifact) []storage.ArtifactChange {
	arts := make([]storage.ArtifactChange, 0, len(domainArts))
	for _, da := range domainArts {
		arts = append(arts, da.StorageChange())
	}
	return arts
}

func commitSyncedDomains(workspace *CLIWorkspace, domains *config.Domains) error {
	artifacts, err := encodeSyncedDomainArtifacts(workspace.Runtime, domains)
	if err != nil {
		return err
	}
	return translateAppError(newWorkspaceAppService(workspace).CommitConfig(artifacts...))
}

// CommitOps implements app.OpsCommitter: it commits raw lifecycle operations and
// revision-checked domain/project artifacts as ONE atomic journaled transaction
// on the real filesystem. Against a test-double repository (no filesystem
// transaction journal) it applies the same effects directly on disk so command
// tests exercise the same routing without a live journal.
func (r *cliRepository) CommitOps(ops []transaction.Op, artifacts []storage.ArtifactChange) error {
	if r.useCommitService() {
		return r.svc.CommitOpsWithArtifacts(ops, artifacts)
	}
	// The test-double path has no production CommitService, but it must preserve
	// the same fail-before-effects revision contract. In particular a rename's
	// revision source can be the old tasks.md path that the raw move consumes.
	if err := validateFallbackArtifactRevisions(artifacts); err != nil {
		return err
	}
	for _, op := range ops {
		if err := applyRawOp(op); err != nil {
			return err
		}
	}
	for _, artifact := range artifacts {
		if err := applyArtifactChange(artifact); err != nil {
			return err
		}
	}
	return nil
}

func validateFallbackArtifactRevisions(artifacts []storage.ArtifactChange) error {
	for i := range artifacts {
		artifact := artifacts[i]
		if artifact.ExpectedRevision == nil {
			continue
		}
		checkPath := artifact.Path
		if artifact.ExpectedRevisionPath != "" {
			checkPath = artifact.ExpectedRevisionPath
		}
		data, err := os.ReadFile(checkPath)
		switch {
		case os.IsNotExist(err):
			if *artifact.ExpectedRevision != "" {
				return &storage.ArtifactConflictError{Path: checkPath, Expected: *artifact.ExpectedRevision}
			}
		case err != nil:
			return fmt.Errorf("reading artifact %s for concurrency check: %w", filepath.Base(checkPath), err)
		default:
			actual := storage.RawArtifactRevision(data)
			if *artifact.ExpectedRevision == "" || actual != *artifact.ExpectedRevision {
				return &storage.ArtifactConflictError{Path: checkPath, Expected: *artifact.ExpectedRevision, Actual: actual}
			}
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
func newAppService(mgr *config.Manager) *app.Service {
	dataDir := catalogWorkspaceDataDir(mgr)
	return app.NewServiceWithArtifacts(newCLIRepository(mgr, nil), nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)
}

// newWorkspaceAppService reuses the command's single RuntimeDependencies
// composition object for lifecycle snapshots and settings instead of rebuilding
// the domain service graph at each mutation boundary.
func newWorkspaceAppService(workspace *CLIWorkspace) *app.Service {
	svc := app.NewServiceWithArtifacts(
		newCLIRepository(workspace.Manager, workspace.Dependencies),
		nil,
		nil,
		app.NewStorageWorkspaceNotePort(workspace.Runtime.DataDir()),
		nil,
	)
	// Fold Today/Next exclusivity into every workflow-state transition committed
	// through the CLI service, so exclusivity holds for `tt task next/state` and
	// every convenience state command. Focus commands override this with an
	// additive builder for the horizon they populate.
	return svc.WithPlanningArtifacts(cliPlanningReconcileArtifactBuilder(workspace))
}

// resolveAppTaskTarget applies the strict qualified-reference rules at the
// adapter edge while retaining project ownership for the application mutation.
// Bare references must be workspace-unique; qualified references keep their
// locator even when another project contains the same task UUID.
func resolveAppTaskTarget(mgr *config.Manager, ref string) (*models.Task, app.TaskLocator, error) {
	task, project, err := findAppTaskGlobally(mgr, ref)
	if err != nil {
		return nil, app.TaskLocator{}, err
	}
	if task == nil || project == nil {
		return nil, app.TaskLocator{}, tterrors.NewTaskNotFoundError(ref)
	}
	return task, app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}, nil
}

// findAppTaskGlobally is the domain-backed task resolver used by application
// adapters. It preserves qualified-reference and ambiguity behavior while
// loading catalog projects through RuntimeDependencies instead of Config.
func findAppTaskGlobally(mgr *config.Manager, query string) (*models.Task, *models.Project, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil, nil
	}
	deps := mgr.RuntimeDependencies()
	dataDir := catalogWorkspaceDataDir(mgr)
	ref := models.ParseTaskRef(query)
	if ref.IsCrossProject() {
		project, err := deps.Project(dataDir, ref.ProjectID)
		if err != nil {
			return nil, nil, err
		}
		if project == nil {
			return nil, nil, tterrors.NewProjectNotFoundError(ref.ProjectID)
		}
		if loadErr := storage.LoadProject(project); loadErr != nil {
			return nil, nil, fmt.Errorf("failed to load project %s: %w", project.Name, loadErr)
		}
		task, err := FindTaskByIdentifier(project, strings.TrimPrefix(ref.TaskID, "#"))
		return task, project, err
	}

	projects, err := deps.AllLoadableProjects(dataDir)
	if err != nil {
		return nil, nil, err
	}
	type match struct {
		task    *models.Task
		project *models.Project
	}
	var matches []match
	if ref.IsNumericShortID() {
		shortID, _ := strconv.Atoi(query)
		for _, project := range projects {
			if err := storage.LoadProject(project); err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", project.Name, err)
				continue
			}
			if task := project.GetTaskByShortID(shortID); task != nil {
				matches = append(matches, match{task: task, project: project})
			}
		}
		if len(matches) > 1 {
			parts := make([]string, min(len(matches), 5))
			for i := range parts {
				parts[i] = fmt.Sprintf("%s:#%d %s", matches[i].project.Slug, matches[i].task.ShortID, matches[i].task.Title)
			}
			candidates := strings.Join(parts, ", ")
			if len(matches) > 5 {
				candidates += fmt.Sprintf(", ... (%d more)", len(matches)-5)
			}
			return nil, nil, tterrors.NewAmbiguousIdentifierError(query, candidates)
		}
	} else {
		lowerQuery := strings.ToLower(query)
		for _, project := range projects {
			if err := storage.LoadProject(project); err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", project.Name, err)
				continue
			}
			for _, task := range project.Tasks {
				if strings.HasPrefix(strings.ToLower(task.ID), lowerQuery) {
					matches = append(matches, match{task: task, project: project})
				}
			}
		}
		if len(matches) > 1 {
			parts := make([]string, min(len(matches), 5))
			for i := range parts {
				parts[i] = fmt.Sprintf("#%d %s (%s)", matches[i].task.ShortID, matches[i].task.Title, shortDisplayID(matches[i].task.ID))
			}
			candidates := strings.Join(parts, ", ")
			if len(matches) > 5 {
				candidates += fmt.Sprintf(", ... (%d more)", len(matches)-5)
			}
			return nil, nil, tterrors.NewAmbiguousIdentifierError(query, candidates)
		}
	}
	if len(matches) == 0 {
		return nil, nil, nil
	}
	return matches[0].task, matches[0].project, nil
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
	case errors.Is(err, storage.ErrMergeConflict):
		// The commit boundary already three-way merged and recorded the conflict;
		// its message names the recorded id and the resolution command. Surface it
		// verbatim so the user is pointed at `tt conflict resolve` rather than a
		// generic retry, and so the app-layer retry never loops on it.
		return err
	case errors.Is(err, storage.ErrStaleRevision), errors.Is(err, storage.ErrStaleArtifact):
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
