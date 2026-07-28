package config

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// Retired planning payloads described lifecycle with a smart-list `status`
// criterion or a saved-view `filter_sections` list. Both vocabularies and the
// presets that carried them are gone, but retired-mode machine routes still
// decode through these snapshots, so the mapping onto canonical states lives on
// here.

// legacySnapshotStates converts a legacy status plus section snapshot into the
// canonical predicate it meant. A contradictory pair intentionally yields an
// empty active set so it stays an explicit match-nothing filter.
func legacySnapshotStates(status string, sections []string) (models.WorkflowStateSet, bool) {
	statusActive := status != ""
	sectionsActive := len(sections) > 0

	statusStates := legacyLifecycleStates("status", status)
	var sectionStates models.WorkflowStateSet
	for _, section := range sections {
		sectionStates = sectionStates.Union(legacyLifecycleStates("section", section))
	}

	switch {
	case statusActive && sectionsActive:
		return statusStates.Intersection(sectionStates), true
	case statusActive:
		return statusStates, true
	case sectionsActive:
		return sectionStates, true
	default:
		return models.WorkflowStateSet{}, false
	}
}

func legacyLifecycleStates(field, value string) models.WorkflowStateSet {
	var matches []models.WorkflowState
	for _, state := range models.AllWorkflowStates() {
		actual := ""
		switch field {
		case "status":
			actual = legacyLifecycleStatus(state)
		case "section":
			actual = legacyLifecycleSection(state)
		}
		if strings.EqualFold(actual, value) {
			matches = append(matches, state)
		}
	}
	states, err := models.NewWorkflowStateSet(matches...)
	if err != nil {
		panic(err)
	}
	return states
}

func legacyLifecycleStatus(state models.WorkflowState) string {
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
		return ""
	}
}

func legacyLifecycleSection(state models.WorkflowState) string {
	switch state {
	case models.StateInbox:
		return "Inbox"
	case models.StateNext:
		return "Next"
	case models.StateWaiting:
		return "Waiting"
	case models.StatePaused:
		return "Standby"
	case models.StateReview:
		return "Review"
	case models.StateSomeday:
		return "Someday"
	case models.StateDone, models.StateCanceled:
		return "Completed"
	default:
		return ""
	}
}
