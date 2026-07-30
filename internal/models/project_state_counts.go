package models

// CountTasksByState returns task counts keyed by exact canonical workflow
// state.
func (p *Project) CountTasksByState() map[WorkflowState]int {
	counts := make(map[WorkflowState]int)
	for _, task := range p.Tasks {
		counts[task.State()]++
	}
	return counts
}
