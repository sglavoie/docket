package models

// CountTasks returns the total number of tasks.
func (p *Project) CountTasks() int {
	return len(p.Tasks)
}

// CountActiveSectionTasks returns the number of tasks in the Active section
// (empty section AND empty custom section) or with in_progress status regardless of section.
// Tasks with a CustomSection are not in the Active view even though their
// Section field is empty.
func (p *Project) CountActiveSectionTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.IsComplete() {
			continue
		}
		if task.Status == StatusInProgress {
			count++
		} else if task.Section == "" && task.CustomSection == "" {
			count++
		}
	}
	return count
}

// CountActiveTasks returns the number of non-completed tasks.
func (p *Project) CountActiveTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.Status.IsActive() {
			count++
		}
	}
	return count
}

// CountCompletedTasks returns the number of completed tasks.
func (p *Project) CountCompletedTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.IsComplete() {
			count++
		}
	}
	return count
}

// CountOverdueTasks returns the number of incomplete tasks with past due dates.
func (p *Project) CountOverdueTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.IsOverdue() {
			count++
		}
	}
	return count
}

// CountDueSoonTasks returns the number of tasks due within the specified days
// that are not overdue.
func (p *Project) CountDueSoonTasks(days int) int {
	count := 0
	for _, task := range p.Tasks {
		if task.IsDueSoon(days) {
			count++
		}
	}
	return count
}

// CountBlockedTasks returns the number of tasks with blocked status.
func (p *Project) CountBlockedTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.Status == StatusBlocked {
			count++
		}
	}
	return count
}
