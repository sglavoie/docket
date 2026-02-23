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

// kanbanQuickAddTriggers defines the trigger characters for kanban quick add autocomplete.
var kanbanQuickAddTriggers = []ACTrigger{
	{'#', acModeTag},
	{'>', acModeParent},
	{'^', acModeType},
	{'!', acModePriority},
	{'@', acModeDate},
	{'~', acModeRecur},
	{'/', acModeSection},
	{'$', acModeStatus},
}

// handleQuickAddInput processes keyboard input while in quick add mode.
func (m *KanbanViewModel) handleQuickAddInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle autocomplete navigation when active
	if m.quickAddAc.HandleKey(key, func() {
		m.acceptQuickAddSuggestion()
		m.updateQuickAddPreview()
	}) {
		return m, nil
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
		m.quickAddAc.Clear()
		return m, nil

	case "enter":
		// Create task if not in autocomplete mode
		if !m.quickAddAc.IsActive() {
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
	mode, pos, partial := DetectTrigger(m.quickAddInput.Value(), kanbanQuickAddTriggers)
	if mode == acModeNone {
		m.quickAddAc.Clear()
		return
	}

	var items []ACSuggestion
	switch mode {
	case acModeTag:
		items = m.getQuickAddTagSuggestions(partial)
	case acModeParent:
		items = m.getQuickAddParentSuggestions(partial)
	case acModeType:
		items = m.getQuickAddTypeSuggestions(partial)
	case acModePriority:
		items = m.getQuickAddPrioritySuggestions(partial)
	case acModeDate:
		items = m.getQuickAddDateSuggestions(partial)
	case acModeRecur:
		items = m.getQuickAddRecurrenceSuggestions(partial)
	case acModeSection:
		items = m.getQuickAddSectionSuggestions(partial)
	case acModeStatus:
		items = m.getQuickAddStatusSuggestions(partial)
	}
	m.quickAddAc.UpdateSuggestions(mode, pos, items)
}

// acceptQuickAddSuggestion inserts the selected suggestion into the input.
func (m *KanbanViewModel) acceptQuickAddSuggestion() {
	if !m.quickAddAc.IsActive() {
		return
	}

	triggerChar := TriggerCharForMode(m.quickAddAc.Mode, kanbanQuickAddTriggers)
	newInput := m.quickAddAc.BuildAcceptedInput(m.quickAddInput.Value(), triggerChar)

	m.quickAddInput.SetValue(newInput)
	m.quickAddInput.SetCursor(len(newInput))

	m.quickAddAc.Clear()
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

		// Determine target section and custom section
		targetSection := result.section
		var customSection string
		if targetSection == "" {
			// Default: in section mode, use current column's section; otherwise use Active
			if IsSectionGroupBy(m.groupBy) {
				targetSection = m.CurrentSection()
			}
			// In non-section modes, leave empty (Active section)
		} else if !models.IsGTDSection(targetSection) {
			// Custom section: route to Inbox, store as CustomSection
			customSection = targetSection
			targetSection = models.SectionInbox
		}

		// Ensure Inbox exists (gets red color)
		if !m.Project.HasSection(models.SectionInbox) {
			m.Project.EnsureSection(models.SectionInbox)
			m.Project.SetSectionColor(models.SectionInbox, "red")
		}
		// Ensure target section exists
		if targetSection != "" && targetSection != models.SectionInbox && !m.Project.HasSection(targetSection) {
			m.Project.EnsureSection(targetSection)
		}
		// Ensure custom section exists
		if customSection != "" && !m.Project.HasSection(customSection) {
			m.Project.EnsureSection(customSection)
		}

		// Create task
		task := &models.Task{
			ID:            uuid.New().String(),
			ShortID:       m.Project.GetNextShortID(),
			Type:          result.taskType,
			Title:         result.title,
			Status:        models.StatusTodo,
			Priority:      result.priority,
			Section:       targetSection,
			CustomSection: customSection,
			ParentID:      result.parentID,
			DueDate:       result.dueDate,
			Recurrence:    result.recurrence,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
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

	// Show autocomplete suggestions if active
	if m.quickAddAc.IsActive() {
		b.WriteString(m.quickAddAc.RenderDropdown("  "))
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
	if m.quickAddAc.IsActive() {
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
		if !models.IsGTDSection(p.section) {
			parts = append(parts, "/"+p.section+" → Inbox")
		} else {
			parts = append(parts, "/"+p.section)
		}
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
	if m.quickAddAc.IsActive() {
		visibleCount := len(m.quickAddAc.Items)
		if visibleCount > acVisibleCount {
			visibleCount = acVisibleCount
		}
		height += visibleCount
		// Add scroll indicators
		if m.quickAddAc.ViewportTop > 0 {
			height++
		}
		if m.quickAddAc.ViewportTop+acVisibleCount < len(m.quickAddAc.Items) {
			height++
		}
	}

	return height
}

// Autocomplete suggestion getters for quick add mode

func (m *KanbanViewModel) getQuickAddTagSuggestions(partial string) []ACSuggestion {
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

	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		suggestions = append(suggestions, ACSuggestion{
			Display: "#" + s.tag,
			Value:   s.tag,
		})
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddTypeSuggestions(partial string) []ACSuggestion {
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

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, t := range types {
		if partial == "" || strings.Contains(t.name, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("^%s %s", t.name, t.icon),
				Value:   t.name,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddPrioritySuggestions(partial string) []ACSuggestion {
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

	var suggestions []ACSuggestion
	for _, p := range priorities {
		if partial == "" || strings.HasPrefix(p.value, partial) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("!%s (%s)", p.value, p.desc),
				Value:   p.value,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddDateSuggestions(partial string) []ACSuggestion {
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

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.value, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("@%s (%s)", o.value, o.desc),
				Value:   o.value,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddRecurrenceSuggestions(partial string) []ACSuggestion {
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

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.value, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("~%s (%s)", o.value, o.desc),
				Value:   o.value,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddSectionSuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, section := range m.quickAddSectionCache {
		if partial == "" || strings.Contains(strings.ToLower(section), partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: "/" + section,
				Value:   section,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddStatusSuggestions(partial string) []ACSuggestion {
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

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, s := range statuses {
		if partial == "" || strings.Contains(s.name, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("$%s %s (%s)", s.name, s.icon, s.desc),
				Value:   s.name,
			})
		}
	}
	return suggestions
}

func (m *KanbanViewModel) getQuickAddParentSuggestions(partial string) []ACSuggestion {
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

	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		task := s.task
		title := task.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		suggestions = append(suggestions, ACSuggestion{
			Display: fmt.Sprintf("%s #%d %s", task.Type.Icon(), task.ShortID, title),
			Value:   strconv.Itoa(task.ShortID),
		})
	}
	return suggestions
}
