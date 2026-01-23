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

// GetTaskByID finds a task by its UUID.
func (p *Project) GetTaskByID(id string) *Task {
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
func (p *Project) GetNextShortID() int {
	id := p.NextShortID
	p.NextShortID++
	return id
}

// GetCompletedTasks returns all tasks that are complete (done or cancelled).
func (p *Project) GetCompletedTasks() []*Task {
	return p.FilterTasks(func(t *Task) bool {
		return t.IsComplete()
	})
}

// GetTasksByStatus returns tasks filtered by status.
func (p *Project) GetTasksByStatus(status TaskStatus) []*Task {
	return p.FilterTasks(func(t *Task) bool {
		return t.Status == status
	})
}

// GetTasksByType returns tasks filtered by type.
func (p *Project) GetTasksByType(taskType TaskType) []*Task {
	return p.FilterTasks(func(t *Task) bool {
		return t.Type == taskType
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
	var result []*Task
	for _, task := range p.Tasks {
		if predicate(task) {
			result = append(result, task)
		}
	}
	return result
}
