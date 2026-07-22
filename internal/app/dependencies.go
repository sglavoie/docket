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

type taskReferenceIdentity struct {
	projectID string
	taskID    string
}

// NewWorkspaceResolver builds a resolver over a snapshot.
func NewWorkspaceResolver(snap Snapshot) *WorkspaceResolver {
	return &WorkspaceResolver{snap: snap}
}

// projectForQualifier resolves both the stable project ID used by current
// producers and the legacy slug qualifier retained in older Markdown.
func (r *WorkspaceResolver) projectForQualifier(qualifier string) *models.Project {
	if project := r.snap.ProjectByID(qualifier); project != nil {
		return project
	}
	for _, project := range r.snap.Projects() {
		if project != nil && project.Slug == qualifier {
			return project
		}
	}
	return nil
}

// referenceIdentity resolves a stored reference to its owning project and task
// identity without requiring the target task to still exist. This lets removal
// clean a dangling legacy-slug reference when the caller supplies the stable-ID
// spelling (and vice versa).
func (r *WorkspaceResolver) referenceIdentity(ref string, owner *models.Project) (taskReferenceIdentity, bool) {
	parsed := models.ParseTaskRef(ref)
	if parsed.IsCrossProject() {
		project := r.projectForQualifier(parsed.ProjectID)
		if project == nil {
			return taskReferenceIdentity{}, false
		}
		return taskReferenceIdentity{projectID: project.ID, taskID: parsed.TaskID}, true
	}
	if owner == nil || parsed.TaskID == "" {
		return taskReferenceIdentity{}, false
	}
	return taskReferenceIdentity{projectID: owner.ID, taskID: parsed.TaskID}, true
}

func (r *WorkspaceResolver) referencesTask(ref string, owner, targetProject *models.Project, targetTaskID string) bool {
	identity, ok := r.referenceIdentity(ref, owner)
	return ok && targetProject != nil && identity.projectID == targetProject.ID && identity.taskID == targetTaskID
}

func (r *WorkspaceResolver) containsTaskReference(refs []string, owner, targetProject *models.Project, targetTaskID string) bool {
	for _, ref := range refs {
		if r.referencesTask(ref, owner, targetProject, targetTaskID) {
			return true
		}
	}
	return false
}

// deduplicateTaskReferences preserves the first persisted spelling for each
// resolved task identity while collapsing stable-ID/legacy-slug aliases. Exact
// duplicates of unresolved references are also collapsed without guessing at
// their identity.
func (r *WorkspaceResolver) deduplicateTaskReferences(refs []string, owner *models.Project) ([]string, bool) {
	if len(refs) == 0 {
		return refs, false
	}
	seenIdentities := make(map[taskReferenceIdentity]bool, len(refs))
	seenRaw := make(map[string]bool, len(refs))
	out := make([]string, 0, len(refs))
	changed := false
	for _, ref := range refs {
		if identity, ok := r.referenceIdentity(ref, owner); ok {
			if seenIdentities[identity] {
				changed = true
				continue
			}
			seenIdentities[identity] = true
		} else {
			if seenRaw[ref] {
				changed = true
				continue
			}
			seenRaw[ref] = true
		}
		out = append(out, ref)
	}
	return out, changed
}

func (r *WorkspaceResolver) removeTaskReferences(refs []string, owner, targetProject *models.Project, targetTaskID string) ([]string, int) {
	if len(refs) == 0 || targetProject == nil || targetTaskID == "" {
		return refs, 0
	}
	out := make([]string, 0, len(refs))
	removed := 0
	for _, ref := range refs {
		if r.referencesTask(ref, owner, targetProject, targetTaskID) {
			removed++
			continue
		}
		out = append(out, ref)
	}
	return out, removed
}

func (r *WorkspaceResolver) removeEquivalentTaskReferences(refs []string, owner *models.Project, requested string) ([]string, int) {
	if identity, ok := r.referenceIdentity(requested, owner); ok {
		target := r.snap.ProjectByID(identity.projectID)
		return r.removeTaskReferences(refs, owner, target, identity.taskID)
	}
	if len(refs) == 0 {
		return refs, 0
	}
	out := make([]string, 0, len(refs))
	removed := 0
	for _, ref := range refs {
		if ref == requested {
			removed++
			continue
		}
		out = append(out, ref)
	}
	return out, removed
}

// resolveRef resolves a dependency/related reference — a bare task UUID in owner,
// or a "projectID:taskID" cross-project reference — to the target task and the
// project that owns it. It returns ok=false when the reference cannot be resolved
// anywhere in the snapshot.
func (r *WorkspaceResolver) resolveRef(ref string, owner *models.Project) (*models.Task, *models.Project, bool) {
	identity, ok := r.referenceIdentity(ref, owner)
	if !ok {
		return nil, nil, false
	}
	project := r.snap.ProjectByID(identity.projectID)
	if project == nil {
		return nil, nil, false
	}
	task := project.GetTaskByID(identity.taskID)
	return task, project, task != nil
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
	return s.addDependencySelection(taskID, TaskLocator{}, blockerRef)
}

