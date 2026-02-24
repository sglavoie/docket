package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// SectionInfo implements PickerItem for section selection.
// FilterText returns the section name for filtering.
func (s SectionInfo) FilterText() string {
	if s.Name == "" {
		return "(no section)"
	}
	return s.Name
}

// DisplayText returns the section name for display, colored if a color is set.
func (s SectionInfo) DisplayText() string {
	name := s.Name
	if name == "" {
		name = "(no section)"
	}
	if s.ColorName != "" && s.ColorName != "gray" {
		colorVal := GetSectionColorValue(s.ColorName)
		if colorVal != "" {
			return lipgloss.NewStyle().Foreground(colorVal).Render(name)
		}
	}
	return name
}

// CountValue returns the task count for this section.
func (s SectionInfo) CountValue() int {
	return s.TaskCount
}

// ItemID returns the section name as its unique identifier.
func (s SectionInfo) ItemID() string {
	return s.Name
}

// ContextInfo implements PickerItem for context selection.
// FilterText returns the context name for filtering.
func (c ContextInfo) FilterText() string {
	if c.Name == "" {
		return "all contexts"
	}
	return c.Name
}

// DisplayText returns the context name for display.
func (c ContextInfo) DisplayText() string {
	if c.Name == "" {
		return "All Contexts"
	}
	return c.Name
}

// CountValue returns the project count for this context.
func (c ContextInfo) CountValue() int {
	return c.ProjectCount
}

// ItemID returns the context name as its unique identifier.
// Custom contexts are prefixed with "+" to distinguish from project contexts.
func (c ContextInfo) ItemID() string {
	if c.IsCustom {
		return "+" + c.Name
	}
	return c.Name
}

// TagInfo implements PickerItem for tag selection.
// FilterText returns the tag name for filtering.
func (t TagInfo) FilterText() string {
	return t.Name
}

// DisplayText returns the tag name for display.
func (t TagInfo) DisplayText() string {
	return t.Name
}

// CountValue returns the task count for this tag.
func (t TagInfo) CountValue() int {
	return t.TaskCount
}

// ItemID returns the tag name as its unique identifier.
func (t TagInfo) ItemID() string {
	return t.Name
}

// TaskPickerItem wraps a Task to implement PickerItem.
type TaskPickerItem struct {
	Task *models.Task
}

// FilterText returns searchable text for task filtering.
func (t TaskPickerItem) FilterText() string {
	return fmt.Sprintf("#%d %s", t.Task.ShortID, t.Task.Title)
}

// DisplayText returns the task title for display.
func (t TaskPickerItem) DisplayText() string {
	return t.Task.Title
}

// CountValue returns 0 as tasks don't have a count.
func (t TaskPickerItem) CountValue() int {
	return 0
}

// ItemID returns the task ID as its unique identifier.
func (t TaskPickerItem) ItemID() string {
	return t.Task.ID
}

// CountDisplayer is implemented by PickerItems that want custom styled count
// text instead of the default "(N)" format. The returned string is pre-rendered
// and should not be wrapped in additional styles.
type CountDisplayer interface {
	CountDisplayText() string
}

// ProjectInfo holds project data for picker display.
type ProjectInfo struct {
	Project        *models.Project
	TaskCount      int
	CompletedCount int
}

// FilterText returns searchable text for project filtering.
func (p ProjectInfo) FilterText() string {
	return p.Project.Name + " " + p.Project.Context
}

// DisplayText returns the project name for display, with pin indicator if pinned.
func (p ProjectInfo) DisplayText() string {
	if p.Project.Pinned {
		return "*" + p.Project.Name
	}
	return p.Project.Name
}

// CountValue returns the task count for this project.
func (p ProjectInfo) CountValue() int {
	return p.TaskCount
}

// ItemID returns the project ID as its unique identifier.
func (p ProjectInfo) ItemID() string {
	return p.Project.ID
}

// CountDisplayText returns a styled string showing active and completed counts.
func (p ProjectInfo) CountDisplayText() string {
	active := lipgloss.NewStyle().Foreground(ColorHighlight).Render(fmt.Sprintf("%d active", p.TaskCount))
	done := lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("%d done", p.CompletedCount))
	return fmt.Sprintf("(%s / %s)", active, done)
}

// IsPinned returns whether the project is pinned.
func (p ProjectInfo) IsPinned() bool {
	return p.Project.Pinned
}

// TriggerFieldInfo wraps a workflow TriggerField with description for picker display.
type TriggerFieldInfo struct {
	Field       workflow.TriggerField
	Description string
}

// FilterText returns searchable text (field name + description).
func (t TriggerFieldInfo) FilterText() string {
	return string(t.Field) + " " + t.Description
}

// DisplayText returns the field name with description.
func (t TriggerFieldInfo) DisplayText() string {
	return fmt.Sprintf("%s - %s", t.Field, t.Description)
}

// CountValue returns 0 as fields don't have counts.
func (t TriggerFieldInfo) CountValue() int {
	return 0
}

// ItemID returns the field name as its unique identifier.
func (t TriggerFieldInfo) ItemID() string {
	return string(t.Field)
}

// ActionTypeInfo wraps a workflow ActionType with description for picker display.
type ActionTypeInfo struct {
	ActionType  workflow.ActionType
	Description string
}

