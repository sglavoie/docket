package models

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusBlocked    TaskStatus = "blocked"
	StatusCancelled  TaskStatus = "cancelled"
)

// SectionWaiting is the section for tasks blocked on external input (GTD "Waiting For").
const SectionWaiting = "Waiting"

// SectionSomeday is the section for uncommitted ideas (GTD "Someday/Maybe").
const SectionSomeday = "Someday"

// IsGTDSection returns true if the section name is a built-in GTD section.
func IsGTDSection(name string) bool {
	return name == SectionNext || name == SectionInbox ||
		name == SectionWaiting || name == SectionSomeday
}

// AllTaskStatuses returns all valid task statuses.
func AllTaskStatuses() []TaskStatus {
	return []TaskStatus{StatusTodo, StatusInProgress, StatusDone, StatusBlocked, StatusCancelled}
}

// IsValid checks if the status is valid.
func (s TaskStatus) IsValid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone, StatusBlocked, StatusCancelled:
		return true
	}
	return false
}

// IsComplete returns true if this status represents a completed state.
func (s TaskStatus) IsComplete() bool {
	return s == StatusDone || s == StatusCancelled
}

// IsActive returns true if this status represents an active/open state.
func (s TaskStatus) IsActive() bool {
	return s == StatusTodo || s == StatusInProgress || s == StatusBlocked
}

// Icon returns a status indicator character.
func (s TaskStatus) Icon() string {
	switch s {
	case StatusTodo:
		return "○"
	case StatusInProgress:
		return "◐"
	case StatusDone:
		return "●"
	case StatusBlocked:
		return "◈"
	case StatusCancelled:
		return "✕"
	default:
		return "?"
	}
}

// String returns the string representation of the status.
func (s TaskStatus) String() string {
	return string(s)
}

// ParseTaskStatus converts a string to a TaskStatus.
func ParseTaskStatus(str string) (TaskStatus, bool) {
	s := TaskStatus(str)
	return s, s.IsValid()
}
