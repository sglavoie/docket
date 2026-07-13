package models

import "testing"

// TestApplyWorkflowStateRoundTrip verifies that ApplyWorkflowState stores the
// canonical state such that Task.State() reports that same state for every
// canonical state. This is the contract schema-v2 load relies on when
// reconstituting the workflow state from the stored `state` field.
func TestApplyWorkflowStateRoundTrip(t *testing.T) {
	for _, state := range AllWorkflowStates() {
		task := &Task{}
		task.ApplyWorkflowState(state)
		if got := task.State(); got != state {
			t.Errorf("ApplyWorkflowState(%q) -> State() = %q, want %q",
				state, got, state)
		}
	}
}

// TestApplyWorkflowStateOverridesPriorState guards that applying a state fully
// replaces a previously-set state, so a stale state cannot linger and override
// the intended state on reload.
func TestApplyWorkflowStateOverridesPriorState(t *testing.T) {
	task := &Task{WorkflowState: StateReview}
	task.ApplyWorkflowState(StateNext)
	if got := task.State(); got != StateNext {
		t.Fatalf("State() = %q, want %q", got, StateNext)
	}
}
