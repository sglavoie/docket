package models

// WorkflowStateGroup is a presentation bucket derived from canonical task
// state. Today is the one synthetic group: its membership comes from the Daily
// Plan rather than from a WorkflowState. The remaining groups map one-to-one to
// workflow states so list surfaces never need project sections to classify
// work.
type WorkflowStateGroup string

const (
	StateGroupToday      WorkflowStateGroup = "today"
	StateGroupInProgress WorkflowStateGroup = "in_progress"
	StateGroupInbox      WorkflowStateGroup = "inbox"
	StateGroupNext       WorkflowStateGroup = "next"
	StateGroupBacklog    WorkflowStateGroup = "backlog"
	StateGroupWaiting    WorkflowStateGroup = "waiting"
	StateGroupPaused     WorkflowStateGroup = "paused"
	StateGroupSomeday    WorkflowStateGroup = "someday"
	StateGroupDone       WorkflowStateGroup = "done"
	StateGroupCanceled   WorkflowStateGroup = "canceled"
)

// WorkflowStateThemeRole is the stable semantic color role assigned to a
// workflow-state group. Rendering packages resolve these roles through their
// active theme; projects and project-section color registries do not
// participate.
type WorkflowStateThemeRole string

const (
	StateThemePlan     WorkflowStateThemeRole = "plan"
	StateThemeWork     WorkflowStateThemeRole = "work"
	StateThemeInbox    WorkflowStateThemeRole = "inbox"
	StateThemeNext     WorkflowStateThemeRole = "next"
	StateThemeBacklog  WorkflowStateThemeRole = "backlog"
	StateThemeWaiting  WorkflowStateThemeRole = "waiting"
	StateThemePaused   WorkflowStateThemeRole = "paused"
	StateThemeSomeday  WorkflowStateThemeRole = "someday"
	StateThemeComplete WorkflowStateThemeRole = "complete"
	StateThemeCanceled WorkflowStateThemeRole = "canceled"
)

// AllModeWorkflowStateGroups returns the canonical visible group order for All.
// Terminal work is intentionally excluded and appears only through explicit
// filters.
func AllModeWorkflowStateGroups() []WorkflowStateGroup {
	return []WorkflowStateGroup{
		StateGroupToday,
		StateGroupInProgress,
		StateGroupInbox,
		StateGroupNext,
		StateGroupPaused,
		StateGroupBacklog,
		StateGroupWaiting,
		StateGroupSomeday,
	}
}

// AllWorkflowStateGroups returns every valid group, including terminal groups.
func AllWorkflowStateGroups() []WorkflowStateGroup {
	groups := AllModeWorkflowStateGroups()
	return append(groups, StateGroupDone, StateGroupCanceled)
}

// IsValid reports whether g is a canonical workflow-state group.
func (g WorkflowStateGroup) IsValid() bool {
	switch g {
	case StateGroupToday, StateGroupInProgress, StateGroupInbox, StateGroupNext,
		StateGroupBacklog, StateGroupWaiting, StateGroupPaused,
		StateGroupSomeday, StateGroupDone, StateGroupCanceled:
		return true
	case StateGroupReview:
		return false
	}
	return false
}

// IsSynthetic reports whether group membership is supplied by a planning
// artifact rather than by task state.
func (g WorkflowStateGroup) IsSynthetic() bool {
	return g == StateGroupToday
}

// Label returns the user-facing canonical group name.
func (g WorkflowStateGroup) Label() string {
	switch g {
	case StateGroupToday:
		return "Today"
	case StateGroupInProgress:
		return "In Progress"
	case StateGroupInbox:
		return "Inbox"
	case StateGroupNext:
		return "Next"
	case StateGroupBacklog:
		return "Backlog"
	case StateGroupReview:
		return "Review"
	case StateGroupWaiting:
		return "Waiting"
	case StateGroupPaused:
		return "Paused"
	case StateGroupSomeday:
		return "Someday"
	case StateGroupDone:
		return "Done"
	case StateGroupCanceled:
		return "Canceled"
	default:
		return ""
	}
}

// ThemeRole returns the static semantic color role for g.
func (g WorkflowStateGroup) ThemeRole() WorkflowStateThemeRole {
	switch g {
	case StateGroupToday:
		return StateThemePlan
	case StateGroupInProgress:
		return StateThemeWork
	case StateGroupInbox:
		return StateThemeInbox
	case StateGroupNext:
		return StateThemeNext
	case StateGroupBacklog:
		return StateThemeBacklog
	case StateGroupReview:
		return StateThemeReview
	case StateGroupWaiting:
		return StateThemeWaiting
	case StateGroupPaused:
		return StateThemePaused
	case StateGroupSomeday:
		return StateThemeSomeday
	case StateGroupDone:
		return StateThemeComplete
	case StateGroupCanceled:
		return StateThemeCanceled
	default:
		return ""
	}
}

// WorkflowStateGroupForState returns the canonical presentation group for a
// task state. Invalid states have no group.
func WorkflowStateGroupForState(state WorkflowState) WorkflowStateGroup {
	switch state {
	case StateInbox:
		return StateGroupInbox
	case StateReady:
		return StateGroupBacklog
	case StateNext:
		return StateGroupNext
	case StateInProgress:
		return StateGroupInProgress
	case StateWaiting:
		return StateGroupWaiting
	case StatePaused:
		return StateGroupPaused
	case StateSomeday:
		return StateGroupSomeday
	case StateDone:
		return StateGroupDone
	case StateCanceled:
		return StateGroupCanceled
	default:
		return ""
	}
}

// WorkflowState returns the state represented by g. Synthetic Today membership
// has no state and returns false.
func (g WorkflowStateGroup) WorkflowState() (WorkflowState, bool) {
	switch g {
	case StateGroupInbox:
		return StateInbox, true
	case StateGroupBacklog:
		return StateReady, true
	case StateGroupNext:
		return StateNext, true
	case StateGroupInProgress:
		return StateInProgress, true
	case StateGroupWaiting:
		return StateWaiting, true
	case StateGroupPaused:
		return StatePaused, true
	case StateGroupReview:
		// StateGroupReview is retained for legacy presentation callers only.
		return StateReady, true
	case StateGroupSomeday:
		return StateSomeday, true
	case StateGroupDone:
		return StateDone, true
	case StateGroupCanceled:
		return StateCanceled, true
	default:
		return "", false
	}
}
