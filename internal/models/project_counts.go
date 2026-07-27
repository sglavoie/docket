package models

// CountTasks returns the total number of tasks.
func (p *Project) CountTasks() int {
	return len(p.Tasks)
}

// CountActiveTasks returns the number of tasks in a nonterminal workflow state.
func (p *Project) CountActiveTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if !task.IsTerminalState() {
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

// CountBlockedTasks returns the number of tasks in the waiting workflow state.
func (p *Project) CountBlockedTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.IsInState(StateWaiting) {
			count++
		}
	}
	return count
}
