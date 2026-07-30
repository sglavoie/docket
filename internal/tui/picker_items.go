package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

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
	return TaskTitleStyleFor(t.Task, TaskTitleStyleOptions{}).Render(t.Task.Title)
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
