package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
)

// Field indices for navigation
const (
	fieldStatus = iota
	fieldPriority
	fieldTags
	fieldSection
	fieldDueDate
	fieldActions
	fieldCount
)

// BulkEditModel handles bulk editing of multiple tasks.
type BulkEditModel struct {
	BaseView
	tasks   []*models.Task
	project *models.Project

	// Navigation
	focusIndex int // Which field is focused (0-5)

	// Status field
	statusCursor  int                // Cursor within status options (0=no change, 1-5=statuses)
	statusValue   *models.TaskStatus // nil = no change
	statusSummary string             // Summary of current values

	// Priority field
	priorityCursor  int    // Cursor within priority options (0=no change, 1-5=priorities)
	priorityValue   *int   // nil = no change
	prioritySummary string // Summary of current values

	// Tags field
	tagMode         int             // 0=normal, 1=add input, 2=remove selection
	tagAddInput     textinput.Model // Input for adding tags
	addTags         []string        // Tags to add
	removeTags      map[string]bool // Tags to remove (toggled)
	commonTags      []string        // Tags present on all tasks
	allTags         []string        // All unique tags across tasks
	tagRemoveCursor int             // Cursor for remove tag selection

	// Section field
	sectionCursor  int      // Cursor within section options
	sectionValue   *string  // nil = no change
	sectionSummary string   // Summary of current values
	sectionOptions []string // Available sections

	// Due date field
	dueDateCursor  int    // 0=no change, 1=set date, 2=clear
	dueDateSummary string // Summary of current values
	clearDueDate   bool   // Whether to clear due dates

	// Actions field
	actionCursor int // 0=Apply, 1=Cancel

	// Scroll management
	scrollOffset int // Lines scrolled from top
}

// NewBulkEditModel creates a new bulk edit panel for the given tasks.
func NewBulkEditModel(tasks []*models.Task, project *models.Project) *BulkEditModel {
	m := &BulkEditModel{
		tasks:      tasks,
		project:    project,
		focusIndex: fieldStatus,
		removeTags: make(map[string]bool),
	}

	// Initialize tag input
	m.tagAddInput = textinput.New()
	m.tagAddInput.Placeholder = "Enter tag name..."
	m.tagAddInput.CharLimit = 50
	m.tagAddInput.Width = 30

	// Analyze current values
	m.analyzeCurrentValues()

	// Build section options
	m.buildSectionOptions()

	return m
}

// analyzeCurrentValues examines all selected tasks to build summaries.
func (m *BulkEditModel) analyzeCurrentValues() {
	if len(m.tasks) == 0 {
		return
	}

	// Status analysis
	statusCounts := make(map[models.TaskStatus]int)
	for _, t := range m.tasks {
		statusCounts[t.Status]++
	}
	if len(statusCounts) == 1 {
		for s := range statusCounts {
			m.statusSummary = fmt.Sprintf("All: %s", s)
		}
	} else {
		parts := []string{}
		for s, count := range statusCounts {
			parts = append(parts, fmt.Sprintf("%d %s", count, s))
		}
		m.statusSummary = fmt.Sprintf("Mixed: %s", strings.Join(parts, ", "))
	}

	// Priority analysis
	priorities := make(map[int]bool)
	minP, maxP := 6, 0
	for _, t := range m.tasks {
		priorities[t.Priority] = true
		if t.Priority < minP {
			minP = t.Priority
		}
		if t.Priority > maxP {
			maxP = t.Priority
		}
	}
	if len(priorities) == 1 {
		m.prioritySummary = fmt.Sprintf("All: P%d", minP)
	} else {
		m.prioritySummary = fmt.Sprintf("Mixed: P%d-P%d", minP, maxP)
	}

	// Tag analysis - find common tags and all unique tags
	tagPresence := make(map[string]int)
	for _, t := range m.tasks {
		for _, tag := range t.Tags {
			tagPresence[tag]++
		}
	}

	m.commonTags = []string{}
	m.allTags = []string{}
	for tag, count := range tagPresence {
		m.allTags = append(m.allTags, tag)
		if count == len(m.tasks) {
			m.commonTags = append(m.commonTags, tag)
		}
	}
	sort.Strings(m.commonTags)
	sort.Strings(m.allTags)

	// Section analysis
	sections := make(map[string]int)
	for _, t := range m.tasks {
		sections[t.Section]++
	}
	if len(sections) == 1 {
		for s := range sections {
			if s == "" {
				m.sectionSummary = "All: Active (no section)"
			} else {
				m.sectionSummary = fmt.Sprintf("All: %s", s)
			}
		}
	} else {
		parts := []string{}
		for s, count := range sections {
			name := s
			if name == "" {
				name = "Active"
			}
			parts = append(parts, fmt.Sprintf("%d %s", count, name))
		}
		m.sectionSummary = fmt.Sprintf("Mixed: %s", strings.Join(parts, ", "))
	}

	// Due date analysis
	withDate, withoutDate := 0, 0
	for _, t := range m.tasks {
		if t.DueDate != nil {
			withDate++
		} else {
			withoutDate++
		}
	}
	if withDate == 0 {
		m.dueDateSummary = "None have due dates"
	} else if withoutDate == 0 {
		m.dueDateSummary = "All have due dates"
	} else {
		m.dueDateSummary = fmt.Sprintf("%d with dates, %d without", withDate, withoutDate)
	}
}

