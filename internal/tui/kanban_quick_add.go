package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// KanbanQuickAddCreatedMsg signals that a task was created via quick add.
type KanbanQuickAddCreatedMsg struct {
	Task *models.Task
}

// handleQuickAddInput processes keyboard input while in quick add mode.
func (m *KanbanViewModel) handleQuickAddInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle autocomplete navigation when active
	if m.quickAddAcMode != acModeNone && len(m.quickAddSuggestions) > 0 {
		switch key {
		case "tab", "enter":
			m.acceptQuickAddSuggestion()
			m.updateQuickAddPreview()
			return m, nil
		case "esc":
			// Dismiss autocomplete dropdown only
			m.quickAddAcMode = acModeNone
			m.quickAddSuggestions = nil
			m.quickAddAcCursor = 0
			m.quickAddAcViewportTop = 0
			return m, nil
		case "up", "ctrl+p", "ctrl+k":
			if m.quickAddAcCursor > 0 {
				m.quickAddAcCursor--
				// Scroll viewport up if cursor moves above visible area
				if m.quickAddAcCursor < m.quickAddAcViewportTop {
					m.quickAddAcViewportTop = m.quickAddAcCursor
				}
			}
			return m, nil
		case "down", "ctrl+n", "ctrl+j":
			if m.quickAddAcCursor < len(m.quickAddSuggestions)-1 {
				m.quickAddAcCursor++
				// Scroll viewport down if cursor moves below visible area
				if m.quickAddAcCursor >= m.quickAddAcViewportTop+acVisibleCount {
					m.quickAddAcViewportTop = m.quickAddAcCursor - acVisibleCount + 1
				}
			}
			return m, nil
		}
	}

	// Regular key handling
	switch key {
	case "esc":
		// Cancel quick add mode
		m.quickAddMode = false
		m.quickAddInput.Blur()
		m.quickAddInput.SetValue("")
		m.quickAddPreview = nil
		m.quickAddError = ""
		m.quickAddAcMode = acModeNone
		m.quickAddSuggestions = nil
		return m, nil

	case "enter":
		// Create task if not in autocomplete mode
		if m.quickAddAcMode == acModeNone {
			return m, m.createQuickAddTask()
		}
	}

	// Update textinput
	var cmd tea.Cmd
	m.quickAddInput, cmd = m.quickAddInput.Update(msg)

	// Update preview and detect autocomplete triggers
	m.updateQuickAddPreview()
	m.detectQuickAddTrigger()

	return m, cmd
}

// updateQuickAddPreview updates the live preview state.
func (m *KanbanViewModel) updateQuickAddPreview() {
	input := strings.TrimSpace(m.quickAddInput.Value())
	if input == "" {
		m.quickAddPreview = nil
		m.quickAddError = ""
		return
	}
	// Use nil for allProjects since we don't support cross-project in Kanban quick add
	m.quickAddPreview, m.quickAddError = parseQuickCaptureForPreview(input, m.Project, nil)
}