func (s *Service) AddDependencyByLocator(locator TaskLocator, blockerRef string) (TaskResult, error) {
	return s.addDependencySelection("", locator, blockerRef)
}

func (s *Service) addDependencySelection(taskID string, locator TaskLocator, blockerRef string) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return TaskResult{}, err
	}
	resolver := NewWorkspaceResolver(snap)
	blockerRef, err = s.addDependency(ref, resolver, blockerRef)
	if err != nil {
		return TaskResult{}, err
	}
	return s.commitProject(ref, revision, "Add Dependency", blockerRef)
}

func (s *Service) addDependency(ref TaskRef, resolver *WorkspaceResolver, blockerRef string) (string, error) {
	blockerRef = strings.TrimSpace(blockerRef)
	if blockerRef == "" {
		return "", invalidInput("dependency", "blocker reference is required")
	}
	blocker, blockerProject, ok := resolver.resolveRef(blockerRef, ref.Project)
	if !ok {
		return "", fmt.Errorf("%w: blocker %s", ErrTaskNotFound, blockerRef)
	}
	if blocker.ID == ref.Task.ID && blockerProject.ID == ref.Project.ID {
		return "", invalidInput("dependency", "a task cannot depend on itself")
	}
	if resolver.WouldCreateDependencyCycle(ref.Project, ref.Task.ID, blockerRef) {
		return "", invalidInput("dependency", "would create a circular dependency")
	}
	var deduplicated bool
	ref.Task.DependsOn, deduplicated = resolver.deduplicateTaskReferences(ref.Task.DependsOn, ref.Project)
	if deduplicated {
		ref.Task.UpdatedAt = s.clock.Now()
	}
	if !resolver.containsTaskReference(ref.Task.DependsOn, ref.Project, blockerProject, blocker.ID) {
		if err := ref.Task.AddDependency(blockerRef); err != nil {
			return "", invalidInput("dependency", err.Error())
		}
	}
	return blockerRef, nil
}

// AddDependencyWithWorkflow validates and adds a dependency through the same
// workspace-aware resolver as AddDependency, then runs the workflow cascade.
func (s *Service) AddDependencyWithWorkflow(taskID, blockerRef string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.addDependencyWithWorkflowSelection(taskID, TaskLocator{}, blockerRef, opts)
}

func (s *Service) AddDependencyWithWorkflowByLocator(locator TaskLocator, blockerRef string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.addDependencyWithWorkflowSelection("", locator, blockerRef, opts)
}

func (s *Service) addDependencyWithWorkflowSelection(taskID string, locator TaskLocator, blockerRef string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Add Dependency")
	if opts.UndoContext == "" {
		opts.UndoContext = strings.TrimSpace(blockerRef)
	}
	return s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, resolver *WorkspaceResolver) error {
		_, err := s.addDependency(ref, resolver, blockerRef)
		return err
	})
}

// RemoveDependency drops a dependency edge. Removing an edge the task does not
// carry is a no-op that still commits the (unchanged) project.
func (s *Service) RemoveDependency(taskID, blockerRef string) (TaskResult, error) {
	return s.removeDependencySelection(taskID, TaskLocator{}, blockerRef)
}

func (s *Service) RemoveDependencyByLocator(locator TaskLocator, blockerRef string) (TaskResult, error) {
	return s.removeDependencySelection("", locator, blockerRef)
}

func (s *Service) removeDependencySelection(taskID string, locator TaskLocator, blockerRef string) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return TaskResult{}, err
	}
	blockerRef, err = s.removeDependency(ref, NewWorkspaceResolver(snap), blockerRef)
	if err != nil {
		return TaskResult{}, err
	}
	return s.commitProject(ref, revision, "Remove Dependency", blockerRef)
}

func (s *Service) removeDependency(ref TaskRef, resolver *WorkspaceResolver, blockerRef string) (string, error) {
	blockerRef = strings.TrimSpace(blockerRef)
	if blockerRef == "" {
		return "", invalidInput("dependency", "blocker reference is required")
	}
	var removed int
	ref.Task.DependsOn, removed = resolver.removeEquivalentTaskReferences(ref.Task.DependsOn, ref.Project, blockerRef)
	if removed > 0 {
		ref.Task.UpdatedAt = s.clock.Now()
	}
	return blockerRef, nil
}

// RemoveDependencyWithWorkflow removes every reference equivalent to blockerRef
// and runs the workflow cascade on the resulting fresh-snapshot task.
func (s *Service) RemoveDependencyWithWorkflow(taskID, blockerRef string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.removeDependencyWithWorkflowSelection(taskID, TaskLocator{}, blockerRef, opts)
}

