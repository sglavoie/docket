package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Autocomplete mode constants
const (
	acModeNone     = iota // No autocomplete active
	acModeTag             // Completing a #tag
	acModeParent          // Completing a >parent
	acModeType            // Completing a ^type
	acModePriority        // Completing a !priority
	acModeDate            // Completing a @date
	acModeRecur           // Completing a ~recurrence
	acModeSection         // Completing a /section
	acModeStatus          // Completing a $status
)

// suggestion represents an autocomplete suggestion
type suggestion struct {
	display string // Text shown in dropdown
	value   string // Text to insert
}

// QuickCaptureModel handles rapid task creation with inline metadata parsing.
type QuickCaptureModel struct {
	project *models.Project
	input   textarea.Model
	err     error
	width   int
	height  int

	// Autocomplete state
	acMode       int            // Current autocomplete mode
	suggestions  []suggestion   // Current suggestions
	acCursor     int            // Selected suggestion index
	acTriggerPos int            // Position of trigger character in input
	tagCache     []string       // Cached project tags
	parentCache  []*models.Task // Cached valid parent tasks
	sectionCache []string       // Cached project sections

	// Live preview
	preview    *quickCaptureResult // Parsed preview (may be partial)
	previewErr string              // Preview validation error
}

// quickCaptureResult holds parsed metadata from quick capture input.
type quickCaptureResult struct {
	title      string
	taskType   models.TaskType
	priority   int
	tags       []string
	dueDate    *time.Time
	parentID   *string
	recurrence *models.RecurrencePattern
	section    string            // Target section (empty = Inbox)
	status     models.TaskStatus // Initial status (empty = todo)
	activate   bool              // Whether to activate task (for $start)
}

// NewQuickCaptureModel creates a new quick capture model.
func NewQuickCaptureModel(project *models.Project) *QuickCaptureModel {
	input := textarea.New()
	input.Placeholder = "^type Task title !priority #tag @due ~recur >parent /section $status"
	input.CharLimit = 200
	input.SetHeight(3)
	input.ShowLineNumbers = false
	input.SetPromptFunc(0, func(lineIdx int) string { return "" })
	input.Focus()

	m := &QuickCaptureModel{
		project:     project,
		input:       input,
		width:       80, // sensible default until SetSize is called
		acMode:      acModeNone,
		suggestions: make([]suggestion, 0),
	}

	// Initialize caches
	m.initCaches()

	return m
}

// initCaches builds tag and parent caches for autocomplete.
func (m *QuickCaptureModel) initCaches() {
	// Cache all tags from the project
	tagSet := make(map[string]bool)
	for _, task := range m.project.Tasks {
		for _, tag := range task.Tags {
			tagSet[tag] = true
		}
	}
	m.tagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		m.tagCache = append(m.tagCache, tag)
	}
	sort.Strings(m.tagCache)

	// Cache potential parent tasks (non-completed, can have children)
	m.parentCache = make([]*models.Task, 0)
	for _, task := range m.project.Tasks {
		if !task.Status.IsComplete() && task.Type.CanHaveChildren() {
			m.parentCache = append(m.parentCache, task)
		}
	}

	// Cache sections
	m.sectionCache = m.project.GetSections()
}

// SetSize sets the view dimensions.
func (m *QuickCaptureModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Use same resize logic as task_creator
	formFactory.ResizeTextarea(&m.input, width)
}

func (m *QuickCaptureModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *QuickCaptureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Handle autocomplete navigation when active
		if m.acMode != acModeNone && len(m.suggestions) > 0 {
			switch key {
			case "tab", "enter":
				m.acceptSuggestion()
				m.updatePreview()
				return m, nil
			case "esc":
				// Dismiss autocomplete
				m.acMode = acModeNone
				m.suggestions = nil
				m.acCursor = 0
				return m, nil
			case "up", "ctrl+p":
				if m.acCursor > 0 {
					m.acCursor--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.acCursor < len(m.suggestions)-1 {
					m.acCursor++
				}
				return m, nil
			}
		}

		// Regular key handling
		switch key {
		case "esc":
			return m, func() tea.Msg { return CloseQuickCaptureMsg{} }

		case "enter":
			// Only create task if not in autocomplete mode
			if m.acMode == acModeNone {
				return m, m.createTask()
			}
		}
	}

	// Update textarea
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Update preview and detect autocomplete triggers
	m.updatePreview()
	mode, pos, partial := m.detectAutocompleteTrigger()
	if mode != acModeNone {
		m.acMode = mode
		m.acTriggerPos = pos
		m.acCursor = 0
		switch mode {
		case acModeTag:
			m.suggestions = m.getTagSuggestions(partial)
		case acModeParent:
			m.suggestions = m.getParentSuggestions(partial)
		case acModeType:
			m.suggestions = m.getTypeSuggestions(partial)
		case acModePriority:
			m.suggestions = m.getPrioritySuggestions(partial)
		case acModeDate:
			m.suggestions = m.getDateSuggestions(partial)
		case acModeRecur:
			m.suggestions = m.getRecurrenceSuggestions(partial)
		case acModeSection:
			m.suggestions = m.getSectionSuggestions(partial)
		case acModeStatus:
			m.suggestions = m.getStatusSuggestions(partial)
		}
		// If no suggestions, exit autocomplete mode
		if len(m.suggestions) == 0 {
			m.acMode = acModeNone
		}
	} else {
		m.acMode = acModeNone
		m.suggestions = nil
	}

	return m, cmd
}