// buildSectionOptions creates the list of available sections.
func (m *BulkEditModel) buildSectionOptions() {
	sectionSet := make(map[string]bool)

	// Add project-defined sections
	for _, s := range m.project.GetSections() {
		sectionSet[s] = true
	}

	// Add sections currently used by tasks
	for _, t := range m.project.Tasks {
		if t.Section != "" {
			sectionSet[t.Section] = true
		}
	}

	// Build sorted list with empty string first (Active/clear)
	m.sectionOptions = []string{""}
	sections := []string{}
	for s := range sectionSet {
		if s != "" {
			sections = append(sections, s)
		}
	}
	sort.Strings(sections)
	m.sectionOptions = append(m.sectionOptions, sections...)
}

// fieldHeight returns the rendered height of a field in lines.
func (m *BulkEditModel) fieldHeight(fieldIndex int) int {
	switch fieldIndex {
	case fieldStatus:
		// Label line + 6 options + blank line
		return 8
	case fieldPriority:
		// Label line + 6 options + blank line
		return 8
	case fieldTags:
		// Base: label + controls hint = 2-3 lines
		height := 2
		if len(m.commonTags) > 0 {
			height++ // Common tags line
		}
		if m.tagMode == 1 {
			height++ // Add input line
		} else {
			if len(m.addTags) > 0 {
				height++ // Tags to add line
			}
			if len(m.removeTags) > 0 {
				height++ // Tags to remove line
			}
		}
		if m.tagMode == 2 {
			height += 1 + len(m.allTags) // "Select tags to remove:" + each tag
		}
		if m.focusIndex == fieldTags && m.tagMode == 0 {
			height++ // Controls hint line
		}
		height++ // Blank line after field
		return height
	case fieldSection:
		// Label line + "Keep current" + section options + blank line
		return 3 + len(m.sectionOptions)
	case fieldDueDate:
		// Label line + 3 options + blank line
		return 5
	case fieldActions:
		// Actions line + blank + footer
		return 3
	default:
		return 0
	}
}

// fieldStartLine returns the starting line number of a field (0-indexed).
func (m *BulkEditModel) fieldStartLine(fieldIndex int) int {
	// Header: title + blank line = 2 lines
	line := 2
	for i := 0; i < fieldIndex; i++ {
		line += m.fieldHeight(i)
	}
	return line
}

// viewportHeight returns the available height for content inside the box.
func (m *BulkEditModel) viewportHeight() int {
	// Account for box border (2 lines) and some padding
	// The centered box adds vertical padding, so we need to be conservative
	vh := m.Height() - 6
	if vh < 15 {
		vh = 15
	}
	return vh
}

