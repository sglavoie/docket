package textgrammar

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// Destination is the canonical lifecycle/planning destination shared by all
// inline-text surfaces and application capture requests.
type Destination string

const (
	DestinationInbox    Destination = "inbox"
	DestinationBacklog  Destination = "backlog"
	DestinationToday    Destination = "today"
	DestinationNext     Destination = "next"
	DestinationDoing    Destination = "doing"
	DestinationWaiting  Destination = "waiting"
	DestinationPaused   Destination = "paused"
	DestinationSomeday  Destination = "someday"
	DestinationDone     Destination = "done"
	DestinationCanceled Destination = "canceled"
)

var destinationNames = []string{
	string(DestinationInbox),
	string(DestinationBacklog),
	string(DestinationToday),
	string(DestinationNext),
	string(DestinationDoing),
	string(DestinationWaiting),
	string(DestinationPaused),
	string(DestinationSomeday),
	string(DestinationDone),
	string(DestinationCanceled),
}

// ParseDestination resolves one exact case-insensitive destination.
func ParseDestination(raw string) (Destination, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if !slices.Contains(destinationNames, value) {
		return "", false
	}
	return Destination(value), true
}

// InvalidDestinationError returns shared actionable guidance for an invalid
// lifecycle/planning destination.
func InvalidDestinationError(raw string) error {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "ready":
		return fmt.Errorf("destination $%s was removed (use $backlog)", value)
	case "in_progress":
		return fmt.Errorf("destination $%s was removed (use $doing)", value)
	case "todo":
		return fmt.Errorf("destination $todo was removed (use $inbox or $backlog)")
	case "active":
		return fmt.Errorf("destination $active was removed (use $backlog)")
	case "start", "started", "progress":
		return fmt.Errorf("destination $%s was removed (use $doing)", value)
	case "blocked":
		return fmt.Errorf("destination $blocked was removed (use $waiting)")
	case "standby", "hold":
		return fmt.Errorf("destination $%s was removed (use $paused)", value)
	case "review":
		return fmt.Errorf("destination $%s is not a state (use #review)", value)
	case "maybe":
		return fmt.Errorf("destination $maybe was removed (use $someday)")
	case "complete", "completed":
		return fmt.Errorf("destination $%s was removed (use $done)", value)
	case "cancel", "cancelled": //nolint:misspell // Intentional rejected-input alias.
		return fmt.Errorf("destination $%s was removed (use $canceled)", value)
	default:
		return fmt.Errorf(
			"invalid destination: $%s (use %s)",
			raw,
			strings.Join(prefixed(destinationNames, "$"), ", "),
		)
	}
}

// WorkflowState returns the lifecycle state represented by the destination.
// Today and Next create Backlog tasks plus a planning membership.
func (d Destination) WorkflowState() models.WorkflowState {
	switch d {
	case DestinationToday, DestinationNext:
		return models.StateBacklog
	default:
		state, _ := models.ParseWorkflowState(string(d))
		return state
	}
}

// IsPlanning reports whether the destination is a planning membership.
func (d Destination) IsPlanning() bool {
	return d == DestinationToday || d == DestinationNext
}
