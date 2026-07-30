package models

// GetRootTasks returns tasks that have no parent.
func (p *Project) GetRootTasks() []*Task {
	var roots []*Task
	for _, task := range p.Tasks {
		if task.IsRoot() {
			roots = append(roots, task)
		}
	}
	return roots
}

// GetTaskByID finds a task by its UUID. An empty id always returns nil so
// that a caller passing an unset identifier cannot accidentally match a task
// whose own ID became empty after a malformed parse (mirrors the guard in
// FindTaskGloballyByID).
func (p *Project) GetTaskByID(id string) *Task {
	if id == "" {
		return nil
	}
	for _, task := range p.Tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}

// GetTaskByShortID finds a task by its short numeric ID.
func (p *Project) GetTaskByShortID(shortID int) *Task {
	for _, task := range p.Tasks {
		if task.ShortID == shortID {
			return task
		}
	}
	return nil
}

// GetNextShortID returns and increments the next available short ID.
// Not goroutine-safe: tt is single-goroutine for project mutations,
// so no synchronization is needed here.
func (p *Project) GetNextShortID() int {
	id := p.NextShortID
	p.NextShortID++
	return id
}

// GetCompletedTasks returns all tasks that are complete (done or canceled).
func (p *Project) GetCompletedTasks() []*Task {
	return p.FilterTasks(func(t *Task) bool {
		return t.IsComplete()
	})
}

// GetTasksByState returns tasks in the exact canonical workflow state while
// preserving project task order. Project sections and display buckets do not
// participate.
func (p *Project) GetTasksByState(state WorkflowState) []*Task {
	return p.FilterTasks(func(t *Task) bool {
		return t.IsInState(state)
	})
}

// GetTasksByTag returns tasks that have the given tag.
func (p *Project) GetTasksByTag(tag string) []*Task {
	return p.FilterTasks(func(t *Task) bool {
		return t.HasTag(tag)
	})
}

// FilterTasks returns tasks that match the given predicate function.
// This is a generic filtering method that can be used directly or by other filtering methods.
func (p *Project) FilterTasks(predicate func(*Task) bool) []*Task {
	if predicate == nil {
		return nil
	}
	var result []*Task
	for _, task := range p.Tasks {
		if predicate(task) {
			result = append(result, task)
		}
	}
	return result
}