// detectQuickAddTrigger scans the input for autocomplete triggers and populates suggestions.
func (m *KanbanViewModel) detectQuickAddTrigger() {
	input := m.quickAddInput.Value()
	if len(input) == 0 {
		m.quickAddAcMode = acModeNone
		m.quickAddSuggestions = nil
		return
	}

	// Scan backward from end to find most recent trigger
	// Triggers only activate if at start of input or preceded by whitespace
	var mode int
	var triggerPos int
	var partial string

	for i := len(input) - 1; i >= 0; i-- {
		ch := input[i]
		if ch == ' ' || ch == '\n' {
			// Hit whitespace before finding trigger
			break
		}
		// Check if trigger is at start or preceded by whitespace
		atStartOrAfterSpace := i == 0 || input[i-1] == ' ' || input[i-1] == '\n'
		if !atStartOrAfterSpace {
			continue
		}
		partial = input[i+1:]
		switch ch {
		case '#':
			mode = acModeTag
			triggerPos = i
		case '>':
			mode = acModeParent
			triggerPos = i
		case '^':
			mode = acModeType
			triggerPos = i
		case '!':
			mode = acModePriority
			triggerPos = i
		case '@':
			mode = acModeDate
			triggerPos = i
		case '~':
			mode = acModeRecur
			triggerPos = i
		case '/':
			mode = acModeSection
			triggerPos = i
		case '$':
			mode = acModeStatus
			triggerPos = i
		}
		if mode != acModeNone {
			break
		}
	}

	if mode == acModeNone {
		m.quickAddAcMode = acModeNone
		m.quickAddSuggestions = nil
		return
	}

	// Only reset cursor if entering new autocomplete context
	if mode != m.quickAddAcMode || triggerPos != m.quickAddAcTriggerPos {
		m.quickAddAcCursor = 0
		m.quickAddAcViewportTop = 0
	}
	m.quickAddAcMode = mode
	m.quickAddAcTriggerPos = triggerPos

	// Get suggestions based on mode
	switch mode {
	case acModeTag:
		m.quickAddSuggestions = m.getQuickAddTagSuggestions(partial)
	case acModeParent:
		m.quickAddSuggestions = m.getQuickAddParentSuggestions(partial)
	case acModeType:
		m.quickAddSuggestions = m.getQuickAddTypeSuggestions(partial)
	case acModePriority:
		m.quickAddSuggestions = m.getQuickAddPrioritySuggestions(partial)
	case acModeDate:
		m.quickAddSuggestions = m.getQuickAddDateSuggestions(partial)
	case acModeRecur:
		m.quickAddSuggestions = m.getQuickAddRecurrenceSuggestions(partial)
	case acModeSection:
		m.quickAddSuggestions = m.getQuickAddSectionSuggestions(partial)
	case acModeStatus:
		m.quickAddSuggestions = m.getQuickAddStatusSuggestions(partial)
	}

	// Clamp cursor and viewport if suggestions list shrunk
	if m.quickAddAcCursor >= len(m.quickAddSuggestions) {
		m.quickAddAcCursor = max(0, len(m.quickAddSuggestions)-1)
	}
	// Ensure viewport keeps cursor visible
	if m.quickAddAcCursor < m.quickAddAcViewportTop {
		m.quickAddAcViewportTop = m.quickAddAcCursor
	} else if m.quickAddAcCursor >= m.quickAddAcViewportTop+acVisibleCount {
		m.quickAddAcViewportTop = m.quickAddAcCursor - acVisibleCount + 1
	}
	// If no suggestions, exit autocomplete mode
	if len(m.quickAddSuggestions) == 0 {
		m.quickAddAcMode = acModeNone
	}
}

// acceptQuickAddSuggestion inserts the selected suggestion into the input.
func (m *KanbanViewModel) acceptQuickAddSuggestion() {
	if m.quickAddAcMode == acModeNone || len(m.quickAddSuggestions) == 0 {
		return
	}

	selected := m.quickAddSuggestions[m.quickAddAcCursor]
	input := m.quickAddInput.Value()

	// Replace from trigger position to end with the completed value
	var newInput string
	switch m.quickAddAcMode {
	case acModeTag:
		newInput = input[:m.quickAddAcTriggerPos] + "#" + selected.value + " "
	case acModeParent:
		newInput = input[:m.quickAddAcTriggerPos] + ">" + selected.value + " "
	case acModeType:
		newInput = input[:m.quickAddAcTriggerPos] + "^" + selected.value + " "
	case acModePriority:
		newInput = input[:m.quickAddAcTriggerPos] + "!" + selected.value + " "
	case acModeDate:
		newInput = input[:m.quickAddAcTriggerPos] + "@" + selected.value + " "
	case acModeRecur:
		newInput = input[:m.quickAddAcTriggerPos] + "~" + selected.value + " "
	case acModeSection:
		newInput = input[:m.quickAddAcTriggerPos] + "/" + selected.value + " "
	case acModeStatus:
		newInput = input[:m.quickAddAcTriggerPos] + "$" + selected.value + " "
	}

	m.quickAddInput.SetValue(newInput)
	m.quickAddInput.SetCursor(len(newInput))

	// Reset autocomplete state
	m.quickAddAcMode = acModeNone
	m.quickAddSuggestions = nil
	m.quickAddAcCursor = 0
	m.quickAddAcViewportTop = 0
}