func (s *Service) RemoveDependencyWithWorkflowByLocator(locator TaskLocator, blockerRef string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.removeDependencyWithWorkflowSelection("", locator, blockerRef, opts)
}

func (s *Service) removeDependencyWithWorkflowSelection(taskID string, locator TaskLocator, blockerRef string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Remove Dependency")
	if opts.UndoContext == "" {
		opts.UndoContext = strings.TrimSpace(blockerRef)
	}
	return s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, resolver *WorkspaceResolver) error {
		_, err := s.removeDependency(ref, resolver, blockerRef)
		return err
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
		return clearTaskDependencies(ref.Task)
	})
}

func (s *Service) ClearDependenciesByLocator(locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Clear Dependencies", "", func(ref TaskRef) error {
		return clearTaskDependencies(ref.Task)
	})
}

func clearTaskDependencies(task *models.Task) error {
	task.ClearDependencies()
	return nil
}

// ClearDependenciesWithWorkflow clears dependency edges while preserving the
// task's workflow state. An already-empty set is a legacy CLI no-op.
func (s *Service) ClearDependenciesWithWorkflow(taskID string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Clear Dependencies")
	return s.mutateWithWorkflowGuarded(taskID, opts, func(ref TaskRef) bool {
		return len(ref.Task.DependsOn) == 0
	}, func(ref TaskRef, _ *WorkspaceResolver) error {
		return clearTaskDependencies(ref.Task)
	})
}

func (s *Service) ClearDependenciesWithWorkflowByLocator(locator TaskLocator, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Clear Dependencies")
	return s.mutateWithWorkflowGuardedByLocator(locator, opts, func(ref TaskRef) bool {
		return len(ref.Task.DependsOn) == 0
	}, func(ref TaskRef, _ *WorkspaceResolver) error {
		return clearTaskDependencies(ref.Task)
	})
}

// SetDependencies replaces a task's entire dependency set with deps, committing
// one ChangeSet. It is the wholesale analog of AddDependency/RemoveDependency
// for callers (the TUI dependency selector) that compute the complete post-edit
// set themselves: the selector only offers non-cyclic, non-self candidates, so
// the set is validated at selection time and applied here after identity-based
// deduplication. A nil or empty slice clears every dependency. Dependency
// blocking is a derived predicate rather than a workflow state, so this never
// reopens or transitions the task.
func (s *Service) SetDependencies(taskID string, deps []string) (TaskResult, error) {
	return s.setDependencies(taskID, TaskLocator{}, deps)
}

func (s *Service) SetDependenciesByLocator(locator TaskLocator, deps []string) (TaskResult, error) {
	return s.setDependencies("", locator, deps)
}

func (s *Service) setDependencies(taskID string, locator TaskLocator, deps []string) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return TaskResult{}, err
	}
	resolver := NewWorkspaceResolver(snap)
	next := append([]string(nil), deps...)
	ref.Task.DependsOn, _ = resolver.deduplicateTaskReferences(next, ref.Project)
	ref.Task.UpdatedAt = s.clock.Now()
	return s.commitProject(ref, revision, "Edit Dependencies", "")
}

// AddRelated links the task to relatedRef (a same-project UUID or a
// "projectID:taskID" cross-project reference). Related links are non-directional
// and carry no cycle constraint, so only existence and self-reference are
// validated.
func (s *Service) AddRelated(taskID, relatedRef string) (TaskResult, error) {
	return s.addRelated(taskID, TaskLocator{}, relatedRef)
}

func (s *Service) AddRelatedByLocator(locator TaskLocator, relatedRef string) (TaskResult, error) {
	return s.addRelated("", locator, relatedRef)
}

func (s *Service) addRelated(taskID string, locator TaskLocator, relatedRef string) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return TaskResult{}, err
	}
	relatedRef = strings.TrimSpace(relatedRef)
	if relatedRef == "" {
		return TaskResult{}, invalidInput("related", "related reference is required")
	}
	resolver := NewWorkspaceResolver(snap)
	related, relatedProject, ok := resolver.resolveRef(relatedRef, ref.Project)
	if !ok {
		return TaskResult{}, fmt.Errorf("%w: related %s", ErrTaskNotFound, relatedRef)
	}
	if related.ID == ref.Task.ID && relatedProject.ID == ref.Project.ID {
		return TaskResult{}, invalidInput("related", "a task cannot be related to itself")
	}
	var deduplicated bool
	ref.Task.RelatedTo, deduplicated = resolver.deduplicateTaskReferences(ref.Task.RelatedTo, ref.Project)
	if deduplicated {
		ref.Task.UpdatedAt = s.clock.Now()
	}
	if !resolver.containsTaskReference(ref.Task.RelatedTo, ref.Project, relatedProject, related.ID) {
		if err := ref.Task.AddRelatedTo(relatedRef); err != nil {
			return TaskResult{}, invalidInput("related", err.Error())
		}
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
	return s.addRelatedBidirectional(taskID, TaskLocator{}, otherRef)
}

