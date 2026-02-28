package tui

import "github.com/sglavoie/tt/internal/models"

// viewModeForKey maps a `g` prefix second key to a ProjectTree ViewMode.
// Returns the view mode and true if valid, or zero value and false if not.
func viewModeForKey(key string) (ViewMode, bool) {
	switch key {
	case "a":
		return ViewModeActive, true
	case "n":
		return ViewModeNext, true
	case "i":
		return ViewModeInbox, true
	case "w":
		return ViewModeWaiting, true
	case "s":
		return ViewModeSomeday, true
	case "z":
		return ViewModeSnoozed, true
	case "*":
		return ViewModeAll, true
	case "r":
		return ViewModeReview, true
	case "S":
		return ViewModeStandby, true
	case "c":
		return ViewModeCompleted, true
	}
	return 0, false
}

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

// StatusForPrefixKey maps a prefix second-key to a TaskStatus.
// Returns the status and true if the key is valid, or zero value and false otherwise.
func StatusForPrefixKey(key string) (models.TaskStatus, bool) {
	switch key {
	case "s":
		return models.StatusInProgress, true
	case "d":
		return models.StatusDone, true
	case "b":
		return models.StatusBlocked, true
	case "o":
		return models.StatusTodo, true
	case "x":
		return models.StatusCanceled, true
	case "r":
		return models.StatusReview, true
	case "S":
		return models.StatusStandby, true
	}
	return "", false
}

// StatusVerb returns the past-tense verb for a status transition.
func StatusVerb(status models.TaskStatus) string {
	switch status {
	case models.StatusInProgress:
		return "started"
	case models.StatusDone:
		return "completed"
	case models.StatusBlocked:
		return "blocked"
	case models.StatusTodo:
		return "reopened"
	case models.StatusCanceled:
		return "canceled"
	case models.StatusReview:
		return "sent to review"
	case models.StatusStandby:
		return "put on standby"
	}
	return string(status)
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
