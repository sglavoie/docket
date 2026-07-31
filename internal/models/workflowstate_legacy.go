package models

// StateReview is the retired v4 review state. It remains exported only while
// legacy readers and unremoved adapters are being migrated. Target operations
// must validate with WorkflowState.IsValid, which deliberately rejects it.
const StateReview WorkflowState = "review"

// LegacyReviewTag preserves the meaning of a retired Review state after it is
// normalized into the target vocabulary.
const LegacyReviewTag = "review"

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
