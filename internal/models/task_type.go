package models

import "strings"

// TaskType represents the kind of task item.
type TaskType string

const (
	TaskTypeEpic  TaskType = "epic"
	TaskTypeStory TaskType = "story"
	TaskTypeTask  TaskType = "task"
	TaskTypeBug   TaskType = "bug"
	TaskTypeIdea  TaskType = "idea"
	TaskTypeNote  TaskType = "note"
)

// AllTaskTypes returns all valid task types.
func AllTaskTypes() []TaskType {
	return []TaskType{TaskTypeEpic, TaskTypeStory, TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeNote}
}

// IsValid checks if the task type is valid.
func (t TaskType) IsValid() bool {
	switch t {
	case TaskTypeEpic, TaskTypeStory, TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeNote:
		return true
	}
	return false
}

// AllowedChildren returns the task types that can be children of this type.
// Epic can contain stories, tasks, bugs, ideas, and notes.
// Story can contain tasks, bugs, and notes.
// Task, Bug, and Idea can contain notes.
// Note is a leaf node with no children.
func (t TaskType) AllowedChildren() []TaskType {
	switch t {
	case TaskTypeEpic:
		return []TaskType{TaskTypeStory, TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeNote}
	case TaskTypeStory:
		return []TaskType{TaskTypeTask, TaskTypeBug, TaskTypeNote}
	case TaskTypeTask, TaskTypeBug, TaskTypeIdea:
		return []TaskType{TaskTypeNote}
	case TaskTypeNote:
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
	for _, allowed := range parent.AllowedChildren() {
		if t == allowed {
			return true
		}
	}
	return false
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
	case TaskTypeNote:
		return "N"
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
	case TaskTypeNote:
		return "Note"
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

// IsLeafType returns true if this is a leaf type (Task, Bug, Idea, or Note).
// Leaf types cannot have children (except Task, Bug, Idea can have Note children).
func (t TaskType) IsLeafType() bool {
	switch t {
	case TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeNote:
		return true
	}
	return false
}

// IsNoteType returns true if this is a Note type.
func (t TaskType) IsNoteType() bool {
	return t == TaskTypeNote
}

// Promote returns the next higher type in the hierarchy.
// Task/Bug/Idea -> Story, Story -> Epic, Epic -> Epic (already at top).
// Note -> Note (notes don't participate in hierarchy promotion).
func (t TaskType) Promote() TaskType {
	switch t {
	case TaskTypeTask, TaskTypeBug, TaskTypeIdea:
		return TaskTypeStory
	case TaskTypeStory:
		return TaskTypeEpic
	case TaskTypeEpic:
		return TaskTypeEpic // Already at top
	case TaskTypeNote:
		return TaskTypeNote // Notes don't participate in hierarchy
	}
	return t
}

// Demote returns the next lower type in the hierarchy.
// Epic -> Story, Story -> Task, leaf types stay the same.
// Note -> Note (notes don't participate in hierarchy demotion).
func (t TaskType) Demote() TaskType {
	switch t {
	case TaskTypeEpic:
		return TaskTypeStory
	case TaskTypeStory:
		return TaskTypeTask // Default leaf type
	case TaskTypeTask, TaskTypeBug, TaskTypeIdea:
		return t // Already at bottom
	case TaskTypeNote:
		return TaskTypeNote // Notes don't participate in hierarchy
	}
	return t
}

// CycleLeafLeft cycles leaf types to the left: Task -> Idea -> Bug -> Task.
// Non-leaf types and Notes return themselves unchanged.
// Notes are excluded from leaf cycling as they are semantically different.
func (t TaskType) CycleLeafLeft() TaskType {
	switch t {
	case TaskTypeTask:
		return TaskTypeIdea
	case TaskTypeBug:
		return TaskTypeTask
	case TaskTypeIdea:
		return TaskTypeBug
	case TaskTypeNote:
		return TaskTypeNote // Notes don't participate in leaf cycling
	}
	return t
}

// CycleLeafRight cycles leaf types to the right: Task -> Bug -> Idea -> Task.
// Non-leaf types and Notes return themselves unchanged.
// Notes are excluded from leaf cycling as they are semantically different.
func (t TaskType) CycleLeafRight() TaskType {
	switch t {
	case TaskTypeTask:
		return TaskTypeBug
	case TaskTypeBug:
		return TaskTypeIdea
	case TaskTypeIdea:
		return TaskTypeTask
	case TaskTypeNote:
		return TaskTypeNote // Notes don't participate in leaf cycling
	}
	return t
}
