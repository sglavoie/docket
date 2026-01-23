package models

import "time"

// GetBlockingTasks returns the tasks that the given task depends on.
func (p *Project) GetBlockingTasks(task *Task) []*Task {
	var blockers []*Task
	for _, depID := range task.DependsOn {
		if blocker := p.GetTaskByID(depID); blocker != nil {
			blockers = append(blockers, blocker)
		}
	}
	return blockers
}

// GetBlockedByTasks returns the tasks that depend on the given task.
func (p *Project) GetBlockedByTasks(task *Task) []*Task {
	var blocked []*Task
	for _, t := range p.Tasks {
		if t.HasDependency(task.ID) {
			blocked = append(blocked, t)
		}
	}
	return blocked
}

// HasCircularDependency checks if adding a dependency from taskID to blockerID
// would create a circular dependency.
func (p *Project) HasCircularDependency(taskID, blockerID string) bool {
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

// CheckAndAutoUnblock checks all blocked tasks and transitions them to todo
// if all their dependencies are complete.
func (p *Project) CheckAndAutoUnblock() {
	for _, task := range p.Tasks {
		if task.Status != StatusBlocked || len(task.DependsOn) == 0 {
			continue
		}

		allComplete := true
		for _, depID := range task.DependsOn {
			dep := p.GetTaskByID(depID)
			if dep == nil || !dep.IsComplete() {
				allComplete = false
				break
			}
		}

		if allComplete {
			task.Status = StatusTodo
			task.UpdatedAt = time.Now()
		}
	}
}

// HasIncompleteDependencies returns true if the task has any dependencies
// that are not yet complete (done or cancelled).
func (p *Project) HasIncompleteDependencies(task *Task) bool {
	if len(task.DependsOn) == 0 {
		return false
	}
	for _, depID := range task.DependsOn {
		dep := p.GetTaskByID(depID)
		if dep != nil && !dep.IsComplete() {
			return true
		}
	}
	return false
}

// GetIncompleteDependencies returns the list of dependencies that are not
// yet complete. Useful for error messages when blocking task completion.
func (p *Project) GetIncompleteDependencies(task *Task) []*Task {
	var incomplete []*Task
	for _, depID := range task.DependsOn {
		dep := p.GetTaskByID(depID)
		if dep != nil && !dep.IsComplete() {
			incomplete = append(incomplete, dep)
		}
	}
	return incomplete
}