func (m *QuickCaptureModel) View() string {
	var b strings.Builder
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Show autocomplete suggestions if active
	if m.acMode != acModeNone && len(m.suggestions) > 0 {
		b.WriteString("\n")
		for i, s := range m.suggestions {
			if i == m.acCursor {
				b.WriteString(SelectedStyle.Render("> " + s.display))
			} else {
				b.WriteString(MutedStyle.Render("  " + s.display))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Show live preview or error
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	} else if preview := m.renderPreview(); preview != "" {
		b.WriteString(preview)
		b.WriteString("\n\n")
	}

	// Help text
	b.WriteString(MutedStyle.Render("^type · !N · #tag · @date · ~recur · >parent · /section · $status"))
	b.WriteString("\n")
	if m.acMode != acModeNone {
		b.WriteString(MutedStyle.Render("Tab: accept · ↑↓: navigate · Esc: dismiss"))
	} else {
		b.WriteString(MutedStyle.Render("Enter: create · Esc: cancel"))
	}

	// Same card width calculation as task_creator
	cardWidth := m.width - 4
	if cardWidth > 80 {
		cardWidth = 80
	}
	config := OverlayCardConfig{
		Title:        "Quick Capture",
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, config)
}

// updatePreview updates the live preview state.
func (m *QuickCaptureModel) updatePreview() {
	input := strings.ReplaceAll(m.input.Value(), "\n", " ")
	input = strings.TrimSpace(input)
	if input == "" {
		m.preview = nil
		m.previewErr = ""
		return
	}
	m.preview, m.previewErr = parseQuickCaptureForPreview(input, m.project)
}

// renderPreview formats the parsed preview for display.
func (m *QuickCaptureModel) renderPreview() string {
	if m.previewErr != "" {
		return ValidationErrorStyle.Render(m.previewErr)
	}
	if m.preview == nil || m.preview.title == "" {
		return ""
	}

	var parts []string

	// Type and title
	typeStyle := GetTypeStyle(string(m.preview.taskType))
	parts = append(parts, typeStyle.Render(m.preview.taskType.Icon())+" "+m.preview.title)

	// Priority (only show if not default)
	if m.preview.priority != 3 {
		parts = append(parts, fmt.Sprintf("!%d", m.preview.priority))
	}

	// Tags
	if len(m.preview.tags) > 0 {
		tagStrs := make([]string, len(m.preview.tags))
		for i, t := range m.preview.tags {
			tagStrs[i] = "#" + t
		}
		parts = append(parts, strings.Join(tagStrs, " "))
	}

	// Due date
	if m.preview.dueDate != nil {
		parts = append(parts, "@"+m.preview.dueDate.Format("Jan 2"))
	}

	// Recurrence
	if m.preview.recurrence != nil {
		parts = append(parts, "~"+m.preview.recurrence.String())
	}

	// Section (only show if explicitly set)
	if m.preview.section != "" {
		parts = append(parts, "/"+m.preview.section)
	}

	// Status (only show if not default todo)
	if m.preview.status != "" && m.preview.status != models.StatusTodo {
		parts = append(parts, m.preview.status.Icon()+" $"+string(m.preview.status))
	}

	return MutedStyle.Render(strings.Join(parts, " · "))
}

func (m *QuickCaptureModel) createTask() tea.Cmd {
	return func() tea.Msg {
		// Replace newlines with spaces since this is a single task title
		input := strings.ReplaceAll(m.input.Value(), "\n", " ")
		input = strings.TrimSpace(input)
		if input == "" {
			m.err = fmt.Errorf("task title cannot be empty")
			return nil
		}

		result, err := parseQuickCapture(input, m.project)
		if err != nil {
			m.err = err
			return nil
		}

		// Determine target section
		targetSection := result.section
		if targetSection == "" {
			targetSection = models.SectionInbox
		}

		// Ensure target section exists (Inbox gets red color)
		if !m.project.HasSection(targetSection) {
			m.project.EnsureSection(targetSection)
			if targetSection == models.SectionInbox {
				m.project.SetSectionColor(models.SectionInbox, "red")
			}
		}

		// Create task
		task := &models.Task{
			ID:         uuid.New().String(),
			ShortID:    m.project.GetNextShortID(),
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
		case models.StatusCancelled:
			task.Cancel()
		}

		// Add to project at the beginning so it's prominently visible
		m.project.Tasks = append([]*models.Task{task}, m.project.Tasks...)
		m.project.BuildTaskTree()
		m.project.ReorderRootTasksByCompletion()

		// Backup and save
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reload to rebuild tree properly
		storage.LoadProject(m.project)

		return TaskCreatedMsg{Task: task}
	}
}
