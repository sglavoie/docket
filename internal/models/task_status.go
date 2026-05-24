package models

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/icons"
)

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusReview     TaskStatus = "review"
	StatusStandby    TaskStatus = "standby"
	StatusDone       TaskStatus = "done"
	StatusBlocked    TaskStatus = "blocked"
	StatusCanceled   TaskStatus = "canceled"
	StatusMaybe      TaskStatus = "maybe"
)

// SectionWaiting is the section for tasks blocked on external input (GTD "Waiting For").
const SectionWaiting = "Waiting"

// SectionSomeday is the section for uncommitted ideas (GTD "Someday/Maybe").
const SectionSomeday = "Someday"

// SectionReview is the section for tasks in review (code review, QA, approval).
const SectionReview = "Review"

// SectionStandby is the section for tasks put on hold (paused work).
const SectionStandby = "Standby"

// SectionCompleted is the section for completed/canceled tasks.
const SectionCompleted = "Completed"

// IsGTDSection returns true if the section name is a built-in GTD section.
func IsGTDSection(name string) bool {
	return name == SectionNext || name == SectionInbox ||
		name == SectionWaiting || name == SectionSomeday || name == SectionReview || name == SectionStandby || name == SectionCompleted
}

// IsGTDSectionNameCI reports whether the given name matches a built-in GTD
// section name under case-insensitive comparison. Use this at command/TUI
// boundaries that accept user-supplied names (e.g., `tt section add inbox`)
// to reject case-variant collisions before they corrupt project.Sections.
// IsGTDSection (exact-case) remains the right check for persisted/canonical
// data such as task.Section values read from markdown.
func IsGTDSectionNameCI(name string) bool {
	switch strings.ToLower(name) {
	case strings.ToLower(SectionNext),
		strings.ToLower(SectionInbox),
		strings.ToLower(SectionWaiting),
		strings.ToLower(SectionSomeday),
		strings.ToLower(SectionReview),
		strings.ToLower(SectionStandby),
		strings.ToLower(SectionCompleted):
		return true
	}
	return false
}

// AllTaskStatuses returns all valid task statuses.
func AllTaskStatuses() []TaskStatus {
	return []TaskStatus{StatusTodo, StatusInProgress, StatusReview, StatusStandby, StatusMaybe, StatusDone, StatusBlocked, StatusCanceled}
}

// IsValid checks if the status is valid.
func (s TaskStatus) IsValid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusReview, StatusStandby, StatusMaybe, StatusDone, StatusBlocked, StatusCanceled:
		return true
	}
	return false
}

// IsComplete returns true if this status represents a completed state.
func (s TaskStatus) IsComplete() bool {
	return s == StatusDone || s == StatusCanceled
}

// IsActive returns true if this status represents an active/open state.
func (s TaskStatus) IsActive() bool {
	return s == StatusTodo || s == StatusInProgress || s == StatusReview || s == StatusStandby || s == StatusMaybe || s == StatusBlocked
}

// Icon returns a status indicator character.
func (s TaskStatus) Icon() string {
	switch s {
	case StatusTodo:
		return icons.Get("status.todo")
	case StatusInProgress:
		return icons.Get("status.in_progress")
	case StatusReview:
		return icons.Get("status.review")
	case StatusStandby:
		return icons.Get("status.standby")
	case StatusDone:
		return icons.Get("status.done")
	case StatusBlocked:
		return icons.Get("status.blocked")
	case StatusCanceled:
		return icons.Get("status.canceled")
	case StatusMaybe:
		return icons.Get("status.maybe")
	default:
		return "?"
	}
}

// String returns the string representation of the status.
func (s TaskStatus) String() string {
	return string(s)
}

// ValidateSectionName returns an error if the section name is empty, too long,
// or contains whitespace. Section names must not contain spaces because the
// /SectionName quick-capture syntax uses whitespace as a delimiter.
func ValidateSectionName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("section name cannot be empty")
	}
	if len(name) > MaxSectionNameLength {
		return fmt.Errorf("section name %q exceeds maximum length of %d characters", name, MaxSectionNameLength)
	}
	if strings.ContainsAny(name, " \t\n\r") {
		return fmt.Errorf("section name %q must not contain spaces (use hyphens or camelCase instead)", name)
	}
	if strings.Contains(name, ";") {
		return fmt.Errorf("section name %q must not contain semicolons (used as internal delimiter)", name)
	}
	if strings.Contains(name, ":") {
		return fmt.Errorf("section name %q must not contain colons (used as internal delimiter for section colors)", name)
	}
	return nil
}

// ParseTaskStatus converts a string to a TaskStatus.
func ParseTaskStatus(str string) (TaskStatus, bool) {
	s := TaskStatus(str)
	return s, s.IsValid()
}
