package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Service is the application-layer entry point. It owns the injected mutation
// repository, clock, and ID generator, and exposes one method per task-mutation
// use case. Every mutating method follows the same shape: load a
// revision-consistent snapshot, resolve the target, capture the expected
// revision, apply domain rules, run the shared post-mutation hooks, and commit
// exactly one ChangeSet.
type Service struct {
	repo  Repository
	clock Clock
	ids   IDGenerator
	notes WorkspaceNotePort
	// planning is the default planning-artifact builder consulted by direct state
	// changes when a call does not supply its own. It
	// folds Today/Next exclusivity into the same ledger transaction as the task
	// write for every adapter, so rule-driven state changes cannot bypass durable
	// reconciliation. Nil disables it.
	planning PlanningArtifactBuilder
}

// WithPlanningArtifacts sets the default planning-artifact builder used by the
// service's direct state changes. It returns the service
// for chaining at construction. A per-call PlanningArtifacts still overrides
// this default.
func (s *Service) WithPlanningArtifacts(builder PlanningArtifactBuilder) *Service {
	s.planning = builder
	return s
}

// defaultedPlanningArtifacts returns the per-call builder when set, otherwise the
// service default.
func (s *Service) defaultedPlanningArtifacts(builder PlanningArtifactBuilder) PlanningArtifactBuilder {
	if builder != nil {
		return builder
	}
	return s.planning
}

// NewServiceWithArtifacts constructs a Service with an explicit note port. A nil
// port falls back to storage. Callers that need global notes inject
// NewStorageWorkspaceNotePort with the active workspace data directory.
func NewServiceWithArtifacts(repo Repository, clock Clock, ids IDGenerator, notes WorkspaceNotePort) *Service {
	if clock == nil {
		clock = SystemClock()
	}
	if ids == nil {
		ids = UUIDGenerator()
	}
	if notes == nil {
		notes = NewStorageWorkspaceNotePort("")
	}
	return &Service{repo: repo, clock: clock, ids: ids, notes: notes}
}

// resolveProject loads a snapshot and resolves a project by its ID, returning the
// project and the revision it was loaded at. Project-scoped mutations and
// queries (notes and workspace metadata) use it the way resolveTask serves
// single-task mutations.
func (s *Service) resolveProject(projectID string) (*models.Project, string, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, "", invalidInput("project", "id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return nil, "", err
	}
	project := snap.ProjectByID(projectID)
	if project == nil {
		return nil, "", fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}
	return project, snap.Revision(project.ID), nil
}

// resolveTaskSelection validates and resolves either an unqualified task UUID or
// a project-qualified locator against one already-loaded snapshot. The two forms
// are mutually exclusive. All mutation paths use this resolver so malformed
// locators, missing tasks, and ambiguous IDs fail before model state changes.
func resolveTaskSelection(snap Snapshot, taskID string, locator TaskLocator) (TaskRef, error) {
	taskID = strings.TrimSpace(taskID)
	locator.ProjectID = strings.TrimSpace(locator.ProjectID)
	locator.TaskID = strings.TrimSpace(locator.TaskID)
	hasLocator := locator.ProjectID != "" || locator.TaskID != ""
	if taskID != "" && hasLocator {
		return TaskRef{}, invalidInput("task", "provide task id or task locator, not both")
	}
	if hasLocator {
		if !locator.Valid() {
			return TaskRef{}, invalidInput("task", "project id and task id are required")
		}
		ref, ok := snap.LookupLocator(locator)
		if !ok {
			return TaskRef{}, fmt.Errorf("%w: %s/%s", ErrTaskNotFound, locator.ProjectID, locator.TaskID)
		}
		return ref, nil
	}
	if taskID == "" {
		return TaskRef{}, invalidInput("task", "id is required")
	}
	ref, status := snap.LookupTaskID(taskID)
	switch status {
	case TaskLookupUnique:
		return ref, nil
	case TaskLookupAmbiguous:
		return TaskRef{}, fmt.Errorf("%w: %s; provide project ownership", ErrAmbiguousTaskID, taskID)
	default:
		return TaskRef{}, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
}

// loadTaskSelection loads a snapshot and resolves one validated target,
// returning the owning project's optimistic-concurrency revision.
func (s *Service) loadTaskSelection(taskID string, locator TaskLocator) (TaskRef, Snapshot, string, error) {
	snap, err := s.repo.Snapshot()
	if err != nil {
		return TaskRef{}, nil, "", err
	}
	ref, err := resolveTaskSelection(snap, taskID, locator)
	if err != nil {
		return TaskRef{}, nil, "", err
	}
	return ref, snap, snap.Revision(ref.Project.ID), nil
}

// mutateTask is the shared runner for every single-task mutation. It
// resolves the task, applies mutate (which enforces domain rules and may return
// a typed error), runs the post-mutation hooks, and commits one ChangeSet
// carrying the captured expected revision plus the undo description. A mutate
// error aborts before any commit, so a rejected mutation never persists.
func (s *Service) mutateTask(locator TaskLocator, undoAction, undoContext string, mutate func(TaskRef) error) (TaskResult, error) {
	ref, _, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskResult{}, err
	}
	if err := mutate(ref); err != nil {
		return TaskResult{}, err
	}
	return s.commitProject(ref, revision, undoAction, undoContext)
}