// ensureFocusedFieldVisible adjusts scrollOffset to keep the focused field visible.
func (m *BulkEditModel) ensureFocusedFieldVisible() {
	viewportH := m.viewportHeight()

	fieldStart := m.fieldStartLine(m.focusIndex)
	fieldEnd := fieldStart + m.fieldHeight(m.focusIndex)

	// If the field starts above the visible area, scroll up to show it
	if fieldStart < m.scrollOffset {
		m.scrollOffset = fieldStart
	}

	// If the field ends below the visible area, scroll down to show it
	// But ensure we show at least the start of the field
	if fieldEnd > m.scrollOffset+viewportH {
		// Scroll so the field end is at the bottom of viewport
		m.scrollOffset = fieldEnd - viewportH
		// But don't scroll past the field start
		if m.scrollOffset > fieldStart {
			m.scrollOffset = fieldStart
		}
	}

	// Ensure scrollOffset doesn't go negative
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// Init initializes the model.
func (m *BulkEditModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *BulkEditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.ensureFocusedFieldVisible()
		return m, nil

	case tea.KeyMsg:
		// Handle tag add input mode
		if m.tagMode == 1 {
			return m.handleTagAddInput(msg)
		}

		switch msg.String() {
		case "esc", "q":
			if m.tagMode == 2 {
				m.tagMode = 0
				return m, nil
			}
			return m, m.cancel()

		case "tab", "down", "j":
			if m.focusIndex == fieldTags && m.tagMode == 2 {
				// Navigate within tag removal list
				if m.tagRemoveCursor < len(m.allTags)-1 {
					m.tagRemoveCursor++
				} else {
					m.tagMode = 0
					m.focusIndex = (m.focusIndex + 1) % fieldCount
				}
			} else {
				m.focusIndex = (m.focusIndex + 1) % fieldCount
			}
			m.ensureFocusedFieldVisible()
			return m, nil

		case "shift+tab", "up", "k":
			if m.focusIndex == fieldTags && m.tagMode == 2 {
				if m.tagRemoveCursor > 0 {
					m.tagRemoveCursor--
				} else {
					m.tagMode = 0
					m.focusIndex = (m.focusIndex - 1 + fieldCount) % fieldCount
				}
			} else {
				m.focusIndex = (m.focusIndex - 1 + fieldCount) % fieldCount
			}
			m.ensureFocusedFieldVisible()
			return m, nil

		case "enter":
			return m.handleEnter()

		case " ":
			return m.handleSpace()

		case "a":
			if m.focusIndex == fieldTags {
				m.tagMode = 1
				m.tagAddInput.Reset()
				m.tagAddInput.Focus()
				return m, textinput.Blink
			}

		case "r":
			if m.focusIndex == fieldTags && len(m.allTags) > 0 {
				m.tagMode = 2
				m.tagRemoveCursor = 0
			}

		case "l", "right":
			m.handleRight()

		case "h", "left":
			m.handleLeft()
		}
	}

	return m, nil
}

// handleTagAddInput handles input when adding a tag.
func (m *BulkEditModel) handleTagAddInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.tagMode = 0
		m.tagAddInput.Blur()
		return m, nil

	case "enter":
		tag := strings.TrimSpace(m.tagAddInput.Value())
		if tag != "" {
			// Check if already in add list
			found := false
			for _, t := range m.addTags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				m.addTags = append(m.addTags, tag)
				sort.Strings(m.addTags)
			}
		}
		m.tagMode = 0
		m.tagAddInput.Blur()
		return m, nil

	default:
		var cmd tea.Cmd
		m.tagAddInput, cmd = m.tagAddInput.Update(msg)
		return m, cmd
	}
}

// handleEnter handles the enter key based on current field.
func (m *BulkEditModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focusIndex {
	case fieldStatus:
		if m.statusCursor == 0 {
			m.statusValue = nil
		} else {
			statuses := []models.TaskStatus{
				models.StatusTodo,
				models.StatusInProgress,
				models.StatusDone,
				models.StatusBlocked,
				models.StatusCancelled,
			}
			if m.statusCursor <= len(statuses) {
				s := statuses[m.statusCursor-1]
				m.statusValue = &s
			}
		}

	case fieldPriority:
		if m.priorityCursor == 0 {
			m.priorityValue = nil
		} else {
			p := m.priorityCursor
			m.priorityValue = &p
		}

	case fieldSection:
		if m.sectionCursor == 0 {
			m.sectionValue = nil
		} else {
			idx := m.sectionCursor - 1
			if idx < len(m.sectionOptions) {
				s := m.sectionOptions[idx]
				m.sectionValue = &s
			}
		}

	case fieldDueDate:
		if m.dueDateCursor == 0 {
			m.clearDueDate = false
		} else if m.dueDateCursor == 2 {
			m.clearDueDate = true
		}
		// Note: dueDateCursor == 1 (Set date) would need date input integration

	case fieldActions:
		if m.actionCursor == 0 {
			return m, m.apply()
		}
		return m, m.cancel()
	}

	return m, nil
}

// handleSpace handles the space key for toggling.
func (m *BulkEditModel) handleSpace() (tea.Model, tea.Cmd) {
	switch m.focusIndex {
	case fieldTags:
		if m.tagMode == 2 && m.tagRemoveCursor < len(m.allTags) {
			tag := m.allTags[m.tagRemoveCursor]
			m.removeTags[tag] = !m.removeTags[tag]
			if !m.removeTags[tag] {
				delete(m.removeTags, tag)
			}
		}
	}
	return m, nil
}

