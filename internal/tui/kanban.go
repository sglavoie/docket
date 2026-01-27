package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
)

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

// Init implements the tea.Model interface. The Kanban view does not require
// any initialization commands as state is set up during construction.
func (m *KanbanViewModel) Init() tea.Cmd {
	return nil
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

// Update implements the tea.Model interface. It handles window resize messages
// and keyboard navigation, delegating key handling to handleKeyMsg.
func (m *KanbanViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

// handleKeyMsg processes keyboard input for the Kanban view.
func (m *KanbanViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc", "q":
		return m, func() tea.Msg {
			return CloseKanbanMsg{}
		}

	// Vertical navigation within current column
	case "up", "k":
		m.moveCursorUp()
		return m, nil

	case "down", "j":
		m.moveCursorDown()
		return m, nil

	// Horizontal navigation between columns
	case "left", "h":
		m.moveColumnLeft()
		return m, nil

	case "right", "l":
		m.moveColumnRight()
		return m, nil

	// Jump to first/last task in column
	case "g":
		// Go to first task in current column
		cs := m.CurrentColumnState()
		cs.Cursor = 0
		return m, nil

	case "G":
		// Go to last task in current column
		tasks := m.CurrentColumnTasks()
		if len(tasks) > 0 {
			cs := m.CurrentColumnState()
			cs.Cursor = len(tasks) - 1
		}
		return m, nil
	}

	return m, nil
}

// moveCursorUp moves the cursor up within the current column.
func (m *KanbanViewModel) moveCursorUp() {
	cs := m.CurrentColumnState()
	if cs.Cursor > 0 {
		cs.Cursor--
	}
}

// moveCursorDown moves the cursor down within the current column.
func (m *KanbanViewModel) moveCursorDown() {
	cs := m.CurrentColumnState()
	tasks := m.CurrentColumnTasks()
	if cs.Cursor < len(tasks)-1 {
		cs.Cursor++
	}
}

// moveColumnLeft moves focus to the column on the left.
func (m *KanbanViewModel) moveColumnLeft() {
	if m.ActiveColumn > 0 {
		m.ActiveColumn--
	}
}

// moveColumnRight moves focus to the column on the right.
func (m *KanbanViewModel) moveColumnRight() {
	if m.ActiveColumn < len(KanbanColumnOrder)-1 {
		m.ActiveColumn++
	}
}

// View implements the tea.Model interface. Renders five columns with
// section-colored headers: Active, Next, Inbox, Waiting, Someday.
func (m *KanbanViewModel) View() string {
	if m.Project == nil {
		return MutedStyle.Render("No project loaded")
	}

	var columns []string
	gap := " " // Single space gap between columns

	for i, section := range KanbanColumnOrder {
		columnContent := m.renderColumn(section, i == m.ActiveColumn)
		columns = append(columns, columnContent)
	}

	// Join columns horizontally with gaps
	var parts []string
	for i, col := range columns {
		parts = append(parts, col)
		if i < len(columns)-1 {
			parts = append(parts, gap)
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderColumn renders a single Kanban column with header and task list.
func (m *KanbanViewModel) renderColumn(section string, isActive bool) string {
	var b strings.Builder

	// Render header with section color
	header := m.renderColumnHeader(section, isActive)
	b.WriteString(header)
	b.WriteString("\n")

	// Render tasks in this column
	tasks := m.Tasks[section]
	cs := m.Columns[section]

	if len(tasks) == 0 {
		b.WriteString(MutedStyle.Render("(empty)"))
	} else {
		for i, task := range tasks {
			isSelected := i == cs.Cursor && isActive
			line := m.renderTaskRow(task, isSelected)
			b.WriteString(line)
			if i < len(tasks)-1 {
				b.WriteString("\n")
			}
		}
	}

	// Apply column width styling
	columnStyle := lipgloss.NewStyle().Width(m.ColumnWidth)
	return columnStyle.Render(b.String())
}

// renderColumnHeader renders the column header with appropriate section color.
func (m *KanbanViewModel) renderColumnHeader(section string, isActive bool) string {
	displayName := sectionDisplayName(section)
	taskCount := len(m.Tasks[section])
	headerText := fmt.Sprintf("%s (%d)", displayName, taskCount)

	// Get the section color from the project
	colorName := m.Project.GetSectionColor(section)
	style := GetSectionColorStyle(colorName)

	// Make header bold and add indicator if this column is active
	style = style.Bold(true)
	if isActive {
		headerText = "▸ " + headerText
	}

	return style.Render(headerText)
}

// renderTaskRow renders a single task row, with selection indicator if selected.
func (m *KanbanViewModel) renderTaskRow(task *models.Task, isSelected bool) string {
	// Build task display: status icon + truncated title
	statusIcon := task.Status.Icon()
	title := task.Title

	// Truncate title to fit in column (accounting for icon and spacing)
	maxTitleLen := m.ColumnWidth - 4 // Account for icon, space, and potential cursor
	if len(title) > maxTitleLen && maxTitleLen > 0 {
		if maxTitleLen > 3 {
			title = title[:maxTitleLen-3] + "..."
		} else {
			title = title[:maxTitleLen]
		}
	}

	statusStyle := GetStatusStyle(string(task.Status))
	content := statusStyle.Render(statusIcon) + " " + title

	if isSelected {
		return SelectionCursorStyle.Render(">") + " " + content
	}
	return "  " + content
}

// sectionDisplayName returns the display name for a section.
// The Active section (empty string) displays as "Active".
func sectionDisplayName(section string) string {
	if section == "" {
		return "Active"
	}
	return section
}

// ShouldCaptureEsc implements ViewCapabilities. The Kanban view does not need
// to capture esc/q as it has no modal states or text inputs.
func (m *KanbanViewModel) ShouldCaptureEsc() bool {
	return false
}

// HasTextInput implements ViewCapabilities. The Kanban view has no text input fields.
func (m *KanbanViewModel) HasTextInput() bool {
	return false
}
