package models

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// Comment represents a timestamped note on a task.
type Comment struct {
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// RecurrenceUnit represents the time unit for recurrence patterns.
type RecurrenceUnit string

const (
	RecurrenceDaily   RecurrenceUnit = "day"
	RecurrenceWeekly  RecurrenceUnit = "week"
	RecurrenceMonthly RecurrenceUnit = "month"
	RecurrenceYearly  RecurrenceUnit = "year"
)

// RecurrencePattern defines how a task recurs.
type RecurrencePattern struct {
	Interval int            `json:"interval"`           // Every N periods (e.g., 2 = every 2 weeks)
	Unit     RecurrenceUnit `json:"unit"`               // day, week, month, year
	Weekdays []int          `json:"weekdays,omitempty"` // For weekly: 0=Sun, 1=Mon, ..., 6=Sat
}

// String returns a human-readable representation of the recurrence pattern.
func (r *RecurrencePattern) String() string {
	if r == nil {
		return ""
	}
	if r.Interval == 1 {
		switch r.Unit {
		case RecurrenceDaily:
			return "daily"
		case RecurrenceWeekly:
			if len(r.Weekdays) > 0 {
				return fmt.Sprintf("weekly on %s", weekdaysToString(r.Weekdays))
			}
			return "weekly"
		case RecurrenceMonthly:
			return "monthly"
		case RecurrenceYearly:
			return "yearly"
		}
	}
	return fmt.Sprintf("every %d %ss", r.Interval, r.Unit)
}

// weekdaysToString converts weekday numbers to abbreviated names.
func weekdaysToString(weekdays []int) string {
	names := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	var result []string
	for _, d := range weekdays {
		if d >= 0 && d < 7 {
			result = append(result, names[d])
		}
	}
	if len(result) == 0 {
		return ""
	}
	if len(result) == 1 {
		return result[0]
	}
	// Join with commas, last one with "and"
	if len(result) == 2 {
		return result[0] + " and " + result[1]
	}
	return fmt.Sprintf("%s, and %s", joinStrings(result[:len(result)-1], ", "), result[len(result)-1])
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusBlocked    TaskStatus = "blocked"
	StatusCancelled  TaskStatus = "cancelled"
)

// SectionNext is the default section for tasks ready to work on (GTD "Next Actions").
const SectionNext = "Next"

// SectionInbox is the section for quick-captured tasks awaiting triage.
const SectionInbox = "Inbox"

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

// Task represents a single task, story, epic, bug, or idea.
type Task struct {
	ID          string             `json:"id"`
	ShortID     int                `json:"short_id"`
	Type        TaskType           `json:"type"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Status      TaskStatus         `json:"status"`
	Priority    int                `json:"priority"`
	ParentID    *string            `json:"parent_id,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Section     string             `json:"section,omitempty"` // Section for organizing tasks (e.g., "Next", "Inbox", "Waiting", "Someday")
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	CompletedAt *time.Time         `json:"completed_at,omitempty"`
	DueDate     *time.Time         `json:"due_date,omitempty"`
	DependsOn   []string           `json:"depends_on,omitempty"` // IDs of tasks this task depends on
	Pinned      bool               `json:"pinned,omitempty"`     // Whether the task is pinned
	PinnedAt    *time.Time         `json:"pinned_at,omitempty"`  // When the task was pinned
	Active      bool               `json:"active,omitempty"`     // Whether the task is flagged as active
	Comments    []*Comment         `json:"comments,omitempty"`   // Timestamped comments/notes
	Recurrence  *RecurrencePattern `json:"recurrence,omitempty"` // Recurrence pattern (nil = non-recurring)

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
		Section:   "", // Empty section = Active view (default for new tasks)
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

// IsNote returns true if this is a Note type task.
func (t *Task) IsNote() bool {
	return t.Type == TaskTypeNote
}

// Complete marks the task as done and sets the completion time.
// If the task is in a non-Active section, it is automatically moved out (auto-triage).
func (t *Task) Complete() {
	now := time.Now()
	t.Status = StatusDone
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.Active = false
	// Auto-triage: move out of Next/Waiting/Someday when completing
	if t.Section == SectionNext || t.Section == SectionWaiting || t.Section == SectionSomeday {
		t.Section = ""
	}
}

// Start marks the task as in progress.
// If the task is in any non-Active section, it is automatically moved out (auto-triage).
func (t *Task) Start() {
	t.Status = StatusInProgress
	t.UpdatedAt = time.Now()
	// Auto-triage: move out of Inbox/Next/Waiting/Someday when starting
	if t.Section == SectionInbox || t.Section == SectionNext || t.Section == SectionWaiting || t.Section == SectionSomeday {
		t.Section = ""
	}
}

// Block marks the task as blocked.
func (t *Task) Block() {
	t.Status = StatusBlocked
	t.UpdatedAt = time.Now()
}

// Cancel marks the task as cancelled.
// If the task is in a non-Active section, it is automatically moved out (auto-triage).
func (t *Task) Cancel() {
	now := time.Now()
	t.Status = StatusCancelled
	t.CompletedAt = &now
	t.UpdatedAt = now
	// Auto-triage: move out of Next/Waiting/Someday when cancelling
	if t.Section == SectionNext || t.Section == SectionWaiting || t.Section == SectionSomeday {
		t.Section = ""
	}
}

// Reopen sets the task back to todo status and moves it to the Inbox for re-processing.
func (t *Task) Reopen() {
	t.Status = StatusTodo
	t.CompletedAt = nil
	t.Section = SectionInbox
	t.UpdatedAt = time.Now()
}

// AddTag adds a tag if not already present.
func (t *Task) AddTag(tag string) {
	if util.SliceContains(t.Tags, tag) {
		return
	}
	t.Tags = append(t.Tags, tag)
	t.UpdatedAt = time.Now()
}

// RemoveTag removes a tag if present.
func (t *Task) RemoveTag(tag string) {
	if newTags, removed := util.SliceRemove(t.Tags, tag); removed {
		t.Tags = newTags
		t.UpdatedAt = time.Now()
	}
}

// HasTag returns true if the task has the given tag.
func (t *Task) HasTag(tag string) bool {
	return util.SliceContains(t.Tags, tag)
}

// GetProgress returns the completion percentage based on children.
// For leaf tasks, returns 100 if complete, 0 otherwise.
// For parent tasks, returns percentage of completed children.
// Notes are excluded from progress calculation as they are not completable items.
func (t *Task) GetProgress() float64 {
	if len(t.Children) == 0 {
		if t.IsComplete() {
			return 100.0
		}
		return 0.0
	}

	total := 0
	completed := 0
	for _, child := range t.Children {
		if child.IsNote() {
			continue
		}
		total++
		if child.IsComplete() {
			completed++
		}
	}
	if total == 0 {
		return 0.0
	}
	return float64(completed) / float64(total) * 100.0
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

// HasProgressChildren returns true if the task has non-note children.
// Used to determine whether to display progress indicators.
func (t *Task) HasProgressChildren() bool {
	for _, child := range t.Children {
		if !child.IsNote() {
			return true
		}
	}
	return false
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
	if util.SliceContains(t.DependsOn, blockerID) {
		return
	}
	t.DependsOn = append(t.DependsOn, blockerID)
	t.UpdatedAt = time.Now()
}

// RemoveDependency removes a dependency if present.
func (t *Task) RemoveDependency(blockerID string) {
	if newDeps, removed := util.SliceRemove(t.DependsOn, blockerID); removed {
		t.DependsOn = newDeps
		t.UpdatedAt = time.Now()
	}
}

// HasDependency returns true if the task depends on the given task ID.
func (t *Task) HasDependency(blockerID string) bool {
	return util.SliceContains(t.DependsOn, blockerID)
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

// Activate marks the task as active.
func (t *Task) Activate() {
	t.Active = true
	t.UpdatedAt = time.Now()
}

// Deactivate removes the active status.
func (t *Task) Deactivate() {
	t.Active = false
	t.UpdatedAt = time.Now()
}

// ToggleActive toggles the active status.
func (t *Task) ToggleActive() {
	if t.Active {
		t.Deactivate()
	} else {
		t.Activate()
	}
}

// AddComment adds a new timestamped comment to the task.
func (t *Task) AddComment(text string) {
	comment := &Comment{
		Text:      text,
		CreatedAt: time.Now(),
	}
	t.Comments = append(t.Comments, comment)
	t.UpdatedAt = time.Now()
}

// RemoveComment removes the comment at the given index.
// Returns true if a comment was removed.
func (t *Task) RemoveComment(index int) bool {
	if index < 0 || index >= len(t.Comments) {
		return false
	}
	t.Comments = append(t.Comments[:index], t.Comments[index+1:]...)
	t.UpdatedAt = time.Now()
	return true
}

// ClearComments removes all comments from the task.
func (t *Task) ClearComments() {
	if len(t.Comments) > 0 {
		t.Comments = []*Comment{}
		t.UpdatedAt = time.Now()
	}
}

// CommentCount returns the number of comments on the task.
func (t *Task) CommentCount() int {
	return len(t.Comments)
}

// IsRecurring returns true if the task has a recurrence pattern.
func (t *Task) IsRecurring() bool {
	return t.Recurrence != nil
}

// SetRecurrence sets the recurrence pattern for the task.
func (t *Task) SetRecurrence(pattern *RecurrencePattern) {
	t.Recurrence = pattern
	t.UpdatedAt = time.Now()
}

// ClearRecurrence removes the recurrence pattern from the task.
func (t *Task) ClearRecurrence() {
	if t.Recurrence != nil {
		t.Recurrence = nil
		t.UpdatedAt = time.Now()
	}
}

// NextDueDate calculates the next due date based on the recurrence pattern.
// Uses fixed interval scheduling: next date is calculated from the current due date.
// If the task has no due date, returns nil.
// If the task is not recurring, returns nil.
func (t *Task) NextDueDate() *time.Time {
	if t.Recurrence == nil || t.DueDate == nil {
		return nil
	}

	next := calculateNextDate(*t.DueDate, t.Recurrence)
	return &next
}

// calculateNextDate computes the next occurrence date from a base date.
func calculateNextDate(base time.Time, r *RecurrencePattern) time.Time {
	switch r.Unit {
	case RecurrenceDaily:
		return base.AddDate(0, 0, r.Interval)

	case RecurrenceWeekly:
		if len(r.Weekdays) == 0 {
			// Simple weekly: add N weeks
			return base.AddDate(0, 0, r.Interval*7)
		}
		// Weekly with specific days: find next matching weekday
		return findNextWeekday(base, r.Weekdays, r.Interval)

	case RecurrenceMonthly:
		return addMonths(base, r.Interval)

	case RecurrenceYearly:
		return base.AddDate(r.Interval, 0, 0)

	default:
		// Fallback to daily
		return base.AddDate(0, 0, r.Interval)
	}
}

// findNextWeekday finds the next occurrence on one of the specified weekdays.
// If interval > 1, it advances by that many weeks first.
func findNextWeekday(base time.Time, weekdays []int, interval int) time.Time {
	if len(weekdays) == 0 {
		return base.AddDate(0, 0, interval*7)
	}

	// Sort weekdays for predictable iteration
	sortedDays := make([]int, len(weekdays))
	copy(sortedDays, weekdays)
	sortInts(sortedDays)

	currentDay := int(base.Weekday())

	// Find the next weekday after today in the same week
	for _, wd := range sortedDays {
		if wd > currentDay {
			daysToAdd := wd - currentDay
			if interval > 1 {
				daysToAdd += (interval - 1) * 7
			}
			return base.AddDate(0, 0, daysToAdd)
		}
	}

	// No more days this week; go to first day of next interval
	daysToFirstDay := (7 - currentDay) + sortedDays[0]
	if interval > 1 {
		daysToFirstDay += (interval - 1) * 7
	}
	return base.AddDate(0, 0, daysToFirstDay)
}

// sortInts sorts a slice of ints in ascending order (simple insertion sort).
func sortInts(nums []int) {
	for i := 1; i < len(nums); i++ {
		for j := i; j > 0 && nums[j] < nums[j-1]; j-- {
			nums[j], nums[j-1] = nums[j-1], nums[j]
		}
	}
}

// addMonths adds N months to a date, handling end-of-month edge cases.
// If the target month has fewer days, clamps to the last day of that month.
func addMonths(base time.Time, months int) time.Time {
	year, month, day := base.Date()
	hour, min, sec := base.Clock()

	targetMonth := int(month) + months
	targetYear := year + (targetMonth-1)/12
	targetMonth = ((targetMonth - 1) % 12) + 1
	if targetMonth <= 0 {
		targetMonth += 12
		targetYear--
	}

	// Find the last day of the target month
	lastDay := daysInMonth(targetYear, time.Month(targetMonth))
	if day > lastDay {
		day = lastDay
	}

	return time.Date(targetYear, time.Month(targetMonth), day, hour, min, sec, base.Nanosecond(), base.Location())
}

// daysInMonth returns the number of days in the given month.
func daysInMonth(year int, month time.Month) int {
	// Go to the first day of the next month, then subtract one day
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// ParseRecurrence parses a recurrence string into a RecurrencePattern.
// Supported formats:
//   - "daily", "weekly", "monthly", "yearly"
//   - "2d" (every 2 days), "3w" (every 3 weeks), "2m" (every 2 months), "1y" (every year)
//   - "weekly:mon,wed,fri" (weekly on specific days)
//
// Returns nil if the string cannot be parsed.
func ParseRecurrence(s string) *RecurrencePattern {
	if s == "" {
		return nil
	}

	// Handle simple keywords
	switch s {
	case "daily":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}
	case "weekly":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly}
	case "biweekly":
		return &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly}
	case "monthly":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	case "yearly":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}
	}

	// Handle Nd, Nw, Nm, Ny format
	if len(s) >= 2 {
		// Check for weekly:days format first
		if len(s) > 7 && s[:7] == "weekly:" {
			weekdays := parseWeekdays(s[7:])
			if len(weekdays) > 0 {
				return &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: weekdays}
			}
		}

		// Check for Nw:days format (e.g., "2w:mon,fri")
		colonIdx := -1
		for i, c := range s {
			if c == ':' {
				colonIdx = i
				break
			}
		}
		if colonIdx > 1 {
			basePattern := parseIntervalUnit(s[:colonIdx])
			if basePattern != nil && basePattern.Unit == RecurrenceWeekly {
				weekdays := parseWeekdays(s[colonIdx+1:])
				if len(weekdays) > 0 {
					basePattern.Weekdays = weekdays
					return basePattern
				}
			}
		}

		// Simple interval format (2d, 3w, etc.)
		return parseIntervalUnit(s)
	}

	return nil
}

