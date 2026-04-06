package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// Field indices for navigation
const (
	fieldStatus = iota
	fieldType
	fieldPriority
	fieldTags
	fieldSection
	fieldDueDate
	fieldRecurrence
	fieldParent
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
	statusCursor  int                // Cursor within status options (0=no change, 1-7=statuses)
	statusValue   *models.TaskStatus // nil = no change
	statusSummary string             // Summary of current values

	// Type field
	typeCursor  int              // Cursor within type options (0=no change, 1-6=types)
	typeValue   *models.TaskType // nil = no change
	typeSummary string           // Summary of current values

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
	dueDateCursor    int             // 0=no change, 1=set date, 2=clear
	dueDateSummary   string          // Summary of current values
	clearDueDate     bool            // Whether to clear due dates
	setDueDate       *time.Time      // Selected date to apply (nil = no change)
	dueDateInputMode bool            // True when typing date
	dueDateInput     textinput.Model // Text input for date entry
	dueDateError     error           // Validation error

	// Recurrence field
	recurrenceCursor  int                       // 0=keep, 1-5=presets, 6=clear
	recurrenceValue   *models.RecurrencePattern // nil = no change
	clearRecurrence   bool                      // Whether to clear recurrence
	recurrenceSummary string                    // Summary of current values

	// Parent field
	parentCursor     int            // 0=Keep, 1=Set parent, 2=Move to root
	parentValue      *string        // Selected parent task ID
	clearParent      bool           // Whether to move to root
	parentSummary    string         // Summary of current parents
	parentCandidates []*models.Task // Valid parent candidates
	parentListMode   bool           // True when browsing candidate list
	parentListCursor int            // Cursor within candidate list
	parentListOffset int            // Scroll offset for candidate list

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
	m.tagAddInput.SetWidth(30)

	// Initialize due date input
	m.dueDateInput = textinput.New()
	m.dueDateInput.Placeholder = "e.g., tomorrow, +3d, fri, 2024-01-15"
	m.dueDateInput.CharLimit = 30
	m.dueDateInput.SetWidth(40)

	// Analyze current values
	m.analyzeCurrentValues()

	// Build section options
	m.buildSectionOptions()

	// Build parent candidates
	m.buildParentCandidates()

	return m
}

