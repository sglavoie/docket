package models

import (
	"slices"
	"strings"
)

// TaskType represents the kind of task item.
type TaskType string

const (
	TaskTypeEpic   TaskType = "epic"
	TaskTypeStory  TaskType = "story"
	TaskTypeTask   TaskType = "task"
	TaskTypeBug    TaskType = "bug"
	TaskTypeIdea   TaskType = "idea"
	TaskTypeAction TaskType = "action"
)

// AllTaskTypes returns all valid task types.
func AllTaskTypes() []TaskType {
	return []TaskType{TaskTypeEpic, TaskTypeStory, TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction}
}

// IsValid checks if the task type is valid.
func (t TaskType) IsValid() bool {
	switch t {
	case TaskTypeEpic, TaskTypeStory, TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction:
		return true
	}
	return false
}

// AllowedChildren returns the task types that can be children of this type.
// Epic can contain stories, tasks, bugs, and ideas.
// Story can contain tasks, bugs, and ideas.
// Task, Bug, and Idea are leaf nodes with no children.
func (t TaskType) AllowedChildren() []TaskType {
	switch t {
	case TaskTypeEpic:
		return []TaskType{TaskTypeStory, TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction}
	case TaskTypeStory:
		return []TaskType{TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction}
	case TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction:
		return []TaskType{}
	default:
		return []TaskType{}
	}
}

// CanHaveChildren returns true if this task type can have child tasks.
func (t TaskType) CanHaveChildren() bool {
	return len(t.AllowedChildren()) > 0
}

// CanBeChildOf returns true if this task type can be a child of the given parent type.
func (t TaskType) CanBeChildOf(parent TaskType) bool {
	return slices.Contains(parent.AllowedChildren(), t)
}

// Icon returns a single-character icon for the task type.
func (t TaskType) Icon() string {
	switch t {
	case TaskTypeEpic:
		return "E"
	case TaskTypeStory:
		return "S"
	case TaskTypeTask:
		return "T"
	case TaskTypeBug:
		return "B"
	case TaskTypeIdea:
		return "I"
	case TaskTypeAction:
		return "A"
	default:
		return "?"
	}
}

// String returns the string representation of the task type.
func (t TaskType) String() string {
	return string(t)
}

// HeadingPrefix returns the markdown heading prefix for this type.
// Used when writing tasks to markdown format.
func (t TaskType) HeadingPrefix() string {
	switch t {
	case TaskTypeEpic:
		return "Epic"
	case TaskTypeStory:
		return "Story"
	case TaskTypeTask:
		return "Task"
	case TaskTypeBug:
		return "Bug"
	case TaskTypeIdea:
		return "Idea"
	case TaskTypeAction:
		return "Action"
	default:
		return "Task"
	}
}

// ParseTaskType converts a string to a TaskType.
func ParseTaskType(s string) (TaskType, bool) {
	t := TaskType(s)
	return t, t.IsValid()
}

// ParseTaskTypeInsensitive converts a string to a TaskType case-insensitively.
func ParseTaskTypeInsensitive(s string) (TaskType, bool) {
	t := TaskType(strings.ToLower(s))
	return t, t.IsValid()
}

// IsLeafType returns true if this is a leaf type (Task, Bug, or Idea).
// Leaf types have no children.
func (t TaskType) IsLeafType() bool {
	switch t {
	case TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction:
		return true
	}
	return false
}

// Promote returns the next higher type in the hierarchy.
// Task/Bug/Idea -> Story, Story -> Epic, Epic -> Epic (already at top).
func (t TaskType) Promote() TaskType {
	switch t {
	case TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction:
		return TaskTypeStory
	case TaskTypeStory:
		return TaskTypeEpic
	case TaskTypeEpic:
		return TaskTypeEpic // Already at top
	}
	return t
}

// Demote returns the next lower type in the hierarchy.
// Epic -> Story, Story -> Task, leaf types stay the same.
func (t TaskType) Demote() TaskType {
	switch t {
	case TaskTypeEpic:
		return TaskTypeStory
	case TaskTypeStory:
		return TaskTypeTask // Default leaf type
	case TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction:
		return t // Already at bottom
	}
	return t
}

// CycleLeafLeft cycles leaf types to the left: Task -> Idea -> Bug -> Task.
// Non-leaf types return themselves unchanged.
func (t TaskType) CycleLeafLeft() TaskType {
	switch t {
	case TaskTypeTask:
		return TaskTypeIdea
	case TaskTypeBug:
		return TaskTypeTask
	case TaskTypeIdea:
		return TaskTypeBug
	}
	return t
}

// CycleLeafRight cycles leaf types to the right: Task -> Bug -> Idea -> Task.
// Non-leaf types return themselves unchanged.
func (t TaskType) CycleLeafRight() TaskType {
	switch t {
	case TaskTypeTask:
		return TaskTypeBug
	case TaskTypeBug:
		return TaskTypeIdea
	case TaskTypeIdea:
		return TaskTypeTask
	}
	return t
}
