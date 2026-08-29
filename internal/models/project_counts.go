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