// analyzeCurrentValues examines all selected tasks to build summaries.
func (m *BulkEditModel) analyzeCurrentValues() { //nolint:gocyclo
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

	// Type analysis
	typeCounts := make(map[models.TaskType]int)
	for _, t := range m.tasks {
		typeCounts[t.Type]++
	}
	if len(typeCounts) == 1 {
		for tp := range typeCounts {
			m.typeSummary = fmt.Sprintf("All: %s", tp.HeadingPrefix())
		}
	} else {
		parts := []string{}
		for tp, count := range typeCounts {
			parts = append(parts, fmt.Sprintf("%d %s", count, tp.HeadingPrefix()))
		}
		m.typeSummary = fmt.Sprintf("Mixed: %s", strings.Join(parts, ", "))
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

	// Recurrence analysis
	withRecurrence, withoutRecurrence := 0, 0
	recurrencePatterns := make(map[string]int)
	for _, t := range m.tasks {
		if t.Recurrence != nil {
			withRecurrence++
			recurrencePatterns[t.Recurrence.String()]++
		} else {
			withoutRecurrence++
		}
	}
	if withRecurrence == 0 {
		m.recurrenceSummary = "None recurring"
	} else if withoutRecurrence == 0 {
		if len(recurrencePatterns) == 1 {
			for p := range recurrencePatterns {
				m.recurrenceSummary = fmt.Sprintf("All: %s", p)
			}
		} else {
			m.recurrenceSummary = "All recurring (mixed)"
		}
	} else {
		m.recurrenceSummary = fmt.Sprintf("%d recurring, %d not", withRecurrence, withoutRecurrence)
	}

	// Parent analysis
	parentCounts := make(map[string]int)
	rootCount := 0
	for _, t := range m.tasks {
		if t.ParentID == nil {
			rootCount++
		} else {
			parentCounts[*t.ParentID]++
		}
	}
	if rootCount == len(m.tasks) {
		m.parentSummary = "All at root"
	} else if rootCount == 0 && len(parentCounts) == 1 {
		for pid := range parentCounts {
			// Find parent task name
			for _, t := range m.project.Tasks {
				if t.ID == pid {
					m.parentSummary = fmt.Sprintf("All under: %s", t.Title)
					break
				}
			}
		}
	} else {
		m.parentSummary = fmt.Sprintf("%d root, %d with parent", rootCount, len(m.tasks)-rootCount)
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

// buildParentCandidates creates the list of valid parent targets.
// Excludes tasks in the selected set and tasks that cannot have children.
func (m *BulkEditModel) buildParentCandidates() {
	selectedSet := make(map[string]bool, len(m.tasks))
	for _, t := range m.tasks {
		selectedSet[t.ID] = true
	}

	m.parentCandidates = nil
	for _, t := range m.project.Tasks {
		if selectedSet[t.ID] {
			continue
		}
		if !t.Type.CanHaveChildren() {
			continue
		}
		m.parentCandidates = append(m.parentCandidates, t)
	}
}

// fieldHeight returns the rendered height of a field in lines.
func (m *BulkEditModel) fieldHeight(fieldIndex int) int {
	switch fieldIndex {
	case fieldStatus:
		// Label line + 6 options + blank line
		return 8
	case fieldType:
		// Label line + 7 options (no change + 6 types) + blank line
		return 9
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
		height := 5
		if m.dueDateInputMode {
			height++ // Input line takes extra space
			if m.dueDateError != nil {
				height++ // Error line
			}
		}
		return height
	case fieldRecurrence:
		// Label line + 7 options (keep + 5 presets + clear) + blank line
		return 9
	case fieldParent:
		// Label line + 3 options (keep/set/root) + blank line = 5
		height := 5
		if m.parentListMode {
			// Show up to 8 candidates in the scrollable list
			visible := min(len(m.parentCandidates), 8)
			if visible == 0 {
				visible = 1 // "No valid parents" message
			}
			height += visible
		}
		return height
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
	line := 0
	for i := range fieldIndex {
		line += m.fieldHeight(i)
	}
	return line
}

// viewportHeight returns the available height for content inside the box.
func (m *BulkEditModel) viewportHeight() int {
	// Account for box border (2 lines) and some padding
	// The centered box adds vertical padding, so we need to be conservative
	vh := max(m.Height()-8, 15)
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
		m.scrollOffset = min(
			// But don't scroll past the field start
			fieldEnd-viewportH, fieldStart)
	}

	// Ensure scrollOffset doesn't go negative
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// HasTextInput returns true when a text input field is active.
func (m *BulkEditModel) HasTextInput() bool {
	return m.tagMode == 1 || m.dueDateInputMode
}

// ShouldCaptureEsc returns true when the view needs to handle esc internally.
func (m *BulkEditModel) ShouldCaptureEsc() bool {
	return m.tagMode == 1 || m.tagMode == 2 || m.dueDateInputMode
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

	case tea.KeyPressMsg:
		// Handle tag add input mode
		if m.tagMode == 1 {
			return m.handleTagAddInput(msg)
		}

		// Handle due date input mode
		if m.dueDateInputMode {
			return m.handleDueDateInput(msg)
		}

		// Handle parent list browsing mode
		if m.parentListMode {
			return m.handleParentListInput(msg)
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

		case "space":
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

func (m *BulkEditModel) HandledKeys() []string {
	return []string{
		// Normal navigation
		"tab", "down", "j", "shift+tab", "up", "k",
		// Field interaction
		"enter", "space", "l", "right", "h", "left",
		// Tag operations
		"a", "r",
	}
}

func (m *BulkEditModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewBulkEdit)
}

// View renders the bulk edit panel.
func (m *BulkEditModel) View() tea.View {
	boxWidth := 60
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}
	_ = boxWidth

	var content strings.Builder

	// Status field
	content.WriteString(m.renderStatusField())
	content.WriteString("\n")

	// Type field
	content.WriteString(m.renderTypeField())
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

	// Recurrence field
	content.WriteString(m.renderRecurrenceField())
	content.WriteString("\n")

	// Parent field
	content.WriteString(m.renderParentField())
	content.WriteString("\n")

	// Preview
	content.WriteString(m.renderPreview())
	content.WriteString("\n")

	// Actions
	content.WriteString(m.renderActions())
	content.WriteString("\n\n")

	// Footer
	content.WriteString(m.renderFooter())

	// Build subtitle
	subtitle := fmt.Sprintf("%d tasks", len(m.tasks))

	// Apply scroll offset for viewport management
	fullContent := content.String()
	lines := strings.Split(fullContent, "\n")
	viewportH := m.viewportHeight()

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Bulk Edit", subtitle, m.AccentColor()))
	out.WriteString("\n\n")

	// Only apply scrolling if content exceeds viewport
	if len(lines) > viewportH {
		start := m.scrollOffset
		end := min(start+viewportH, len(lines))

		if start > 0 {
			out.WriteString(MutedStyle.Render("  ↑ scroll up for more"))
			out.WriteString("\n")
		}

		for i := start; i < end; i++ {
			out.WriteString(lines[i])
			if i < end-1 {
				out.WriteString("\n")
			}
		}

		if end < len(lines) {
			out.WriteString("\n")
			out.WriteString(MutedStyle.Render("  ↓ scroll down for more"))
		}
	} else {
		out.WriteString(fullContent)
	}

	return tea.NewView(out.String())
}

func (m *BulkEditModel) StatusBarLabel() string { return "Bulk Edit" }