// createQuickAddTask parses the input and creates a new task.
func (m *KanbanViewModel) createQuickAddTask() tea.Cmd {
	return func() tea.Msg {
		input := strings.TrimSpace(m.quickAddInput.Value())
		if input == "" {
			m.quickAddError = "Task title cannot be empty"
			return nil
		}

		// Parse the quick capture input (no cross-project support in Kanban)
		result, err := parseQuickCapture(input, m.Project, nil)
		if err != nil {
			m.quickAddError = err.Error()
			return nil
		}

		// Determine target section
		targetSection := result.section
		if targetSection == "" {
			// Default to current column's section
			targetSection = m.CurrentSection()
		}

		// Ensure target section exists
		if targetSection != "" && !m.Project.HasSection(targetSection) {
			m.Project.EnsureSection(targetSection)
		}

		// Create task
		task := &models.Task{
			ID:         uuid.New().String(),
			ShortID:    m.Project.GetNextShortID(),
			Type:       result.taskType,
			Title:      result.title,
			Status:     models.StatusTodo,
			Priority:   result.priority,
			Section:    targetSection,
			ParentID:   result.parentID,
			DueDate:    result.dueDate,
			Recurrence: result.recurrence,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		for _, tag := range result.tags {
			task.AddTag(tag)
		}

		// Apply status transition using Task methods (handles auto-triage)
		switch result.status {
		case models.StatusInProgress:
			task.Start()
			if result.activate {
				task.Activate()
			}
		case models.StatusBlocked:
			task.Block()
		case models.StatusDone:
			task.Complete()
		case models.StatusCanceled:
			task.Cancel()
		case models.StatusReview:
			task.Review()
		}

		// Capture previous task states for workflow rule evaluation
		prevStates := captureTaskStates(m.Project.Tasks)

		// Add to project at the beginning so it's prominently visible
		m.Project.Tasks = append([]*models.Task{task}, m.Project.Tasks...)
		m.Project.BuildTaskTree()
		m.Project.ReorderRootTasksByCompletion()

		// Save with workflow rules
		saveMsg := m.saveWithWorkflow(prevStates, func() tea.Msg {
			return KanbanQuickAddCreatedMsg{Task: task}
		})

		return saveMsg()
	}
}

// renderQuickAddInput renders the quick add input bar at the bottom.
func (m *KanbanViewModel) renderQuickAddInput() string {
	var b strings.Builder

	// Divider line
	b.WriteString(renderHorizontalDivider(m.Width()))
	b.WriteString("\n")

	// Input line with label
	b.WriteString("Add: ")
	b.WriteString(m.quickAddInput.View())
	b.WriteString("\n")

	// Show autocomplete suggestions if active (scrollable window)
	if m.quickAddAcMode != acModeNone && len(m.quickAddSuggestions) > 0 {
		// Show scroll indicator if there are items above
		if m.quickAddAcViewportTop > 0 {
			b.WriteString(MutedStyle.Render("  ↑ " + fmt.Sprintf("%d more", m.quickAddAcViewportTop)))
			b.WriteString("\n")
		}

		// Calculate visible range
		visibleEnd := m.quickAddAcViewportTop + acVisibleCount
		if visibleEnd > len(m.quickAddSuggestions) {
			visibleEnd = len(m.quickAddSuggestions)
		}

		// Render visible suggestions
		for i := m.quickAddAcViewportTop; i < visibleEnd; i++ {
			s := m.quickAddSuggestions[i]
			if i == m.quickAddAcCursor {
				b.WriteString(SelectedStyle.Render("> " + s.display))
			} else {
				b.WriteString(MutedStyle.Render("  " + s.display))
			}
			b.WriteString("\n")
		}

		// Show scroll indicator if there are items below
		remaining := len(m.quickAddSuggestions) - visibleEnd
		if remaining > 0 {
			b.WriteString(MutedStyle.Render("  ↓ " + fmt.Sprintf("%d more", remaining)))
			b.WriteString("\n")
		}
	}

	// Show preview or error
	if m.quickAddError != "" {
		b.WriteString(ValidationErrorStyle.Render(m.quickAddError))
		b.WriteString("\n")
	} else if m.quickAddPreview != nil && m.quickAddPreview.title != "" {
		preview := m.renderQuickAddPreview()
		b.WriteString(preview)
		b.WriteString("\n")
	}

	// Help hints
	if m.quickAddAcMode != acModeNone {
		b.WriteString(MutedStyle.Render("Tab: accept · ↑↓: navigate · Esc: dismiss"))
	} else {
		b.WriteString(MutedStyle.Render("Enter: create · Esc: cancel · ^type !N #tag @date /section"))
	}

	return b.String()
}

// renderQuickAddPreview formats the parsed preview for display.
func (m *KanbanViewModel) renderQuickAddPreview() string {
	if m.quickAddPreview == nil || m.quickAddPreview.title == "" {
		return ""
	}

	var parts []string
	p := m.quickAddPreview

	// Type icon and title
	typeStyle := GetTypeStyle(string(p.taskType))
	parts = append(parts, typeStyle.Render(p.taskType.Icon())+" "+p.title)

	// Priority (only show if not default)
	if p.priority != 3 {
		parts = append(parts, fmt.Sprintf("!%d", p.priority))
	}

	// Tags
	if len(p.tags) > 0 {
		tagStrs := make([]string, len(p.tags))
		for i, t := range p.tags {
			tagStrs[i] = "#" + t
		}
		parts = append(parts, strings.Join(tagStrs, " "))
	}

	// Due date
	if p.dueDate != nil {
		parts = append(parts, "@"+p.dueDate.Format("Jan 2"))
	}

	// Recurrence
	if p.recurrence != nil {
		parts = append(parts, "~"+p.recurrence.String())
	}

	// Section (only show if explicitly set)
	if p.section != "" {
		parts = append(parts, "/"+p.section)
	}

	// Status (only show if not default todo)
	if p.status != "" && p.status != models.StatusTodo {
		parts = append(parts, p.status.Icon()+" $"+string(p.status))
	}

	previewText := strings.Join(parts, " · ")

	// Wrap text if it exceeds available width
	width := m.Width()
	if width > 0 {
		previewText = wrapPreviewText(previewText, width)
	}

	return previewText
}

// getQuickAddHeight returns the height needed for the quick add bar.
func (m *KanbanViewModel) getQuickAddHeight() int {
	// Minimum: divider (1) + input (1) + help (1) = 3
	height := 3

	// Add height for preview/error line
	if m.quickAddError != "" || (m.quickAddPreview != nil && m.quickAddPreview.title != "") {
		height++
	}

	// Add height for autocomplete suggestions
	if m.quickAddAcMode != acModeNone && len(m.quickAddSuggestions) > 0 {
		visibleCount := len(m.quickAddSuggestions)
		if visibleCount > acVisibleCount {
			visibleCount = acVisibleCount
		}
		height += visibleCount
		// Add scroll indicators
		if m.quickAddAcViewportTop > 0 {
			height++
		}
		if m.quickAddAcViewportTop+acVisibleCount < len(m.quickAddSuggestions) {
			height++
		}
	}

	return height
}

// Autocomplete suggestion getters for quick add mode

func (m *KanbanViewModel) getQuickAddTagSuggestions(partial string) []suggestion {
	type scoredTag struct {
		tag   string
		score int
	}

	var scored []scoredTag
	for _, tag := range m.quickAddTagCache {
		score := fuzzy.Score(tag, partial)
		if score > 0 {
			scored = append(scored, scoredTag{tag: tag, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	suggestions := make([]suggestion, 0, len(scored))
	for _, s := range scored {
		suggestions = append(suggestions, suggestion{
			display: "#" + s.tag,
			value:   s.tag,
		})
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddTypeSuggestions(partial string) []suggestion {
	types := []struct {
		name string
		icon string
	}{
		{"epic", "◆"},
		{"story", "◇"},
		{"task", "□"},
		{"bug", "⚠"},
		{"idea", "💡"},
		{"note", "📝"},
	}

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, t := range types {
		if partial == "" || strings.Contains(t.name, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("^%s %s", t.name, t.icon),
				value:   t.name,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddPrioritySuggestions(partial string) []suggestion {
	priorities := []struct {
		value string
		desc  string
	}{
		{"1", "Urgent"},
		{"2", "High"},
		{"3", "Normal"},
		{"4", "Low"},
		{"5", "Lowest"},
	}

	var suggestions []suggestion
	for _, p := range priorities {
		if partial == "" || strings.HasPrefix(p.value, partial) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("!%s (%s)", p.value, p.desc),
				value:   p.value,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddDateSuggestions(partial string) []suggestion {
	options := []struct {
		value string
		desc  string
	}{
		{"today", "today"},
		{"tomorrow", "tomorrow"},
		{"+1d", "in 1 day"},
		{"+3d", "in 3 days"},
		{"+1w", "in 1 week"},
		{"+2w", "in 2 weeks"},
		{"+1m", "in 1 month"},
		{"monday", "next Monday"},
		{"friday", "next Friday"},
	}

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.value, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("@%s (%s)", o.value, o.desc),
				value:   o.value,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddRecurrenceSuggestions(partial string) []suggestion {
	options := []struct {
		value string
		desc  string
	}{
		{"daily", "every day"},
		{"weekly", "every week"},
		{"biweekly", "every 2 weeks"},
		{"monthly", "every month"},
		{"2d", "every 2 days"},
		{"3d", "every 3 days"},
		{"2w", "every 2 weeks"},
	}

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.value, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("~%s (%s)", o.value, o.desc),
				value:   o.value,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddSectionSuggestions(partial string) []suggestion {
	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, section := range m.quickAddSectionCache {
		if partial == "" || strings.Contains(strings.ToLower(section), partialLower) {
			suggestions = append(suggestions, suggestion{
				display: "/" + section,
				value:   section,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddStatusSuggestions(partial string) []suggestion {
	statuses := []struct {
		name string
		icon string
		desc string
	}{
		{"start", "◐", "in progress + activate"},
		{"blocked", "◈", "blocked"},
		{"done", "●", "completed"},
		{"canceled", "✕", "canceled"},
	}

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, s := range statuses {
		if partial == "" || strings.Contains(s.name, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("$%s %s (%s)", s.name, s.icon, s.desc),
				value:   s.name,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddParentSuggestions(partial string) []suggestion {
	type scoredParent struct {
		task  *models.Task
		score int
	}

	var scored []scoredParent
	for _, task := range m.quickAddParentCache {
		searchText := fmt.Sprintf("#%d %s %s", task.ShortID, task.Type.HeadingPrefix(), task.Title)
		score := fuzzy.Score(searchText, partial)
		if score > 0 {
			scored = append(scored, scoredParent{task: task, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	suggestions := make([]suggestion, 0, len(scored))
	for _, s := range scored {
		task := s.task
		title := task.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		suggestions = append(suggestions, suggestion{
			display: fmt.Sprintf("%s #%d %s", task.Type.Icon(), task.ShortID, title),
			value:   strconv.Itoa(task.ShortID),
		})
	}
	return suggestions
}
