package models

// GetBlockingTasks returns the tasks that the given task depends on.
// A nil task returns nil so a caller passing an unset argument cannot
// trigger a nil-pointer dereference on the DependsOn slice.
func (p *Project) GetBlockingTasks(task *Task) []*Task {
	if task == nil {
		return nil
	}
	var blockers []*Task
	for _, depID := range task.DependsOn {
		if blocker := p.GetTaskByID(depID); blocker != nil {
			blockers = append(blockers, blocker)
		}
	}
	return blockers
}

// GetBlockedByTasks returns the tasks that depend on the given task.
// A nil task returns nil so a caller passing an unset argument cannot
// trigger a nil-pointer dereference when reading task.ID inside the loop.
func (p *Project) GetBlockedByTasks(task *Task) []*Task {
	if task == nil {
		return nil
	}
	var blocked []*Task
	for _, t := range p.Tasks {
		if t.HasDependency(task.ID) {
			blocked = append(blocked, t)
		}
	}
	return blocked
}

// HasCircularDependency checks if adding a dependency from taskID to blockerID
// would create a circular dependency. Empty IDs cannot form a real cycle and
// always return false; without this guard, dependsOnTransitive("", "") returns
// true because sourceID == targetID, falsely reporting a cycle for an unset
// caller argument.
func (p *Project) HasCircularDependency(taskID, blockerID string) bool {
	if taskID == "" || blockerID == "" {
		return false
	}
	// If blocker depends (directly or transitively) on task, adding this dependency
	// would create a cycle.
	visited := make(map[string]bool)
	return p.dependsOnTransitive(blockerID, taskID, visited)
}

// dependsOnTransitive checks if source task depends on target task transitively.
func (p *Project) dependsOnTransitive(sourceID, targetID string, visited map[string]bool) bool {
	if sourceID == targetID {
		return true
	}
	if visited[sourceID] {
		return false
	}
	visited[sourceID] = true

	source := p.GetTaskByID(sourceID)
	if source == nil {
		return false
	}

	for _, depID := range source.DependsOn {
		if p.dependsOnTransitive(depID, targetID, visited) {
			return true
		}
	}
	return false
}

// HasIncompleteDependencies returns true if the task has any dependencies
// that are not yet complete (done or canceled).
// Cross-project qualified references (format "projectID:taskID") that cannot
// be resolved locally are conservatively treated as incomplete.
// Dangling local references (depID points to a task that no longer exists)
// are silently skipped: BuildTaskTree prunes orphaned refs on load, so any
// survivors are tolerated as resolved rather than blocking completion.
// A nil task returns false so an unset argument cannot panic on task.DependsOn.
func (p *Project) HasIncompleteDependencies(task *Task) bool {
	if task == nil {
		return false
	}
	if len(task.DependsOn) == 0 {
		return false
	}
	for _, depID := range task.DependsOn {
		if ParseTaskRef(depID).IsCrossProject() {
			// Cross-project dep cannot be resolved locally; treat as incomplete
			// to prevent premature completion.
			return true
		}
		dep := p.GetTaskByID(depID)
		if dep != nil && !dep.IsComplete() {
			return true
		}
	}
	return false
}

// GetIncompleteDependencies returns the list of dependencies that are not
// yet complete. Useful for error messages when blocking task completion.
// A nil task returns nil so an unset argument cannot panic on task.DependsOn.
// Dangling local references (depID points to a task that no longer exists)
// are silently skipped, matching the HasIncompleteDependencies contract.
// Cross-project references are also skipped here because they cannot be
// resolved to a *Task locally; callers needing to flag cross-project
// blockers should consult HasIncompleteDependencies (which does count
// them) and report the count separately.
func (p *Project) GetIncompleteDependencies(task *Task) []*Task {
	if task == nil {
		return nil
	}
	var incomplete []*Task
	for _, depID := range task.DependsOn {
		dep := p.GetTaskByID(depID)
		if dep != nil && !dep.IsComplete() {
			incomplete = append(incomplete, dep)
		}
	}
	return incomplete
}
