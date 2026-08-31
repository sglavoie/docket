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

// retiredDestinationMessages names every destination that is recognized and
// wrong. Membership is also what lets search tokenize a retired name so it can
// show this guidance instead of searching for the word as prose. Each message
// takes the rejected name so the retired token never appears as a literal.
var retiredDestinationMessages = map[string]string{
	"ready":       "destination $%s was removed (use $backlog)",
	"in_progress": "destination $%s was removed (use $doing)",
	"todo":        "destination $%s was removed (use $inbox or $backlog)",
	"active":      "destination $%s was removed (use $backlog)",
	"start":       "destination $%s was removed (use $doing)",
	"started":     "destination $%s was removed (use $doing)",
	"progress":    "destination $%s was removed (use $doing)",
	"blocked":     "destination $%s was removed (use $waiting)",
	"standby":     "destination $%s was removed (use $paused)",
	"hold":        "destination $%s was removed (use $paused)",
	"review":      "destination $%s is not a state (use =review)",
	"maybe":       "destination $%s was removed (use $someday)",
	"complete":    "destination $%s was removed (use $done)",
	"completed":   "destination $%s was removed (use $done)",
	"cancel":      "destination $%s was removed (use $canceled)",
	"cancelled":   "destination $%s was removed (use $canceled)", //nolint:misspell // Intentional rejected-input alias.
}

// IsRetiredDestination reports whether a destination name is recognized and
// wrong rather than unknown.
func IsRetiredDestination(raw string) bool {
	_, retired := retiredDestinationMessages[strings.ToLower(strings.TrimSpace(raw))]
	return retired
}

// InvalidDestinationError returns shared actionable guidance for an invalid
// lifecycle/planning destination.
func InvalidDestinationError(raw string) error {
	value := strings.ToLower(strings.TrimSpace(raw))
	if message, retired := retiredDestinationMessages[value]; retired {
		return fmt.Errorf(message, value) //nolint:govet // Every message takes exactly the rejected name.
	}
	return fmt.Errorf(
		"invalid destination: $%s (use %s)",
		raw,
		strings.Join(prefixed(destinationNames, "$"), ", "),
	)
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
