package tui

import "github.com/sglavoie/tt/internal/models"

// stateMatchesLegacyStatus maps the legacy status filter vocabulary onto the
// canonical workflow states.
func stateMatchesLegacyStatus(state models.WorkflowState, status models.TaskStatus) bool {
	if status == models.StatusTodo {
		return state == models.StateInbox || state == models.StateReady || state == models.StateNext
	}
	return state == models.TargetStateForStatus(status)
}

// taskInPlanningState reports whether the task sits in one of the three
// todo-ish planning states.
func taskInPlanningState(t *models.Task) bool {
	switch t.State() {
	case models.StateInbox, models.StateReady, models.StateNext:
		return true
	case models.StateInProgress, models.StateWaiting, models.StatePaused,
		models.StateReview, models.StateSomeday, models.StateDone, models.StateCanceled:
		return false
	}
	return false
}

// taskIsActionableState reports whether a task is immediately workable.
func taskIsActionableState(t *models.Task, p *models.Project) bool {
	if t.IsDependencyBlocked(p) {
		return false
	}
	state := t.State()
	return state == models.StateReady || state == models.StateInProgress
}

// taskSectionBucket returns the GTD section bucket for a task's workflow state.
func taskSectionBucket(t *models.Task) string {
	return models.SectionForState(t.State())
}

// taskStateGroup returns the canonical presentation group derived from task
// state. Unlike taskSectionBucket it does not collapse ready/in-progress or
// terminal states into legacy section buckets.
func taskStateGroup(t *models.Task) models.WorkflowStateGroup {
	if t == nil {
		return ""
	}
	return models.WorkflowStateGroupForState(t.State())
}

// legacyAllPresentationGroup is the temporary visible-surface adapter used
// before the atomic Outliner cutover. The current Active and Completed headers
// each combine two canonical groups; all other groups remain one-to-one.
func legacyAllPresentationGroup(group models.WorkflowStateGroup) models.WorkflowStateGroup {
	switch group {
	case models.StateGroupInProgress:
		return models.StateGroupBacklog
	case models.StateGroupCanceled:
		return models.StateGroupDone
	default:
		return group
	}
}

// legacySectionForStateGroup converts an authoritative state group into the
// section-shaped label key expected by the current Outliner and section tools.
// It is presentation-only compatibility and must not be used for lifecycle
// decisions.
func legacySectionForStateGroup(group models.WorkflowStateGroup) string {
	switch legacyAllPresentationGroup(group) {
	case models.StateGroupInbox:
		return models.SectionInbox
	case models.StateGroupNext:
		return models.SectionNext
	case models.StateGroupWaiting:
		return models.SectionWaiting
	case models.StateGroupPaused:
		return models.SectionStandby
	case models.StateGroupReview:
		return models.SectionReview
	case models.StateGroupSomeday:
		return models.SectionSomeday
	case models.StateGroupDone:
		return models.SectionCompleted
	default:
		return ""
	}
}

func legacyAllPresentationGroupOrder() []models.WorkflowStateGroup {
	return []models.WorkflowStateGroup{
		models.StateGroupInbox,
		models.StateGroupBacklog,
		models.StateGroupNext,
		models.StateGroupWaiting,
		models.StateGroupReview,
		models.StateGroupPaused,
		models.StateGroupSomeday,
	}
}

// stateForSectionName maps a GTD section name to its workflow state.
func stateForSectionName(section string) (models.WorkflowState, bool) {
	if section != "" && !models.IsGTDSection(section) {
		return "", false
	}
	return models.StateForSection(section), true
}

// workflowStatesForSectionNames translates the temporary section-filter
// presentation into the canonical state-set predicate it represents. Section
// selections use OR semantics; Completed therefore expands to both terminal
// states.
func workflowStatesForSectionNames(sections []string) models.WorkflowStateSet {
	selected := make(map[string]bool, len(sections))
	for _, section := range sections {
		selected[section] = true
	}
	var values []models.WorkflowState
	for _, state := range models.AllWorkflowStates() {
		if selected[models.SectionForState(state)] {
			values = append(values, state)
		}
	}
	states, err := models.NewWorkflowStateSet(values...)
	if err != nil {
		panic(err)
	}
	return states
}

// legacyStatusColumnForState retains the historical status vocabulary used by
// saved smart-filter criteria and palette matching.
func legacyStatusColumnForState(state models.WorkflowState) string {
	switch state {
	case models.StateInbox, models.StateReady, models.StateNext:
		return "todo"
	case models.StateInProgress:
		return "in_progress"
	case models.StateWaiting:
		return "blocked"
	case models.StatePaused:
		return "standby"
	case models.StateReview:
		return "review"
	case models.StateSomeday:
		return "maybe"
	case models.StateDone:
		return "done"
	case models.StateCanceled:
		return "canceled"
	default:
		return string(state)
	}
}
