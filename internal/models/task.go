package models

import (
	"fmt"
	"time"
)

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusBlocked    TaskStatus = "blocked"
	StatusCancelled  TaskStatus = "cancelled"
)

// SectionBacklog is the default section for new tasks awaiting triage.
const SectionBacklog = "Backlog"

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

// Task represents a single task, story, epic, bug, or idea.
type Task struct {
	ID          string     `json:"id"`
	ShortID     int        `json:"short_id"`
	Type        TaskType   `json:"type"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	Priority    int        `json:"priority"`
	ParentID    *string    `json:"parent_id,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Section     string     `json:"section,omitempty"` // Section for organizing tasks (e.g., "Backlog", "Active")
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	DependsOn   []string   `json:"depends_on,omitempty"` // IDs of tasks this task depends on
	Pinned      bool       `json:"pinned,omitempty"`     // Whether the task is pinned
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`  // When the task was pinned

	// Computed fields (not persisted)
	Children []*Task  `json:"-"`
	Parent   *Task    `json:"-"`
	Project  *Project `json:"-"`
	Level    int      `json:"-"` // Depth in hierarchy (0 for root)
}

// NewTask creates a new task with default values.
func NewTask(id string, shortID int, taskType TaskType, title string) *Task {
	now := time.Now()
	return &Task{
		ID:        id,
		ShortID:   shortID,
		Type:      taskType,
		Title:     title,
		Status:    StatusTodo,
		Priority:  3, // Medium priority
		Tags:      []string{},
		Section:   SectionBacklog,
		DependsOn: []string{},
		CreatedAt: now,
		UpdatedAt: now,
		Children:  []*Task{},
	}
}

// IsLeaf returns true if this task has no children.
func (t *Task) IsLeaf() bool {
	return len(t.Children) == 0
}

// IsRoot returns true if this task has no parent.
func (t *Task) IsRoot() bool {
	return t.ParentID == nil
}

// IsComplete returns true if the task status is complete.
func (t *Task) IsComplete() bool {
	return t.Status.IsComplete()
}

// Complete marks the task as done and sets the completion time.
// If the task is in the Backlog section, it is automatically moved out (auto-triage).
func (t *Task) Complete() {
	now := time.Now()
	t.Status = StatusDone
	t.CompletedAt = &now
	t.UpdatedAt = now
	// Auto-triage: move out of Backlog when completing
	if t.Section == SectionBacklog {
		t.Section = ""
	}
}

// Start marks the task as in progress.
// If the task is in the Backlog section, it is automatically moved out (auto-triage).
func (t *Task) Start() {
	t.Status = StatusInProgress
	t.UpdatedAt = time.Now()
	// Auto-triage: move out of Backlog when starting
	if t.Section == SectionBacklog {
		t.Section = ""
	}
}

// Block marks the task as blocked.
func (t *Task) Block() {
	t.Status = StatusBlocked
	t.UpdatedAt = time.Now()
}

// Cancel marks the task as cancelled.
// If the task is in the Backlog section, it is automatically moved out (auto-triage).
func (t *Task) Cancel() {
	now := time.Now()
	t.Status = StatusCancelled
	t.CompletedAt = &now
	t.UpdatedAt = now
	// Auto-triage: move out of Backlog when cancelling
	if t.Section == SectionBacklog {
		t.Section = ""
	}
}

// Reopen sets the task back to todo status.
func (t *Task) Reopen() {
	t.Status = StatusTodo
	t.CompletedAt = nil
	t.UpdatedAt = time.Now()
}

// sliceContains returns true if the slice contains the value.
func sliceContains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}

// sliceRemove returns a new slice with the value removed (if present).
func sliceRemove(slice []string, value string) ([]string, bool) {
	for i, s := range slice {
		if s == value {
			return append(slice[:i], slice[i+1:]...), true
		}
	}
	return slice, false
}

// AddTag adds a tag if not already present.
func (t *Task) AddTag(tag string) {
	if sliceContains(t.Tags, tag) {
		return
	}
	t.Tags = append(t.Tags, tag)
	t.UpdatedAt = time.Now()
}

// RemoveTag removes a tag if present.
func (t *Task) RemoveTag(tag string) {
	if newTags, removed := sliceRemove(t.Tags, tag); removed {
		t.Tags = newTags
		t.UpdatedAt = time.Now()
	}
}

// HasTag returns true if the task has the given tag.
func (t *Task) HasTag(tag string) bool {
	return sliceContains(t.Tags, tag)
}

// GetProgress returns the completion percentage based on children.
// For leaf tasks, returns 100 if complete, 0 otherwise.
// For parent tasks, returns percentage of completed children.
func (t *Task) GetProgress() float64 {
	if len(t.Children) == 0 {
		if t.IsComplete() {
			return 100.0
		}
		return 0.0
	}

	completed := 0
	for _, child := range t.Children {
		if child.IsComplete() {
			completed++
		}
	}
	return float64(completed) / float64(len(t.Children)) * 100.0
}

// CountChildren returns the total number of descendants.
func (t *Task) CountChildren() int {
	count := len(t.Children)
	for _, child := range t.Children {
		count += child.CountChildren()
	}
	return count
}

// CountCompletedChildren returns the number of completed descendants.
func (t *Task) CountCompletedChildren() int {
	count := 0
	for _, child := range t.Children {
		if child.IsComplete() {
			count++
		}
		count += child.CountCompletedChildren()
	}
	return count
}

// CanAddChild returns true if the given task type can be added as a child.
func (t *Task) CanAddChild(childType TaskType) bool {
	return childType.CanBeChildOf(t.Type)
}

// AddChild adds a task as a child, setting the parent reference.
func (t *Task) AddChild(child *Task) {
	child.ParentID = &t.ID
	child.Parent = t
	child.Level = t.Level + 1
	t.Children = append(t.Children, child)
}

// CanPromote checks if this task can be promoted to the next higher type.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanPromote() (bool, string) {
	newType := t.Type.Promote()

	// Already at top of hierarchy
	if newType == t.Type {
		return false, "already at top of hierarchy"
	}

	// Check if new type is valid under current parent
	if t.Parent != nil {
		if !newType.CanBeChildOf(t.Parent.Type) {
			return false, newType.String() + " cannot be child of " + t.Parent.Type.String()
		}
	}

	// Check if all children are valid under new type
	for _, child := range t.Children {
		if !child.Type.CanBeChildOf(newType) {
			return false, child.Type.String() + " cannot be child of " + newType.String()
		}
	}

	return true, ""
}

// CanDemote checks if this task can be demoted to the next lower type.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanDemote() (bool, string) {
	newType := t.Type.Demote()

	// Already at bottom of hierarchy
	if newType == t.Type {
		return false, "already at bottom of hierarchy"
	}

	// Check if new type can have children (if we have any)
	if len(t.Children) > 0 && !newType.CanHaveChildren() {
		return false, newType.String() + " cannot have children"
	}

	// Check if new type is valid under current parent
	if t.Parent != nil {
		if !newType.CanBeChildOf(t.Parent.Type) {
			return false, newType.String() + " cannot be child of " + t.Parent.Type.String()
		}
	}

	// Check if all children are valid under new type
	for _, child := range t.Children {
		if !child.Type.CanBeChildOf(newType) {
			return false, child.Type.String() + " cannot be child of " + newType.String()
		}
	}

	return true, ""
}

// CanCycleLeafType checks if this task can cycle its leaf type.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanCycleLeafType(left bool) (bool, string) {
	if !t.Type.IsLeafType() {
		return false, "not a leaf type"
	}

	var newType TaskType
	if left {
		newType = t.Type.CycleLeafLeft()
	} else {
		newType = t.Type.CycleLeafRight()
	}

	// Check if new type is valid under current parent
	if t.Parent != nil {
		if !newType.CanBeChildOf(t.Parent.Type) {
			return false, newType.String() + " cannot be child of " + t.Parent.Type.String()
		}
	}

	return true, ""
}

// CanIndent checks if this task can become a child of the given target parent.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanIndent(targetParent *Task) (bool, string) {
	if targetParent == nil {
		return false, "no target parent"
	}

	// Check if target can have children
	if !targetParent.Type.CanHaveChildren() {
		return false, targetParent.Type.String() + " cannot have children"
	}

	// Check if this task's type can be a child of target
	if !t.Type.CanBeChildOf(targetParent.Type) {
		return false, t.Type.String() + " cannot be child of " + targetParent.Type.String()
	}

	// Check if all children remain valid (they stay under this task, so no change)
	// Children keep the same parent type, so they're still valid

	return true, ""
}

// CanOutdent checks if this task can move to be a sibling of its current parent.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanOutdent() (bool, string) {
	// Must have a parent to outdent
	if t.Parent == nil {
		return false, "already at root level"
	}

	// If parent has a grandparent, check if our type is valid under grandparent
	if t.Parent.Parent != nil {
		if !t.Type.CanBeChildOf(t.Parent.Parent.Type) {
			return false, t.Type.String() + " cannot be child of " + t.Parent.Parent.Type.String()
		}
	}

	// Children stay under this task, so they remain valid

	return true, ""
}

// Promote changes this task's type to the next higher type in the hierarchy.
// Returns an error if promotion is not allowed.
func (t *Task) Promote() error {
	if allowed, reason := t.CanPromote(); !allowed {
		return fmt.Errorf("cannot promote: %s", reason)
	}

	t.Type = t.Type.Promote()
	t.UpdatedAt = time.Now()
	return nil
}

// Demote changes this task's type to the next lower type in the hierarchy.
// Returns an error if demotion is not allowed.
func (t *Task) Demote() error {
	if allowed, reason := t.CanDemote(); !allowed {
		return fmt.Errorf("cannot demote: %s", reason)
	}

	t.Type = t.Type.Demote()
	t.UpdatedAt = time.Now()
	return nil
}

// CycleLeafType cycles this task's type among leaf types (Task, Bug, Idea).
// If left is true, cycles left (Task -> Idea -> Bug -> Task).
// If left is false, cycles right (Task -> Bug -> Idea -> Task).
// Returns an error if cycling is not allowed.
func (t *Task) CycleLeafType(left bool) error {
	if allowed, reason := t.CanCycleLeafType(left); !allowed {
		return fmt.Errorf("cannot cycle type: %s", reason)
	}

	if left {
		t.Type = t.Type.CycleLeafLeft()
	} else {
		t.Type = t.Type.CycleLeafRight()
	}
	t.UpdatedAt = time.Now()
	return nil
}

// AddDependency adds a dependency on another task if not already present.
func (t *Task) AddDependency(blockerID string) {
	if sliceContains(t.DependsOn, blockerID) {
		return
	}
	t.DependsOn = append(t.DependsOn, blockerID)
	t.UpdatedAt = time.Now()
}

// RemoveDependency removes a dependency if present.
func (t *Task) RemoveDependency(blockerID string) {
	if newDeps, removed := sliceRemove(t.DependsOn, blockerID); removed {
		t.DependsOn = newDeps
		t.UpdatedAt = time.Now()
	}
}

// HasDependency returns true if the task depends on the given task ID.
func (t *Task) HasDependency(blockerID string) bool {
	return sliceContains(t.DependsOn, blockerID)
}

// ClearDependencies removes all dependencies.
func (t *Task) ClearDependencies() {
	if len(t.DependsOn) > 0 {
		t.DependsOn = []string{}
		t.UpdatedAt = time.Now()
	}
}

// Pin marks the task as pinned.
func (t *Task) Pin() {
	now := time.Now()
	t.Pinned = true
	t.PinnedAt = &now
	t.UpdatedAt = now
}

// Unpin removes the pinned status.
func (t *Task) Unpin() {
	t.Pinned = false
	t.PinnedAt = nil
	t.UpdatedAt = time.Now()
}

// TogglePinned toggles the pinned status.
func (t *Task) TogglePinned() {
	if t.Pinned {
		t.Unpin()
	} else {
		t.Pin()
	}
}
