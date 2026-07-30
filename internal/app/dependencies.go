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

// IsDependencyBlocked is the workspace-aware derived blocked predicate. It is
// deliberately independent of workflow state and standalone project blockers:
// only unresolved task dependencies contribute to this status.
func (r *WorkspaceResolver) IsDependencyBlocked(task *models.Task, owner *models.Project) bool {
	return r.HasIncompleteDependencies(task, owner)
}

// GetBlockingTasks resolves every dependency carried by task across the
// workspace. Unresolvable references are omitted because there is no TaskRef to
// return; HasIncompleteDependencies still treats unresolvable qualified
// references as blocking.
func (r *WorkspaceResolver) GetBlockingTasks(task *models.Task, owner *models.Project) []TaskRef {
	if task == nil || owner == nil {
		return nil
	}
	blockers := make([]TaskRef, 0, len(task.DependsOn))
	seen := make(map[taskReferenceIdentity]bool, len(task.DependsOn))
	for _, dependency := range task.DependsOn {
		blocker, project, ok := r.resolveRef(dependency, owner)
		if !ok {
			continue
		}
		identity := taskReferenceIdentity{projectID: project.ID, taskID: blocker.ID}
		if seen[identity] {
			continue
		}
		seen[identity] = true
		blockers = append(blockers, TaskRef{
			Task:    blocker,
			Project: project,
			Base:    r.snap.Document(project.ID),
		})
	}
	return blockers
}

// GetBlockedByTasks performs the reverse workspace lookup for blocker. Results
// follow snapshot project order and each project's durable task order. Both
// stable-project-ID and legacy-slug spellings resolve to the same identity.
func (r *WorkspaceResolver) GetBlockedByTasks(blocker *models.Task, owner *models.Project) []TaskRef {
	if blocker == nil || owner == nil {
		return nil
	}
	var blocked []TaskRef
	for _, project := range r.snap.Projects() {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || !r.containsTaskReference(task.DependsOn, project, owner, blocker.ID) {
				continue
			}
			blocked = append(blocked, TaskRef{
				Task:    task,
				Project: project,
				Base:    r.snap.Document(project.ID),
			})
		}
	}
	return blocked
}

// WouldCreateDependencyCycle reports whether adding a dependency from taskID (in
// owner) onto blockerRef would create a cycle, following DependsOn edges across
// projects. A cycle forms when the blocker already depends — directly or
// transitively — on the task. A reference that leaves the snapshot simply ends
// that path; it cannot close a cycle through an unknown task.
func (r *WorkspaceResolver) WouldCreateDependencyCycle(owner *models.Project, taskID, blockerRef string) bool {
	if owner == nil || taskID == "" {
		return false
	}
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

// SetDependencies replaces a task's entire dependency set with deps, committing
// one ChangeSet. It is the wholesale analog of AddDependency/RemoveDependency
// for callers (the TUI dependency selector) that compute the complete post-edit
// set themselves. The application boundary validates the complete replacement:
// every reference must resolve, self-dependencies and workspace-spanning cycles
// are rejected, and stable-ID/legacy-slug aliases are identity-deduplicated. A
// nil or empty slice clears every dependency. Dependency blocking is a derived
// predicate rather than a workflow state, so this never reopens or transitions
// the task.
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
	next, err := validateDependencySet(ref, resolver, deps)
	if err != nil {
		return TaskResult{}, err
	}
	ref.Task.DependsOn = next
	ref.Task.UpdatedAt = s.clock.Now()
	return s.commitProject(ref, revision, "Edit Dependencies", "")
}

func validateDependencySet(ref TaskRef, resolver *WorkspaceResolver, deps []string) ([]string, error) {
	if len(deps) == 0 {
		return nil, nil
	}
	next := make([]string, 0, len(deps))
	seen := make(map[taskReferenceIdentity]bool, len(deps))
	for _, raw := range deps {
		dependency := strings.TrimSpace(raw)
		if dependency == "" {
			return nil, invalidInput("dependency", "blocker reference is required")
		}
		blocker, blockerProject, ok := resolver.resolveRef(dependency, ref.Project)
		if !ok {
			return nil, fmt.Errorf("%w: blocker %s", ErrTaskNotFound, dependency)
		}
		if blocker.ID == ref.Task.ID && blockerProject.ID == ref.Project.ID {
			return nil, invalidInput("dependency", "a task cannot depend on itself")
		}
		if resolver.WouldCreateDependencyCycle(ref.Project, ref.Task.ID, dependency) {
			return nil, invalidInput("dependency", "would create a circular dependency")
		}
		identity := taskReferenceIdentity{projectID: blockerProject.ID, taskID: blocker.ID}
		if seen[identity] {
			continue
		}
		seen[identity] = true
		next = append(next, dependency)
	}
	return next, nil
}
