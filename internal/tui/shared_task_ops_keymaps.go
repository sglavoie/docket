package tui

import "github.com/sglavoie/tt/internal/models"

// sectionForMoveKey maps a move prefix second key to a GTD section name.
// Returns the section name and true if valid, or empty string and false if not.
func sectionForMoveKey(key string) (string, bool) {
	switch key {
	case "a":
		return "", true // Active = empty section
	case "n":
		return models.SectionNext, true
	case "i":
		return models.SectionInbox, true
	case "w":
		return models.SectionWaiting, true
	case "s":
		return models.SectionSomeday, true
	case "r":
		return models.SectionReview, true
	case "S":
		return models.SectionStandby, true
	}
	return "", false
}

// StateForPrefixKey maps a "," status-prefix second-key to a canonical
// WorkflowState. It is the WorkflowState-native successor to the retired
// StatusForPrefixKey: the same physical keys now select states directly
// (b→waiting, o→inbox/reopen, S→paused, m→someday) rather than legacy statuses
// routed through TargetStateForStatus. Returns the state and true if the key is
// valid, or zero value and false otherwise.
func StateForPrefixKey(key string) (models.WorkflowState, bool) {
	switch key {
	case "s":
		return models.StateInProgress, true
	case "d":
		return models.StateDone, true
	case "b":
		return models.StateWaiting, true
	case "o":
		return models.StateInbox, true
	case "x":
		return models.StateCanceled, true
	case "r":
		return models.StateReview, true
	case "S":
		return models.StatePaused, true
	case "m":
		return models.StateSomeday, true
	}
	return "", false
}

// StateVerb returns the past-tense verb for a workflow-state transition, used to
// label undo entries and status toasts.
func StateVerb(state models.WorkflowState) string {
	switch state {
	case models.StateInProgress:
		return "started"
	case models.StateDone:
		return "completed"
	case models.StateWaiting:
		return "set to waiting"
	case models.StateInbox:
		return "reopened"
	case models.StateCanceled:
		return "canceled"
	case models.StateReview:
		return "sent to review"
	case models.StatePaused:
		return "paused"
	case models.StateSomeday:
		return "set to someday"
	case models.StateReady:
		return "set to ready"
	case models.StateNext:
		return "queued as next"
	}
	return string(state)
}

// DueDateDaysForPrefixKey maps a due-date prefix second-key to a day offset.
// Returns (days, isClear, matched). When isClear is true the caller should
// remove the due date instead of setting one.
func DueDateDaysForPrefixKey(key string) (int, bool, bool) {
	switch key {
	case "t":
		return 0, false, true // today
	case "m":
		return 1, false, true // tomorrow
	case "w":
		return 7, false, true // next week
	case "n":
		return 30, false, true // next month
	case "0":
		return 0, true, true // clear
	}
	return 0, false, false
}

// PriorityForPrefixKey maps a priority prefix second-key to a priority value.
// Returns the priority (1-5) and true if valid.
func PriorityForPrefixKey(key string) (int, bool) {
	if len(key) == 1 && key[0] >= '1' && key[0] <= '5' {
		return int(key[0] - '0'), true
	}
	return 0, false
}

// SecondaryViewMovePrefixHints contains "m" prefix hints for secondary views.
var SecondaryViewMovePrefixHints = []PrefixHint{
	{Key: "a", Description: "→Active", Detail: "Send task to Active section"},
	{Key: "n", Description: "→Next", Detail: "Send task to Next section"},
	{Key: "i", Description: "→Inbox", Detail: "Send task to Inbox section"},
	{Key: "w", Description: "→Waiting", Detail: "Send task to Waiting section"},
	{Key: "s", Description: "→Someday", Detail: "Send task to Someday section"},
	{Key: "r", Description: "→Review", Detail: "Send task to Review section"},
	{Key: "S", Description: "→Standby", Detail: "Send task to Standby section"},
}
