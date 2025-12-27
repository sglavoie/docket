package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// SectionInfo implements PickerItem for section selection.
// FilterText returns the section name for filtering.
func (s SectionInfo) FilterText() string {
	if s.Name == "" {
		return "(no section)"
	}
	return s.Name
}

// DisplayText returns the section name for display.
func (s SectionInfo) DisplayText() string {
	if s.Name == "" {
		return "(no section)"
	}
	return s.Name
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
func (c ContextInfo) ItemID() string {
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

// WrapTasks converts a slice of tasks to TaskPickerItems.
func WrapTasks(tasks []*models.Task) []TaskPickerItem {
	items := make([]TaskPickerItem, len(tasks))
	for i, t := range tasks {
		items[i] = TaskPickerItem{Task: t}
	}
	return items
}

// ProjectInfo holds project data for picker display.
type ProjectInfo struct {
	Project   *models.Project
	TaskCount int
}

// FilterText returns searchable text for project filtering.
func (p ProjectInfo) FilterText() string {
	return p.Project.Name + " " + p.Project.Context
}

// DisplayText returns the project name for display.
func (p ProjectInfo) DisplayText() string {
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