// handleRight moves cursor right within current field.
func (m *BulkEditModel) handleRight() {
	switch m.focusIndex {
	case fieldStatus:
		if m.statusCursor < 5 {
			m.statusCursor++
		}
	case fieldPriority:
		if m.priorityCursor < 5 {
			m.priorityCursor++
		}
	case fieldSection:
		if m.sectionCursor < len(m.sectionOptions) {
			m.sectionCursor++
		}
	case fieldDueDate:
		if m.dueDateCursor < 2 {
			m.dueDateCursor++
		}
	case fieldActions:
		if m.actionCursor < 1 {
			m.actionCursor++
		}
	}
}

// handleLeft moves cursor left within current field.
func (m *BulkEditModel) handleLeft() {
	switch m.focusIndex {
	case fieldStatus:
		if m.statusCursor > 0 {
			m.statusCursor--
		}
	case fieldPriority:
		if m.priorityCursor > 0 {
			m.priorityCursor--
		}
	case fieldSection:
		if m.sectionCursor > 0 {
			m.sectionCursor--
		}
	case fieldDueDate:
		if m.dueDateCursor > 0 {
			m.dueDateCursor--
		}
	case fieldActions:
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	}
}

// apply returns a command that applies the changes.
func (m *BulkEditModel) apply() tea.Cmd {
	changes := m.buildChanges()
	return func() tea.Msg {
		return BulkEditDoneMsg{
			Changes:   changes,
			Tasks:     m.tasks,
			Project:   m.project,
			TaskCount: len(m.tasks),
			Applied:   true,
		}
	}
}

// cancel returns a command that cancels the bulk edit.
func (m *BulkEditModel) cancel() tea.Cmd {
	return func() tea.Msg {
		return BulkEditDoneMsg{
			Applied: false,
		}
	}
}

// buildChanges creates a PendingChanges struct from current selections.
func (m *BulkEditModel) buildChanges() PendingChanges {
	changes := PendingChanges{
		SetStatus:    m.statusValue,
		SetPriority:  m.priorityValue,
		AddTags:      m.addTags,
		SetSection:   m.sectionValue,
		ClearDueDate: m.clearDueDate,
	}

	// Collect remove tags
	for tag := range m.removeTags {
		changes.RemoveTags = append(changes.RemoveTags, tag)
	}
	sort.Strings(changes.RemoveTags)

	return changes
}

// View renders the bulk edit panel.
func (m *BulkEditModel) View() string {
	boxWidth := 60
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	// Title
	content.WriteString(TitleStyle.Render(fmt.Sprintf("Bulk Edit: %d tasks", len(m.tasks))))
	content.WriteString("\n\n")

	// Status field
	content.WriteString(m.renderStatusField())
	content.WriteString("\n")

	// Priority field
	content.WriteString(m.renderPriorityField())
	content.WriteString("\n")

	// Tags field
	content.WriteString(m.renderTagsField())
	content.WriteString("\n")

	// Section field
	content.WriteString(m.renderSectionField())
	content.WriteString("\n")

	// Due date field
	content.WriteString(m.renderDueDateField())
	content.WriteString("\n")

	// Preview
	content.WriteString(m.renderPreview())
	content.WriteString("\n")

	// Actions
	content.WriteString(m.renderActions())
	content.WriteString("\n\n")

	// Footer
	content.WriteString(m.renderFooter())

	// Apply scroll offset for viewport management
	fullContent := content.String()
	lines := strings.Split(fullContent, "\n")
	viewportH := m.viewportHeight()

	// Only apply scrolling if content exceeds viewport
	if len(lines) > viewportH {
		start := m.scrollOffset
		end := start + viewportH

		// Clamp end to content length
		if end > len(lines) {
			end = len(lines)
		}

		var visible strings.Builder

		// Show scroll-up indicator if content is hidden above
		if start > 0 {
			visible.WriteString(MutedStyle.Render("  ↑ scroll up for more"))
			visible.WriteString("\n")
		}

		// Render visible lines
		for i := start; i < end; i++ {
			visible.WriteString(lines[i])
			if i < end-1 {
				visible.WriteString("\n")
			}
		}

		// Show scroll-down indicator if content is hidden below
		if end < len(lines) {
			visible.WriteString("\n")
			visible.WriteString(MutedStyle.Render("  ↓ scroll down for more"))
		}

		return RenderCenteredBox(visible.String(), boxWidth, m.Width(), m.Height(), TagManagerBoxStyle)
	}

	return RenderCenteredBox(fullContent, boxWidth, m.Width(), m.Height(), TagManagerBoxStyle)
}