// FilterText returns searchable text (action type + description).
func (a ActionTypeInfo) FilterText() string {
	return string(a.ActionType) + " " + a.Description
}

// DisplayText returns the action type with description.
func (a ActionTypeInfo) DisplayText() string {
	return fmt.Sprintf("%s - %s", a.ActionType, a.Description)
}

// CountValue returns 0 as action types don't have counts.
func (a ActionTypeInfo) CountValue() int {
	return 0
}

// ItemID returns the action type as its unique identifier.
func (a ActionTypeInfo) ItemID() string {
	return string(a.ActionType)
}

// TriggerFieldDescriptions maps trigger fields to human-readable descriptions.
var TriggerFieldDescriptions = map[workflow.TriggerField]string{
	workflow.FieldStatus:           "Current task status",
	workflow.FieldPriority:         "Task priority level (1-5)",
	workflow.FieldSection:          "Section name (Active, Backlog, etc.)",
	workflow.FieldTags:             "Task tags",
	workflow.FieldType:             "Task type (epic, story, task, bug, idea)",
	workflow.FieldDueDays:          "Days until due (negative = overdue)",
	workflow.FieldCreatedDays:      "Days since task creation",
	workflow.FieldUpdatedDays:      "Days since last update",
	workflow.FieldStatusChanged:    "Status transition (e.g., todo->done)",
	workflow.FieldSectionChanged:   "Section transition (e.g., Backlog->Active)",
	workflow.FieldPriorityChanged:  "Priority transition (e.g., 3->1)",
	workflow.FieldTagAdded:         "Tag was added to task",
	workflow.FieldTagRemoved:       "Tag was removed from task",
	workflow.FieldCompleted:        "Task was completed or canceled",
	workflow.FieldReopened:         "Task was reopened",
	workflow.FieldParentStatus:     "Parent task's status",
	workflow.FieldParentType:       "Parent task's type",
	workflow.FieldParentTags:       "Parent task has tag",
	workflow.FieldLevel:            "Hierarchy depth (0 = root)",
	workflow.FieldHasChildren:      "Task has subtasks",
	workflow.FieldChildrenProgress: "Subtask completion percentage (0-100)",
	workflow.FieldAllChildrenDone:  "All subtasks are complete",
	workflow.FieldIsBlocked:        "Task has incomplete dependencies",
	workflow.FieldBlockerCount:     "Number of incomplete blockers",
	// Event triggers (new)
	workflow.FieldDueDateChanged:      "Due date was added/removed/changed",
	workflow.FieldTypeChanged:         "Task type transition (e.g., task->bug)",
	workflow.FieldRecurrenceCompleted: "Recurring task completed its cycle",
	workflow.FieldStreakBroken:        "Recurrence streak was reset to zero",
	// Boolean state triggers
	workflow.FieldHasDueDate:       "Task has a due date set",
	workflow.FieldHasTags:          "Task has one or more tags",
	workflow.FieldDescriptionEmpty: "Task description is empty",
}

// ActionTypeDescriptions maps action types to human-readable descriptions.
var ActionTypeDescriptions = map[workflow.ActionType]string{
	workflow.ActionSetStatus:      "Change task status",
	workflow.ActionSetPriority:    "Change task priority",
	workflow.ActionSetSection:     "Move task to section",
	workflow.ActionAddTag:         "Add tag to task",
	workflow.ActionRemoveTag:      "Remove tag from task",
	workflow.ActionSetDueRelative: "Set due date relative to now (+3d, +1w)",
	workflow.ActionClearDue:       "Clear due date",
	workflow.ActionSetActive:      "Set/clear active flag",
	workflow.ActionSetPinned:      "Pin/unpin task",
	workflow.ActionAddComment:     "Add comment to task",
	workflow.ActionSetType:        "Change task type",
	workflow.ActionCompleteParent: "Mark parent task as done",
	workflow.ActionBlockParent:    "Mark parent task as blocked",
	workflow.ActionCascadeTag:     "Add tag to all children",
	workflow.ActionInheritTag:     "Copy tag from parent",
	workflow.ActionNotifyParent:   "Add comment to parent task",
	workflow.ActionSetSnooze:      "Snooze task for a duration (+3d, +1w)",
	workflow.ActionCreateFollowup: "Create a follow-up task",
	workflow.ActionMoveToProject:  "Move task to another project",
	workflow.ActionSetRecurrence:  "Set recurrence pattern (daily, 2w, etc.)",
}

// BuildTriggerFieldItems creates picker items for all trigger fields.
func BuildTriggerFieldItems() []TriggerFieldInfo {
	fields := workflow.ValidTriggerFields()
	items := make([]TriggerFieldInfo, len(fields))
	for i, f := range fields {
		desc := TriggerFieldDescriptions[f]
		if desc == "" {
			desc = string(f)
		}
		items[i] = TriggerFieldInfo{Field: f, Description: desc}
	}
	return items
}

// BuildActionTypeItems creates picker items for all action types.
func BuildActionTypeItems() []ActionTypeInfo {
	types := workflow.ValidActionTypes()
	items := make([]ActionTypeInfo, len(types))
	for i, t := range types {
		desc := ActionTypeDescriptions[t]
		if desc == "" {
			desc = string(t)
		}
		items[i] = ActionTypeInfo{ActionType: t, Description: desc}
	}
	return items
}