func (s *Service) AddRelatedBidirectionalByLocator(locator TaskLocator, otherRef string) (RelatedLinkResult, error) {
	return s.addRelatedBidirectional("", locator, otherRef)
}

func (s *Service) addRelatedBidirectional(taskID string, locator TaskLocator, otherRef string) (RelatedLinkResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(taskID, locator)
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
	resolver := NewWorkspaceResolver(snap)
	var deduplicated bool
	ref.Task.RelatedTo, deduplicated = resolver.deduplicateTaskReferences(ref.Task.RelatedTo, ref.Project)
	if deduplicated {
		ref.Task.UpdatedAt = s.clock.Now()
	}
	if !resolver.containsTaskReference(ref.Task.RelatedTo, ref.Project, otherProject, other.ID) {
		if err := ref.Task.AddRelatedTo(otherRef); err != nil {
			return RelatedLinkResult{}, invalidInput("related", err.Error())
		}
	}
	other.RelatedTo, deduplicated = resolver.deduplicateTaskReferences(other.RelatedTo, otherProject)
	if deduplicated {
		other.UpdatedAt = s.clock.Now()
	}
	if !resolver.containsTaskReference(other.RelatedTo, otherProject, ref.Project, ref.Task.ID) {
		if err := other.AddRelatedTo(backRef); err != nil {
			return RelatedLinkResult{}, invalidInput("related", err.Error())
		}
	}
	commits := []projectCommit{{project: ref.Project, expectedRevision: revision, base: ref.Base}}
	touched := []string{ref.Project.ID}
	if otherProject.ID != ref.Project.ID {
		commits = append(commits, projectCommit{project: otherProject, expectedRevision: snap.Revision(otherProject.ID), base: snap.Document(otherProject.ID)})
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
	return s.removeRelated(taskID, TaskLocator{}, relatedRef)
}

func (s *Service) RemoveRelatedByLocator(locator TaskLocator, relatedRef string) (TaskResult, error) {
	return s.removeRelated("", locator, relatedRef)
}

func (s *Service) removeRelated(taskID string, locator TaskLocator, relatedRef string) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return TaskResult{}, err
	}
	relatedRef = strings.TrimSpace(relatedRef)
	if relatedRef == "" {
		return TaskResult{}, invalidInput("related", "related reference is required")
	}
	var removed int
	ref.Task.RelatedTo, removed = NewWorkspaceResolver(snap).removeEquivalentTaskReferences(ref.Task.RelatedTo, ref.Project, relatedRef)
	if removed > 0 {
		ref.Task.UpdatedAt = s.clock.Now()
	}
	return s.commitProject(ref, revision, "Remove Related Link", relatedRef)
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
	return s.removeRelatedBidirectional(taskID, TaskLocator{}, otherRef)
}

func (s *Service) RemoveRelatedBidirectionalByLocator(locator TaskLocator, otherRef string) (RelatedLinkResult, error) {
	return s.removeRelatedBidirectional("", locator, otherRef)
}

func (s *Service) removeRelatedBidirectional(taskID string, locator TaskLocator, otherRef string) (RelatedLinkResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return RelatedLinkResult{}, err
	}
	otherRef = strings.TrimSpace(otherRef)
	if otherRef == "" {
		return RelatedLinkResult{}, invalidInput("related", "related reference is required")
	}
	resolver := NewWorkspaceResolver(snap)
	// Always drop every equivalent forward spelling from the source.
	var removed int
	ref.Task.RelatedTo, removed = resolver.removeEquivalentTaskReferences(ref.Task.RelatedTo, ref.Project, otherRef)
	if removed > 0 {
		ref.Task.UpdatedAt = s.clock.Now()
	}

	commits := []projectCommit{{project: ref.Project, expectedRevision: revision, base: ref.Base}}
	touched := []string{ref.Project.ID}
	targetShortID := 0

	if other, otherProject, ok := resolver.resolveRef(otherRef, ref.Project); ok {
		targetShortID = other.ShortID
		other.RelatedTo, removed = resolver.removeTaskReferences(other.RelatedTo, otherProject, ref.Project, ref.Task.ID)
		if removed > 0 {
			other.UpdatedAt = s.clock.Now()
		}
		if otherProject.ID != ref.Project.ID {
			commits = append(commits, projectCommit{project: otherProject, expectedRevision: snap.Revision(otherProject.ID), base: snap.Document(otherProject.ID)})
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
