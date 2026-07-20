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
	repo     Repository
	clock    Clock
	ids      IDGenerator
	notes    WorkspaceNotePort
	blockers ProjectBlockerPort
	// planning is the default planning-artifact builder consulted by direct state
	// changes and every workflow cascade when a call does not supply its own. It
	// folds Today/Next exclusivity into the same ledger transaction as the task
	// write for every adapter, so rule-driven state changes cannot bypass durable
	// reconciliation. Nil disables it.
	planning PlanningArtifactBuilder
}

// WithPlanningArtifacts sets the default planning-artifact builder used by the
// service's direct state changes and workflow cascades. It returns the service
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

// NewService constructs a Service backed by the storage note/blocker artifact
// ports. A nil clock or ID generator falls back to the system implementations, so
// production callers can pass only the repository.
func NewService(repo Repository, clock Clock, ids IDGenerator) *Service {
	return NewServiceWithArtifacts(repo, clock, ids, nil, nil)
}

// NewServiceWithArtifacts constructs a Service with explicit, deliberately
// separate note and blocker ports. A nil port falls back to storage. The default
// note port supports project notes; callers that need global notes inject
// NewStorageWorkspaceNotePort with the active workspace data directory.
func NewServiceWithArtifacts(repo Repository, clock Clock, ids IDGenerator, notes WorkspaceNotePort, blockers ProjectBlockerPort) *Service {
	if clock == nil {
		clock = SystemClock()
	}
	if ids == nil {
		ids = UUIDGenerator()
	}
	if notes == nil {
		notes = NewStorageWorkspaceNotePort("")
	}
	if blockers == nil {
		blockers = storageProjectBlockerPort{}
	}
	return &Service{repo: repo, clock: clock, ids: ids, notes: notes, blockers: blockers}
}

// resolveProject loads a snapshot and resolves a project by its ID, returning the
// snapshot, the project, and the revision it was loaded at. Project-scoped
// mutations and queries (notes, blockers, workspace metadata) use it the way
// resolveTask serves single-task mutations.
func (s *Service) resolveProject(projectID string) (Snapshot, *models.Project, string, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, nil, "", invalidInput("project", "id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return nil, nil, "", err
	}
	project := snap.ProjectByID(projectID)
	if project == nil {
		return nil, nil, "", fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}
	return snap, project, snap.Revision(project.ID), nil
}

// loadTask loads a snapshot and resolves a task by its UUID, returning the task
// reference, the snapshot it was resolved from, and the revision the owning
// project was loaded at. Use cases that need workspace-wide context (dependency
// resolution, cross-project moves) consume the snapshot directly; single-project
// mutations go through resolveTask, which discards it.
func (s *Service) loadTask(taskID string) (TaskRef, Snapshot, string, error) {
	if strings.TrimSpace(taskID) == "" {
		return TaskRef{}, nil, "", invalidInput("task", "id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return TaskRef{}, nil, "", err
	}
	ref, ok := snap.Lookup(taskID)
	if !ok {
		return TaskRef{}, nil, "", fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	return ref, snap, snap.Revision(ref.Project.ID), nil
}

// resolveTask loads a snapshot and resolves a task by its UUID, returning the
// task, its owning project, and the revision the project was loaded at.
func (s *Service) resolveTask(taskID string) (TaskRef, string, error) {
	ref, _, revision, err := s.loadTask(taskID)
	return ref, revision, err
}

// mutateTask is the shared runner for every single-task mutation. It resolves
// the task, applies mutate (which enforces domain rules and may return a typed
// error), runs the post-mutation hooks, and commits one ChangeSet carrying the
// captured expected revision plus the undo description. A mutate error aborts
// before any commit, so a rejected mutation never persists.
func (s *Service) mutateTask(taskID, undoAction, undoContext string, mutate func(TaskRef) error) (TaskResult, error) {
	ref, revision, err := s.resolveTask(taskID)
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

// commitProjectWithArtifacts commits a project write plus zero or more auxiliary
// durable artifacts (e.g. a config store carrying a bumped template use-count) as
// ONE ChangeSet, so a mutation whose side effects live outside tasks.md stays
// atomic with the task write — either the task and its side effects all land or
// none do. It is the single-project analog of the note/blocker commitArtifact
// path, generalized to carry an already-encoded artifact set the caller assembled.
func (s *Service) commitProjectWithArtifacts(ref TaskRef, expectedRevision string, artifacts []storage.ArtifactChange, undoAction, undoContext string) (TaskResult, error) {
	generated := applyPostMutationHooks(ref.Project)
	results, err := s.repo.Commit(storage.ChangeSet{
		Project:          ref.Project,
		ExpectedRevision: expectedRevision,
		Base:             ref.Base,
		Artifacts:        artifacts,
		UndoAction:       undoAction,
		UndoContext:      undoContext,
	})
	if err != nil {
		return TaskResult{}, err
	}
	revision := ""
	if len(results) > 0 && results[0] != nil {
		revision = results[0].Revision
	}
	result := newTaskResult(ref, revision)
	result.RecurringGenerated = generated
	return result, nil
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

// groupedCommitOutcome retains both the storage result for each changeset and
// application-level effects produced while preparing that project for commit.
// Keeping recurring counts keyed by project prevents a grouped workflow commit
// from losing which project generated each new instance.
type groupedCommitOutcome struct {
	results              []*storage.CommitResult
	recurringByProjectID map[string]int
}

// commitProjects runs the post-mutation hooks on every project and commits their
// ChangeSets as ONE transaction, so a cascade that spans projects (a cross-project
// move, a workflow move_to_project) applies atomically or not at all. Any per-commit
// artifacts and attachment ops ride the same transaction. The storage results are
// retained in commit order, while recurring-instance counts remain associated with
// their projects. A stale revision on any project aborts the whole commit.
func (s *Service) commitProjects(commits []projectCommit, undoAction, undoContext string) (groupedCommitOutcome, error) {
	changesets := make([]storage.ChangeSet, len(commits))
	recurring := make(map[string]int, len(commits))
	for i, c := range commits {
		recurring[c.project.ID] += applyPostMutationHooks(c.project)
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
	return groupedCommitOutcome{results: results, recurringByProjectID: recurring}, nil
}

// revisionAt returns the new revision reported for the changeset at index i, or
// "" when the results do not include it.
func revisionAt(results []*storage.CommitResult, i int) string {
	if i < 0 || i >= len(results) || results[i] == nil {
		return ""
	}
	return results[i].Revision
}

// applyPostMutationHooks runs the same post-mutation reconciliation the legacy
// SaveProject path runs, so a mutation committed through the application layer
// leaves the project in the identical reconciled shape: dependents auto-unblock,
// illegal completed-parent-with-incomplete-child states are reconciled, recurring
// instances are generated for freshly completed recurring tasks, the task tree is
// rebuilt, and root tasks are reordered by completion. It returns the number of
// recurring instances generated so single-task callers can surface that count.
func applyPostMutationHooks(project *models.Project) int {
	project.CheckAndAutoUnblock()
	project.ReconcileParentCompletion()
	generated := project.GenerateRecurringInstances()
	project.BuildTaskTree()
	project.ReorderRootTasksByCompletion()
	return generated
}