// parseIntervalUnit parses formats like "2d", "3w", "1m", "2y".
func parseIntervalUnit(s string) *RecurrencePattern {
	if len(s) < 2 {
		return nil
	}

	// Parse the number part
	numStr := s[:len(s)-1]
	unitChar := s[len(s)-1]

	interval := 0
	for _, c := range numStr {
		if c < '0' || c > '9' {
			return nil
		}
		interval = interval*10 + int(c-'0')
	}
	if interval <= 0 {
		return nil
	}

	var unit RecurrenceUnit
	switch unitChar {
	case 'd':
		unit = RecurrenceDaily
	case 'w':
		unit = RecurrenceWeekly
	case 'm':
		unit = RecurrenceMonthly
	case 'y':
		unit = RecurrenceYearly
	default:
		return nil
	}

	return &RecurrencePattern{Interval: interval, Unit: unit}
}

// parseWeekdays parses a comma-separated list of weekday abbreviations.
// Returns slice of weekday numbers (0=Sun, 1=Mon, ..., 6=Sat).
func parseWeekdays(s string) []int {
	dayMap := map[string]int{
		"sun": 0, "sunday": 0,
		"mon": 1, "monday": 1,
		"tue": 2, "tuesday": 2,
		"wed": 3, "wednesday": 3,
		"thu": 4, "thursday": 4,
		"fri": 5, "friday": 5,
		"sat": 6, "saturday": 6,
	}

	var result []int
	seen := make(map[int]bool)

	// Split by comma
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			// Trim spaces and convert to lowercase
			part = trimSpace(toLower(part))
			if day, ok := dayMap[part]; ok {
				if !seen[day] {
					result = append(result, day)
					seen[day] = true
				}
			}
			start = i + 1
		}
	}

	return result
}

// toLower converts a string to lowercase.
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// trimSpace removes leading and trailing whitespace.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
