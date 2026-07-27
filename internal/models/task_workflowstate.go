package models

// State returns the task's authoritative WorkflowState. The stored
// WorkflowState field is the single source of truth; a task that carries no
// stored state (only possible for a zero-value in-memory task, since the loader
// always derives one) resolves to StateReady.
func (t *Task) State() WorkflowState {
	if t.WorkflowState != "" {
		return t.WorkflowState
	}
	return StateReady
}

// IsInState reports whether the task's state equals the given state.
func (t *Task) IsInState(state WorkflowState) bool {
	return t.State() == state
}

// ApplyWorkflowState stores the canonical state. CompletedAt is intentionally
// left untouched: terminal timestamps are owned by ApplyStateTransitionAt.
func (t *Task) ApplyWorkflowState(state WorkflowState) {
	t.WorkflowState = state
}

// IsTerminalState reports whether the task is in a terminal state.
func (t *Task) IsTerminalState() bool {
	return t.State().IsTerminal()
}

// IsDependencyBlocked reports whether the task is gated by an unresolved
// dependency within the given project. Dependency blocking is derived, not a
// workflow state: it is surfaced as a badge/filter and combines with the task's
// state rather than replacing it. A nil project yields false.
func (t *Task) IsDependencyBlocked(p *Project) bool {
	if p == nil {
		return false
	}
	return p.HasIncompleteDependencies(t)
}
