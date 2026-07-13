package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// WorkspaceResolver centralizes dependency resolution across every project in a
// snapshot. The per-project models helpers (HasIncompleteDependencies,
// HasCircularDependency) conservatively treat any cross-project "projectID:taskID"
// dependency as unresolved because they cannot see other projects. The resolver
// closes that gap: it resolves a qualified reference against the whole workspace
// so a task blocked only by an already-completed task in another project can
// complete, and it detects dependency cycles that span projects.
type WorkspaceResolver struct {
	snap Snapshot
}

// NewWorkspaceResolver builds a resolver over a snapshot.
func NewWorkspaceResolver(snap Snapshot) *WorkspaceResolver {
	return &WorkspaceResolver{snap: snap}
}

// resolveRef resolves a dependency/related reference — a bare task UUID in owner,
// or a "projectID:taskID" cross-project reference — to the target task and the
// project that owns it. It returns ok=false when the reference cannot be resolved
// anywhere in the snapshot.
func (r *WorkspaceResolver) resolveRef(ref string, owner *models.Project) (*models.Task, *models.Project, bool) {
	parsed := models.ParseTaskRef(ref)
	if parsed.IsCrossProject() {
		proj := r.snap.ProjectByID(parsed.ProjectID)
		if proj == nil {
			return nil, nil, false
		}
		task := proj.GetTaskByID(parsed.TaskID)
		return task, proj, task != nil
	}
	if owner == nil {
		return nil, nil, false
	}
	task := owner.GetTaskByID(parsed.TaskID)
	return task, owner, task != nil
}

// HasIncompleteDependencies reports whether task has any dependency that is not
// yet complete, resolving cross-project references across the whole workspace. It
// refines models.HasIncompleteDependencies: a same-project dangling reference is
// tolerated as resolved (BuildTaskTree prunes orphans on load), while a
// cross-project reference that cannot be resolved in the snapshot stays blocking
// (fail-closed), and a resolvable cross-project reference is judged on the actual
// completion of the target task.
func (r *WorkspaceResolver) HasIncompleteDependencies(task *models.Task, owner *models.Project) bool {
	if task == nil {
		return false
	}
	for _, depID := range task.DependsOn {
		dep, _, ok := r.resolveRef(depID, owner)
		if !ok {
			if models.ParseTaskRef(depID).IsCrossProject() {
				return true
			}
			continue
		}
		if !dep.IsComplete() {
			return true
		}
	}
	return false
}

// WouldCreateDependencyCycle reports whether adding a dependency from taskID (in
// owner) onto blockerRef would create a cycle, following DependsOn edges across
// projects. A cycle forms when the blocker already depends — directly or
// transitively — on the task. A reference that leaves the snapshot simply ends
// that path; it cannot close a cycle through an unknown task.
func (r *WorkspaceResolver) WouldCreateDependencyCycle(owner *models.Project, taskID, blockerRef string) bool {
	blocker, blockerHome, ok := r.resolveRef(blockerRef, owner)
	if !ok {
		return false
	}
	targetKey := globalTaskKey(owner.ID, taskID)
	visited := make(map[string]bool)

	var reaches func(t *models.Task, home *models.Project) bool
	reaches = func(t *models.Task, home *models.Project) bool {
		if t == nil || home == nil {
			return false
		}
		key := globalTaskKey(home.ID, t.ID)
		if key == targetKey {
			return true
		}
		if visited[key] {
			return false
		}
		visited[key] = true
		for _, depID := range t.DependsOn {
			next, nextHome, ok := r.resolveRef(depID, home)
			if !ok {
				continue
			}
			if reaches(next, nextHome) {
				return true
			}
		}
		return false
	}

	return reaches(blocker, blockerHome)
}

// globalTaskKey builds a workspace-unique key for a task from its owning project
// ID and its per-project UUID, so cycle detection can compare tasks that live in
// different projects without conflating identical UUIDs (which never collide in
// practice, but the key keeps the traversal unambiguous).
func globalTaskKey(projectID, taskID string) string {
	return projectID + "\x00" + taskID
}

