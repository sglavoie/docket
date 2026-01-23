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
	var result []*Task
	for _, task := range p.Tasks {
		if task.IsComplete() {
			result = append(result, task)
		}
	}
	return result
}

// GetTasksByStatus returns tasks filtered by status.
func (p *Project) GetTasksByStatus(status TaskStatus) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		if task.Status == status {
			result = append(result, task)
		}
	}
	return result
}

// GetTasksByType returns tasks filtered by type.
func (p *Project) GetTasksByType(taskType TaskType) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		if task.Type == taskType {
			result = append(result, task)
		}
	}
	return result
}

// GetTasksByTag returns tasks that have the given tag.
func (p *Project) GetTasksByTag(tag string) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		if task.HasTag(tag) {
			result = append(result, task)
		}
	}
	return result
}
