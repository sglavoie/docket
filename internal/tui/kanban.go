package tui

import "github.com/sglavoie/tt/internal/models"

// Section name constants for Kanban columns.
// Active is represented by empty string per existing convention.
const (
	KanbanSectionActive  = ""
	KanbanSectionNext    = "Next"
	KanbanSectionInbox   = "Inbox"
	KanbanSectionWaiting = "Waiting"
	KanbanSectionSomeday = "Someday"
)

// KanbanColumnOrder defines the left-to-right display order of columns.
var KanbanColumnOrder = []string{
	KanbanSectionActive,
	KanbanSectionNext,
	KanbanSectionInbox,
	KanbanSectionWaiting,
	KanbanSectionSomeday,
}

// ColumnState encapsulates the cursor position and scroll offset for a single
// Kanban column. Each column maintains independent state since columns have
// different task counts.
type ColumnState struct {
	Cursor       int
	ScrollOffset int
}

// KanbanViewModel manages the Kanban board view state.
// It displays non-completed tasks organized into five columns by section.
type KanbanViewModel struct {
	BaseView

	// Project being displayed
	Project *models.Project

	// Columns maps section name to column state.
	// Keys: "" (Active), "Next", "Inbox", "Waiting", "Someday"
	Columns map[string]*ColumnState

	// ActiveColumn tracks which column is currently focused (index into KanbanColumnOrder)
	ActiveColumn int
}

// NewKanbanViewModel creates a new Kanban view model for the given project.
func NewKanbanViewModel(project *models.Project) *KanbanViewModel {
	m := &KanbanViewModel{
		Project: project,
		Columns: make(map[string]*ColumnState),
	}

	// Initialize column state for each of the five sections
	for _, section := range KanbanColumnOrder {
		m.Columns[section] = &ColumnState{}
	}

	return m
}

// CurrentColumnState returns the ColumnState for the currently active column.
func (m *KanbanViewModel) CurrentColumnState() *ColumnState {
	section := KanbanColumnOrder[m.ActiveColumn]
	return m.Columns[section]
}

// CurrentSection returns the section name of the currently active column.
func (m *KanbanViewModel) CurrentSection() string {
	return KanbanColumnOrder[m.ActiveColumn]
}

// Validate adjusts Cursor and ScrollOffset to be within valid bounds for the
// given task count. Call this after any mutation that changes the column's
// task list (task moves, status changes, deletions).
func (cs *ColumnState) Validate(taskCount int) {
	cs.Cursor = ClampCursor(cs.Cursor, taskCount)
	cs.validateScrollOffset(taskCount)
}

// validateScrollOffset ensures ScrollOffset keeps the selected task visible.
// This is called internally by Validate.
func (cs *ColumnState) validateScrollOffset(taskCount int) {
	if taskCount == 0 {
		cs.ScrollOffset = 0
		return
	}
	if cs.ScrollOffset < 0 {
		cs.ScrollOffset = 0
	}
	// Ensure cursor is visible (scroll offset should not exceed cursor)
	if cs.ScrollOffset > cs.Cursor {
		cs.ScrollOffset = cs.Cursor
	}
}

// InBounds returns true if the cursor is valid for the given task count.
func (cs *ColumnState) InBounds(taskCount int) bool {
	return CursorInBounds(cs.Cursor, taskCount)
}

// AdjustScrollForViewport adjusts the scroll offset to ensure the cursor
// remains visible within a viewport of the given height.
func (cs *ColumnState) AdjustScrollForViewport(taskCount, viewportHeight int) {
	if taskCount == 0 || viewportHeight <= 0 {
		cs.ScrollOffset = 0
		return
	}

	// If cursor is above the viewport, scroll up
	if cs.Cursor < cs.ScrollOffset {
		cs.ScrollOffset = cs.Cursor
	}

	// If cursor is below the viewport, scroll down
	if cs.Cursor >= cs.ScrollOffset+viewportHeight {
		cs.ScrollOffset = cs.Cursor - viewportHeight + 1
	}

	// Clamp scroll offset to valid range
	maxOffset := taskCount - viewportHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if cs.ScrollOffset > maxOffset {
		cs.ScrollOffset = maxOffset
	}
	if cs.ScrollOffset < 0 {
		cs.ScrollOffset = 0
	}
}