// AddDependency adds a dependency edge from the task onto blockerRef, which is a
// bare task UUID in the same project or a "projectID:taskID" cross-project
// reference. The blocker must resolve in the workspace, must not be the task
// itself, and must not close a dependency cycle (checked across projects). The
// stored reference preserves the qualified form so a later load resolves it to
// the same target.
func (s *Service) AddDependency(taskID, blockerRef string) (TaskResult, error) {
	ref, snap, revision, err := s.loadTask(taskID)
	if err != nil {
		return TaskResult{}, err
	}
	blockerRef = strings.TrimSpace(blockerRef)
	if blockerRef == "" {
		return TaskResult{}, invalidInput("dependency", "blocker reference is required")
	}
	resolver := NewWorkspaceResolver(snap)
	blocker, _, ok := resolver.resolveRef(blockerRef, ref.Project)
	if !ok {
		return TaskResult{}, fmt.Errorf("%w: blocker %s", ErrTaskNotFound, blockerRef)
	}
	if blocker.ID == ref.Task.ID && !models.ParseTaskRef(blockerRef).IsCrossProject() {
		return TaskResult{}, invalidInput("dependency", "a task cannot depend on itself")
	}
	if resolver.WouldCreateDependencyCycle(ref.Project, ref.Task.ID, blockerRef) {
		return TaskResult{}, invalidInput("dependency", "would create a circular dependency")
	}
	if err := ref.Task.AddDependency(blockerRef); err != nil {
		return TaskResult{}, invalidInput("dependency", err.Error())
	}
	return s.commitProject(ref, revision, "Add Dependency", blockerRef)
}

// RemoveDependency drops a dependency edge. Removing an edge the task does not
// carry is a no-op that still commits the (unchanged) project.
func (s *Service) RemoveDependency(taskID, blockerRef string) (TaskResult, error) {
	return s.mutateTask(taskID, "Remove Dependency", blockerRef, func(ref TaskRef) error {
		blockerRef = strings.TrimSpace(blockerRef)
		if blockerRef == "" {
			return invalidInput("dependency", "blocker reference is required")
		}
		ref.Task.RemoveDependency(blockerRef)
		return nil
	})
}

// ClearDependencies removes every dependency edge from the task. Dependency
// blocking is a derived predicate (HasIncompleteDependencies), not a workflow
// state, so clearing the last dependency simply makes the task no longer blocked
// while preserving its workflow state — the use case never reopens or transitions
// the task. Clearing a task that carries no dependencies is a no-op that still
// commits the (unchanged) project.
func (s *Service) ClearDependencies(taskID string) (TaskResult, error) {
	return s.mutateTask(taskID, "Clear Dependencies", "", func(ref TaskRef) error {
		ref.Task.ClearDependencies()
		return nil
	})
}

