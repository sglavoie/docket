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

// Minimum width for a Kanban column to remain legible.
// This ensures column headers and truncated task titles are readable.
const KanbanMinColumnWidth = 15

// KanbanViewModel manages the Kanban board view state.
// It displays non-completed tasks organized into five columns by section.
type KanbanViewModel struct {
	BaseView

	// Project being displayed
	Project *models.Project

	// Columns maps section name to column state.
	// Keys: "" (Active), "Next", "Inbox", "Waiting", "Someday"
	Columns map[string]*ColumnState

	// Tasks maps section name to the list of non-completed tasks in that section.
	// This is populated during construction and should be refreshed when tasks change.
	Tasks map[string][]*models.Task

	// ActiveColumn tracks which column is currently focused (index into KanbanColumnOrder)
	ActiveColumn int

	// ColumnWidth is the calculated width for each column after SetSize.
	// All columns share the same width for uniform appearance.
	ColumnWidth int
}

// NewKanbanViewModel creates a new Kanban view model for the given project.
// It initializes five columns mapped to Active/Next/Inbox/Waiting/Someday
// and loads non-completed tasks filtered by their Section field.
func NewKanbanViewModel(project *models.Project) *KanbanViewModel {
	m := &KanbanViewModel{
		Project: project,
		Columns: make(map[string]*ColumnState),
		Tasks:   make(map[string][]*models.Task),
	}

	// Initialize column state for each of the five sections
	for _, section := range KanbanColumnOrder {
		m.Columns[section] = &ColumnState{}
		m.Tasks[section] = []*models.Task{}
	}

	// Load tasks into columns by section, excluding completed tasks
	m.loadTasks()

	return m
}

// loadTasks populates the Tasks map by filtering project tasks into columns.
// Only non-completed tasks are included. Each task is placed in the column
// matching its Section field (empty string maps to Active column).
func (m *KanbanViewModel) loadTasks() {
	// Clear existing tasks
	for _, section := range KanbanColumnOrder {
		m.Tasks[section] = []*models.Task{}
	}

	if m.Project == nil {
		return
	}

	// Filter and categorize tasks by section
	for _, task := range m.Project.Tasks {
		// Skip completed tasks (done or canceled)
		if task.Status.IsComplete() {
			continue
		}

		// Determine which column this task belongs to
		section := task.Section
		// Validate the section is one of our known columns
		if _, exists := m.Columns[section]; !exists {
			// Unknown section defaults to Active (empty string)
			section = KanbanSectionActive
		}

		m.Tasks[section] = append(m.Tasks[section], task)
	}

	// Validate cursor positions for each column after loading
	for _, section := range KanbanColumnOrder {
		taskCount := len(m.Tasks[section])
		m.Columns[section].Validate(taskCount)
	}
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

// CurrentColumnTasks returns the tasks in the currently active column.
func (m *KanbanViewModel) CurrentColumnTasks() []*models.Task {
	section := KanbanColumnOrder[m.ActiveColumn]
	return m.Tasks[section]
}

// TasksForSection returns the tasks in the specified section.
func (m *KanbanViewModel) TasksForSection(section string) []*models.Task {
	if tasks, exists := m.Tasks[section]; exists {
		return tasks
	}
	return nil
}

// SetSize distributes the available width across five columns.
// Each column gets an equal share of the available width minus gaps between columns.
// If the calculated width falls below the minimum, columns use the minimum width
// which may cause horizontal overflow on very narrow terminals.
func (m *KanbanViewModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)

	// Account for gaps between columns (1 char gap between each pair)
	numColumns := len(KanbanColumnOrder)
	numGaps := numColumns - 1
	availableWidth := width - numGaps

	// Distribute width evenly across columns
	columnWidth := availableWidth / numColumns

	// Enforce minimum width to keep columns legible
	if columnWidth < KanbanMinColumnWidth {
		columnWidth = KanbanMinColumnWidth
	}

	m.ColumnWidth = columnWidth
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