// commitProject runs the post-mutation hooks on the resolved project and commits
// one ChangeSet, returning the presentation-neutral result built from the new
// revision. Capture uses it directly (the new task is appended before calling).
func (s *Service) commitProject(ref TaskRef, expectedRevision, undoAction, undoContext string) (TaskResult, error) {
	return s.commitProjectWithArtifacts(ref, expectedRevision, nil, undoAction, undoContext)
}

// commitProjectWithSideEffects commits a project write plus zero or more auxiliary
// durable artifacts and zero or more attachment file side effects (copy/delete) as
// ONE ChangeSet, so a mutation whose side effects live outside tasks.md stays
// atomic with the task write — either the task and its side effects all land or
// none do. It is the single-project analog of commitProjects, carrying the same
// pair of side-effect kinds projectCommit does for a multi-project transaction.
func (s *Service) commitProjectWithSideEffects(ref TaskRef, expectedRevision string, artifacts []storage.ArtifactChange, ops []storage.AttachmentOp, undoAction, undoContext string) (TaskResult, error) {
	applyPostMutationHooks(ref.Project)
	results, err := s.repo.Commit(storage.ChangeSet{
		Project:          ref.Project,
		ExpectedRevision: expectedRevision,
		Base:             ref.Base,
		Artifacts:        artifacts,
		AttachmentOps:    ops,
		UndoAction:       undoAction,
		UndoContext:      undoContext,
	})
	if err != nil {
		return TaskResult{}, err
	}
	revision := ""
	transactionID := ""
	if len(results) > 0 && results[0] != nil {
		revision = results[0].Revision
		transactionID = results[0].TransactionID
	}
	return newTaskResult(ref, revision, transactionID), nil
}

// commitProjectWithArtifacts is the artifacts-only spelling of
// commitProjectWithSideEffects, kept for the mutations that carry no attachment
// side effect at all (planning writes, state changes, line edits).
func (s *Service) commitProjectWithArtifacts(ref TaskRef, expectedRevision string, artifacts []storage.ArtifactChange, undoAction, undoContext string) (TaskResult, error) {
	return s.commitProjectWithSideEffects(ref, expectedRevision, artifacts, nil, undoAction, undoContext)
}

// projectCommit pairs a mutated project with the revision it was loaded at, so a
// multi-project transaction can carry one optimistic-concurrency expectation per
// project. It may also carry auxiliary durable artifacts (e.g. a config store with
// reassigned planning refs) and attachment file side effects (e.g. a cross-project
// attachment move) that must land in the same transaction as the project write.
type projectCommit struct {
	project          *models.Project
	expectedRevision string
	// base is the exact lossless document retained by the snapshot that supplied
	// project and expectedRevision.
	base *storage.ProjectDocument
	// artifacts are auxiliary durable files committed in the same transaction.
	artifacts []storage.ArtifactChange
	// attachmentOps are attachment file side effects (move/copy/delete) committed
	// in the same transaction as the project write.
	attachmentOps []storage.AttachmentOp
}

// groupedCommitOutcome retains the storage result for each changeset.
type groupedCommitOutcome struct {
	results []*storage.CommitResult
}

// commitProjects runs the post-mutation hooks on every project and commits their
// ChangeSets as ONE transaction, so a cascade that spans projects (a cross-project
// move) applies atomically or not at all. Any per-commit
// artifacts and attachment ops ride the same transaction. The storage results are
// retained in commit order. A stale revision on any project aborts the whole commit.
func (s *Service) commitProjects(commits []projectCommit, undoAction, undoContext string) (groupedCommitOutcome, error) {
	changesets := make([]storage.ChangeSet, len(commits))
	for i, c := range commits {
		applyPostMutationHooks(c.project)
		changesets[i] = storage.ChangeSet{
			Project:          c.project,
			ExpectedRevision: c.expectedRevision,
			Base:             c.base,
			Artifacts:        c.artifacts,
			AttachmentOps:    c.attachmentOps,
			UndoAction:       undoAction,
			UndoContext:      undoContext,
		}
	}
	results, err := s.repo.Commit(changesets...)
	if err != nil {
		return groupedCommitOutcome{}, err
	}
	return groupedCommitOutcome{results: results}, nil
}

// revisionAt returns the new revision reported for the changeset at index i, or
// "" when the results do not include it.
func revisionAt(results []*storage.CommitResult, i int) string {
	if i < 0 || i >= len(results) || results[i] == nil {
		return ""
	}
	return results[i].Revision
}

// transactionIDAt returns the transaction identity reported for the changeset
// at index i, or "" when the repository could not provide one. Every result in
// a grouped commit should carry the same ID.
func transactionIDAt(results []*storage.CommitResult, i int) string {
	if i < 0 || i >= len(results) || results[i] == nil {
		return ""
	}
	return results[i].TransactionID
}

// applyPostMutationHooks runs post-mutation reconciliation:
// illegal completed-parent-with-incomplete-child states are reconciled, the
// task tree is rebuilt, and root tasks are reordered by completion.
func applyPostMutationHooks(project *models.Project) {
	project.ReconcileParentCompletion()
	project.BuildTaskTree()
	project.ReorderRootTasksByCompletion()
}
