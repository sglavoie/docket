package models

// StateReview is the retired v4 review state. It remains exported only for
// migration readers. Live code must use the eight canonical lifecycle states.
const StateReview WorkflowState = "review"

// LegacyReviewTag preserves the meaning of a retired Review state after it is
// normalized into the target vocabulary.
const LegacyReviewTag = "review"

// TargetStateForLegacy converts a recognized legacy state into the target
// vocabulary. Review becomes Backlog; callers preserving a task record must also
// add LegacyReviewTag.
func TargetStateForLegacy(state WorkflowState) (WorkflowState, bool) {
	if state.IsValid() {
		return state, true
	}
	if state == StateReview {
		return StateBacklog, true
	}
	return "", false
}