// SetDependencies replaces a task's entire dependency set with deps, committing
// one ChangeSet. It is the wholesale analogue of AddDependency/RemoveDependency
// for callers (the TUI dependency selector) that compute the complete post-edit
// set themselves: the selector only offers non-cyclic, non-self candidates, so
// the set is validated at selection time and applied here verbatim. A nil or
// empty slice clears every dependency. Dependency blocking is a derived predicate
// rather than a workflow state, so this never reopens or transitions the task.
func (s *Service) SetDependencies(taskID string, deps []string) (TaskResult, error) {
	return s.mutateTask(taskID, "Edit Dependencies", "", func(ref TaskRef) error {
		next := make([]string, 0, len(deps))
		next = append(next, deps...)
		ref.Task.DependsOn = next
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// AddRelated links the task to relatedRef (a same-project UUID or a
// "projectID:taskID" cross-project reference). Related links are non-directional
// and carry no cycle constraint, so only existence and self-reference are
// validated.
func (s *Service) AddRelated(taskID, relatedRef string) (TaskResult, error) {
	ref, snap, revision, err := s.loadTask(taskID)
	if err != nil {
		return TaskResult{}, err
	}
	relatedRef = strings.TrimSpace(relatedRef)
	if relatedRef == "" {
		return TaskResult{}, invalidInput("related", "related reference is required")
	}
	related, _, ok := NewWorkspaceResolver(snap).resolveRef(relatedRef, ref.Project)
	if !ok {
		return TaskResult{}, fmt.Errorf("%w: related %s", ErrTaskNotFound, relatedRef)
	}
	if related.ID == ref.Task.ID && !models.ParseTaskRef(relatedRef).IsCrossProject() {
		return TaskResult{}, invalidInput("related", "a task cannot be related to itself")
	}
	if err := ref.Task.AddRelatedTo(relatedRef); err != nil {
		return TaskResult{}, invalidInput("related", err.Error())
	}
	return s.commitProject(ref, revision, "Add Related Link", relatedRef)
}

// RelatedLinkResult is the presentation-neutral outcome of a bidirectional
// related-link mutation, carrying every project the transaction touched so an
// adapter can reload exactly those projects.
type RelatedLinkResult struct {
	// TouchedProjectIDs lists the committed projects (one entry when both tasks
	// share a project, two otherwise).
	TouchedProjectIDs []string
	// SourceShortID and TargetShortID identify the linked tasks for display.
	SourceShortID int
	TargetShortID int
}

// AddRelatedBidirectional links two tasks with reciprocal related references
// and commits every touched project as ONE transaction, so a cross-project
// follow-up link never lands on only one side. otherRef is a same-project task
// UUID or a "projectID:taskID" cross-project reference; the stored back-reference
// is qualified with the source project when the tasks live in different projects.
func (s *Service) AddRelatedBidirectional(taskID, otherRef string) (RelatedLinkResult, error) {
	ref, snap, revision, err := s.loadTask(taskID)
	if err != nil {
		return RelatedLinkResult{}, err
	}
	otherRef = strings.TrimSpace(otherRef)
	if otherRef == "" {
		return RelatedLinkResult{}, invalidInput("related", "related reference is required")
	}
	other, otherProject, ok := NewWorkspaceResolver(snap).resolveRef(otherRef, ref.Project)
	if !ok {
		return RelatedLinkResult{}, fmt.Errorf("%w: related %s", ErrTaskNotFound, otherRef)
	}
	if other.ID == ref.Task.ID && otherProject.ID == ref.Project.ID {
		return RelatedLinkResult{}, invalidInput("related", "a task cannot be related to itself")
	}
	backRef := ref.Task.ID
	if otherProject.ID != ref.Project.ID {
		backRef = ref.Project.ID + ":" + ref.Task.ID
	}
	if err := ref.Task.AddRelatedTo(otherRef); err != nil {
		return RelatedLinkResult{}, invalidInput("related", err.Error())
	}
	if err := other.AddRelatedTo(backRef); err != nil {
		return RelatedLinkResult{}, invalidInput("related", err.Error())
	}
	commits := []projectCommit{{project: ref.Project, expectedRevision: revision}}
	touched := []string{ref.Project.ID}
	if otherProject.ID != ref.Project.ID {
		commits = append(commits, projectCommit{project: otherProject, expectedRevision: snap.Revision(otherProject.ID)})
		touched = append(touched, otherProject.ID)
	}
	undoContext := fmt.Sprintf("#%d <-> #%d", ref.Task.ShortID, other.ShortID)
	if _, err := s.commitProjects(commits, "Add Related Link", undoContext); err != nil {
		return RelatedLinkResult{}, err
	}
	return RelatedLinkResult{
		TouchedProjectIDs: touched,
		SourceShortID:     ref.Task.ShortID,
		TargetShortID:     other.ShortID,
	}, nil
}

// RemoveRelated drops a related link. Removing a link the task does not carry is a
// no-op that still commits the (unchanged) project.
func (s *Service) RemoveRelated(taskID, relatedRef string) (TaskResult, error) {
	return s.mutateTask(taskID, "Remove Related Link", relatedRef, func(ref TaskRef) error {
		relatedRef = strings.TrimSpace(relatedRef)
		if relatedRef == "" {
			return invalidInput("related", "related reference is required")
		}
		ref.Task.RemoveRelatedTo(relatedRef)
		return nil
	})
}

// RemoveRelatedBidirectional drops a related link from BOTH sides and commits
// every touched project as ONE transaction, the mirror of AddRelatedBidirectional.
// The forward reference is always removed from the source task; the reciprocal
// back-reference is removed from the target when it still resolves in the
// workspace (a same-project target rides the same commit, a cross-project target
// adds a second atomic commit). A target that no longer resolves — a dangling or
// already-deleted link — degenerates to a source-only removal, so a half-broken
// related pair can always be cleaned up.
func (s *Service) RemoveRelatedBidirectional(taskID, otherRef string) (RelatedLinkResult, error) {
	ref, snap, revision, err := s.loadTask(taskID)
	if err != nil {
		return RelatedLinkResult{}, err
	}
	otherRef = strings.TrimSpace(otherRef)
	if otherRef == "" {
		return RelatedLinkResult{}, invalidInput("related", "related reference is required")
	}
	// Always drop the forward reference from the source.
	ref.Task.RemoveRelatedTo(otherRef)

	commits := []projectCommit{{project: ref.Project, expectedRevision: revision}}
	touched := []string{ref.Project.ID}
	targetShortID := 0

	if other, otherProject, ok := NewWorkspaceResolver(snap).resolveRef(otherRef, ref.Project); ok {
		targetShortID = other.ShortID
		backRef := ref.Task.ID
		if otherProject.ID != ref.Project.ID {
			backRef = ref.Project.ID + ":" + ref.Task.ID
		}
		other.RemoveRelatedTo(backRef)
		if otherProject.ID != ref.Project.ID {
			commits = append(commits, projectCommit{project: otherProject, expectedRevision: snap.Revision(otherProject.ID)})
			touched = append(touched, otherProject.ID)
		}
	}

	undoContext := fmt.Sprintf("#%d", ref.Task.ShortID)
	if targetShortID != 0 {
		undoContext = fmt.Sprintf("#%d <-> #%d", ref.Task.ShortID, targetShortID)
	}
	if _, err := s.commitProjects(commits, "Remove Related Link", undoContext); err != nil {
		return RelatedLinkResult{}, err
	}
	return RelatedLinkResult{
		TouchedProjectIDs: touched,
		SourceShortID:     ref.Task.ShortID,
		TargetShortID:     targetShortID,
	}, nil
}