// renderStatusField renders the status selection field.
func (m *BulkEditModel) renderStatusField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldStatus
	label := "Status:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.statusSummary)))
	b.WriteString("\n")

	options := []string{"No change", "Todo", "In Progress", "Done", "Blocked", "Cancelled"}
	for i, opt := range options {
		prefix := "  "
		if focused {
			prefix = "    "
		}

		selected := (m.statusValue == nil && i == 0) ||
			(m.statusValue != nil && i > 0 && getStatusIndex(*m.statusValue) == i-1)

		cursor := "[ ]"
		if selected {
			cursor = "[x]"
		}
		if focused && m.statusCursor == i {
			cursor = "[>]"
		}

		line := fmt.Sprintf("%s%s %s", prefix, cursor, opt)
		if focused && m.statusCursor == i {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderPriorityField renders the priority selection field.
func (m *BulkEditModel) renderPriorityField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldPriority
	label := "Priority:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.prioritySummary)))
	b.WriteString("\n")

	options := []string{"Keep current", "1 - Critical", "2 - High", "3 - Normal", "4 - Low", "5 - Someday"}
	for i, opt := range options {
		prefix := "  "
		if focused {
			prefix = "    "
		}

		selected := (m.priorityValue == nil && i == 0) ||
			(m.priorityValue != nil && *m.priorityValue == i)

		cursor := "[ ]"
		if selected {
			cursor = "[x]"
		}
		if focused && m.priorityCursor == i {
			cursor = "[>]"
		}

		line := fmt.Sprintf("%s%s %s", prefix, cursor, opt)
		if focused && m.priorityCursor == i {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderTagsField renders the tags field.
func (m *BulkEditModel) renderTagsField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldTags
	label := "Tags:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("\n")

	prefix := "    "

	// Show common tags
	if len(m.commonTags) > 0 {
		b.WriteString(prefix)
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Common: %s", strings.Join(m.commonTags, ", "))))
		b.WriteString("\n")
	}

	// Tag add input mode
	if m.tagMode == 1 {
		b.WriteString(prefix)
		b.WriteString("Add: ")
		b.WriteString(m.tagAddInput.View())
		b.WriteString("\n")
	} else {
		// Show tags to add
		if len(m.addTags) > 0 {
			b.WriteString(prefix)
			b.WriteString(SuccessStyle.Render(fmt.Sprintf("+ %s", strings.Join(m.addTags, ", "))))
			b.WriteString("\n")
		}

		// Show tags to remove
		if len(m.removeTags) > 0 {
			tags := []string{}
			for t := range m.removeTags {
				tags = append(tags, t)
			}
			sort.Strings(tags)
			b.WriteString(prefix)
			b.WriteString(ValidationErrorStyle.Render(fmt.Sprintf("- %s", strings.Join(tags, ", "))))
			b.WriteString("\n")
		}
	}

	// Tag removal mode
	if m.tagMode == 2 {
		b.WriteString(prefix)
		b.WriteString("Select tags to remove:\n")
		for i, tag := range m.allTags {
			cursor := "[ ]"
			if m.removeTags[tag] {
				cursor = "[x]"
			}
			if m.tagRemoveCursor == i {
				cursor = "[>]"
			}
			line := fmt.Sprintf("%s  %s %s", prefix, cursor, tag)
			if m.tagRemoveCursor == i {
				b.WriteString(TaskJumperSelectedStyle.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	// Show controls
	if focused && m.tagMode == 0 {
		b.WriteString(prefix)
		b.WriteString(MutedStyle.Render("[a] add tag  [r] remove tag"))
		b.WriteString("\n")
	}

	return b.String()
}

// renderSectionField renders the section selection field.
func (m *BulkEditModel) renderSectionField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldSection
	label := "Section:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.sectionSummary)))
	b.WriteString("\n")

	prefix := "  "
	if focused {
		prefix = "    "
	}

	// Keep current option
	cursor := "[ ]"
	if m.sectionValue == nil {
		cursor = "[x]"
	}
	if focused && m.sectionCursor == 0 {
		cursor = "[>]"
	}
	line := fmt.Sprintf("%s%s Keep current", prefix, cursor)
	if focused && m.sectionCursor == 0 {
		b.WriteString(TaskJumperSelectedStyle.Render(line))
	} else {
		b.WriteString(line)
	}
	b.WriteString("\n")

	// Section options
	for i, section := range m.sectionOptions {
		displayName := section
		if displayName == "" {
			displayName = "Active (clear section)"
		}

		cursor := "[ ]"
		if m.sectionValue != nil && *m.sectionValue == section {
			cursor = "[x]"
		}
		if focused && m.sectionCursor == i+1 {
			cursor = "[>]"
		}

		line := fmt.Sprintf("%s%s %s", prefix, cursor, displayName)
		if focused && m.sectionCursor == i+1 {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderDueDateField renders the due date field.
func (m *BulkEditModel) renderDueDateField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldDueDate
	label := "Due Date:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.dueDateSummary)))
	b.WriteString("\n")

	prefix := "  "
	if focused {
		prefix = "    "
	}

	options := []string{"Keep current", "Set date... (not yet supported)", "Clear due date"}
	for i, opt := range options {
		cursor := "[ ]"
		if (i == 0 && !m.clearDueDate) || (i == 2 && m.clearDueDate) {
			cursor = "[x]"
		}
		if focused && m.dueDateCursor == i {
			cursor = "[>]"
		}

		line := fmt.Sprintf("%s%s %s", prefix, cursor, opt)
		if focused && m.dueDateCursor == i {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderPreview renders the preview of pending changes.
func (m *BulkEditModel) renderPreview() string {
	changes := m.buildChanges()
	if !changes.HasChanges() {
		return MutedStyle.Render("  Preview: No changes pending")
	}

	var parts []string

	if changes.SetStatus != nil {
		parts = append(parts, fmt.Sprintf("Status → %s", *changes.SetStatus))
	}
	if changes.SetPriority != nil {
		parts = append(parts, fmt.Sprintf("Priority → P%d", *changes.SetPriority))
	}
	for _, tag := range changes.AddTags {
		parts = append(parts, fmt.Sprintf("+%s", tag))
	}
	for _, tag := range changes.RemoveTags {
		parts = append(parts, fmt.Sprintf("-%s", tag))
	}
	if changes.SetSection != nil {
		if *changes.SetSection == "" {
			parts = append(parts, "Section → Active")
		} else {
			parts = append(parts, fmt.Sprintf("Section → %s", *changes.SetSection))
		}
	}
	if changes.ClearDueDate {
		parts = append(parts, "Clear due dates")
	}

	previewStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	return previewStyle.Render(fmt.Sprintf("  Preview: %s", strings.Join(parts, ", ")))
}

// renderActions renders the action buttons.
func (m *BulkEditModel) renderActions() string {
	var b strings.Builder

	focused := m.focusIndex == fieldActions

	applyStyle := NormalStyle
	cancelStyle := NormalStyle

	if focused {
		if m.actionCursor == 0 {
			applyStyle = TaskJumperSelectedStyle
		} else {
			cancelStyle = TaskJumperSelectedStyle
		}
	}

	changes := m.buildChanges()
	applyLabel := "Apply Changes"
	if !changes.HasChanges() {
		applyLabel = "(No changes)"
	}

	b.WriteString("  ")
	b.WriteString(applyStyle.Render(fmt.Sprintf("[%s]", applyLabel)))
	b.WriteString("   ")
	b.WriteString(cancelStyle.Render("[Cancel]"))

	return b.String()
}

// renderFooter renders the help footer.
func (m *BulkEditModel) renderFooter() string {
	if m.tagMode == 1 {
		return MutedStyle.Render("  Enter: add tag | Esc: cancel")
	}
	if m.tagMode == 2 {
		return MutedStyle.Render("  Space: toggle | Tab: next | Esc: done")
	}
	return MutedStyle.Render("  Tab/j/k: navigate | Enter: select | h/l: options | Esc: cancel")
}

// fieldLabel formats a field label.
func (m *BulkEditModel) fieldLabel(label string, focused bool) string {
	if focused {
		return BoldStyle.Render(label)
	}
	return label
}

// getStatusIndex returns the index of a status in the status list.
func getStatusIndex(s models.TaskStatus) int {
	switch s {
	case models.StatusTodo:
		return 0
	case models.StatusInProgress:
		return 1
	case models.StatusDone:
		return 2
	case models.StatusBlocked:
		return 3
	case models.StatusCancelled:
		return 4
	default:
		return 0
	}
}
