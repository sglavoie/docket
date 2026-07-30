package models

// StateReview is the retired v4 review state. It remains exported only while
// legacy readers and unremoved adapters are being migrated. Target operations
// must validate with WorkflowState.IsValid, which deliberately rejects it.
const StateReview WorkflowState = "review"

// LegacyReviewTag preserves the meaning of a retired Review state after it is
// normalized into the target vocabulary.
const LegacyReviewTag = "review"

// AllLegacyWorkflowStates returns the complete v4 vocabulary in its historical
// order. Migration and compatibility code may use this; target code should use
// AllWorkflowStates.
func AllLegacyWorkflowStates() []WorkflowState {
	return []WorkflowState{
		StateInbox,
		StateReady,
		StateNext,
		StateInProgress,
		StateWaiting,
		StatePaused,
		StateReview,
		StateSomeday,
		StateDone,
		StateCanceled,
	}
}

// ParseLegacyWorkflowState recognizes the v4 state vocabulary without making a
// retired state valid for target operations.
func ParseLegacyWorkflowState(value string) (WorkflowState, bool) {
	state := WorkflowState(value)
	if state.IsValid() || state == StateReview {
		return state, true
	}
	return "", false
}

// TargetStateForLegacy converts a recognized legacy state into the target
// vocabulary. Review becomes Ready; callers preserving a task record must also
// add LegacyReviewTag.
func TargetStateForLegacy(state WorkflowState) (WorkflowState, bool) {
	if state.IsValid() {
		return state, true
	}
	if state == StateReview {
		return StateReady, true
	}
	return "", false
}
