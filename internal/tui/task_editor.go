package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// TaskEditorModel handles editing an existing task via TUI.
type TaskEditorModel struct {
	project       *models.Project
	task          *models.Task
	config        *config.Config
	configManager *config.Manager

	// Original values for change detection
	originalTitle      string
	originalTags       string
	originalType       models.TaskType
	originalParentID   *string
	originalPriority   int
	originalSection    string
	originalDueDate    *time.Time
	originalPinned     bool
	originalActive     bool
	originalRecurrence *models.RecurrencePattern

	// Form fields
	titleInput textarea.Model
	tags       []string

	// Selectable fields
	taskType         models.TaskType
	typeCursor       int
	types            []models.TaskType
	parentID         *string
	parentCursor     int
	validParents     []*models.Task
	priority         int
	section          string
	sectionCursor    int
	sections         []string
	dueDate          *time.Time
	dueDateInput     string
	pinned           bool
	active           bool
	flagFocus        int // 0=pinned, 1=active (within the flags field)
	recurrence       *models.RecurrencePattern
	recurrenceCursor int
	recurrenceOpts   []string // Display labels for recurrence options

	// Form state
	focusIndex int // 0=title, 1=type, 2=parent, 3=priority, 4=section, 5=due date, 6=flags, 7=recurrence
	width      int
	height     int
	err        error
	hasChanges bool
}

// NewTaskEditorModel creates a new task editor.
func NewTaskEditorModel(project *models.Project, task *models.Task, cfg *config.Config, manager *config.Manager) *TaskEditorModel {
	factory := DefaultFormFieldFactory()

	titleInput := factory.NewTitleInputWithValue(task.Title)
	titleInput.Focus()
	titleInput.SetPromptFunc(0, func(lineIdx int) string { return "" })

	// Copy tags slice
	tags := append([]string{}, task.Tags...)

	types := []models.TaskType{
		models.TaskTypeTask,
		models.TaskTypeStory,
		models.TaskTypeEpic,
		models.TaskTypeBug,
		models.TaskTypeIdea,
		models.TaskTypeNote,
	}

	typeCursor := 0
	for i, t := range types {
		if t == task.Type {
			typeCursor = i
			break
		}
	}

	// Get available sections from project
	sections := project.GetSections()
	sectionCursor := 0
	for i, s := range sections {
		if s == task.Section {
			sectionCursor = i
			break
		}
	}

	// Copy due date if present
	var dueDate *time.Time
	var dueDateInput string
	if task.DueDate != nil {
		dueCopy := *task.DueDate
		dueDate = &dueCopy
		dueDateInput = task.DueDate.Format("2006-01-02")
	}

	// Copy original due date for change detection
	var originalDueDate *time.Time
	if task.DueDate != nil {
		dueCopy := *task.DueDate
		originalDueDate = &dueCopy
	}

	// Copy original recurrence for change detection
	var originalRecurrence *models.RecurrencePattern
	var recurrence *models.RecurrencePattern
	if task.Recurrence != nil {
		origCopy := *task.Recurrence
		originalRecurrence = &origCopy
		recurCopy := *task.Recurrence
		recurrence = &recurCopy
	}

	// Recurrence options: None, daily, weekly, biweekly, monthly, yearly, custom
	recurrenceOpts := []string{"None", "Daily", "Weekly", "Biweekly", "Monthly", "Yearly", "Custom..."}
	recurrenceCursor := 0
	if task.Recurrence != nil {
		// Find matching option
		switch {
		case task.Recurrence.Unit == models.RecurrenceDaily && task.Recurrence.Interval == 1 && len(task.Recurrence.Weekdays) == 0:
			recurrenceCursor = 1
		case task.Recurrence.Unit == models.RecurrenceWeekly && task.Recurrence.Interval == 1 && len(task.Recurrence.Weekdays) == 0:
			recurrenceCursor = 2
		case task.Recurrence.Unit == models.RecurrenceWeekly && task.Recurrence.Interval == 2 && len(task.Recurrence.Weekdays) == 0:
			recurrenceCursor = 3
		case task.Recurrence.Unit == models.RecurrenceMonthly && task.Recurrence.Interval == 1:
			recurrenceCursor = 4
		case task.Recurrence.Unit == models.RecurrenceYearly && task.Recurrence.Interval == 1:
			recurrenceCursor = 5
		default:
			// Custom pattern - show as "Custom..."
			recurrenceCursor = 6
		}
	}

	m := &TaskEditorModel{
		project:            project,
		task:               task,
		config:             cfg,
		configManager:      manager,
		originalTitle:      task.Title,
		originalTags:       strings.Join(task.Tags, ", "),
		originalType:       task.Type,
		originalParentID:   task.ParentID,
		originalPriority:   task.Priority,
		originalSection:    task.Section,
		originalDueDate:    originalDueDate,
		originalPinned:     task.Pinned,
		originalActive:     task.Active,
		titleInput:         titleInput,
		tags:               tags,
		taskType:           task.Type,
		typeCursor:         typeCursor,
		types:              types,
		parentID:           task.ParentID,
		parentCursor:       0,
		priority:           task.Priority,
		section:            task.Section,
		sectionCursor:      sectionCursor,
		sections:           sections,
		dueDate:            dueDate,
		dueDateInput:       dueDateInput,
		pinned:             task.Pinned,
		active:             task.Active,
		flagFocus:          0,
		recurrence:         recurrence,
		recurrenceCursor:   recurrenceCursor,
		recurrenceOpts:     recurrenceOpts,
		originalRecurrence: originalRecurrence,
		focusIndex:         0,
		width:              80,
		height:             24,
	}

	m.updateValidParents()

	return m
}

// updateValidParents filters tasks that can be parents of the current type.
func (m *TaskEditorModel) updateValidParents() {
	m.validParents = []*models.Task{}

	for _, task := range m.project.Tasks {
		// Can't be parent of itself or its descendants
		if task.ID == m.task.ID {
			continue
		}
		if m.isDescendant(task, m.task) {
			continue
		}
		if task.CanAddChild(m.taskType) {
			m.validParents = append(m.validParents, task)
		}
	}

	// Find current parent cursor position
	m.parentCursor = 0
	if m.parentID != nil {
		for i, p := range m.validParents {
			if p.ID == *m.parentID {
				m.parentCursor = i + 1 // +1 because 0 is "None"
				break
			}
		}
		// If parent no longer valid, clear it
		if m.parentCursor == 0 {
			m.parentID = nil
		}
	}
}

// isDescendant checks if candidate is a descendant of task.
func (m *TaskEditorModel) isDescendant(candidate, task *models.Task) bool {
	for _, child := range task.Children {
		if child.ID == candidate.ID {
			return true
		}
		if m.isDescendant(candidate, child) {
			return true
		}
	}
	return false
}

// checkChanges detects if any field has been modified.
func (m *TaskEditorModel) checkChanges() bool {
	// Normalize title by replacing newlines with spaces (same as saveTask)
	currentTitle := strings.TrimSpace(strings.ReplaceAll(m.titleInput.Value(), "\n", " "))
	if currentTitle != m.originalTitle {
		return true
	}
	if strings.Join(m.tags, ", ") != m.originalTags {
		return true
	}
	if m.taskType != m.originalType {
		return true
	}
	if m.priority != m.originalPriority {
		return true
	}
	// Compare parent IDs
	if m.parentID == nil && m.originalParentID != nil {
		return true
	}
	if m.parentID != nil && m.originalParentID == nil {
		return true
	}
	if m.parentID != nil && m.originalParentID != nil && *m.parentID != *m.originalParentID {
		return true
	}
	// Check section
	if m.section != m.originalSection {
		return true
	}
	// Check due date
	if m.dueDate == nil && m.originalDueDate != nil {
		return true
	}
	if m.dueDate != nil && m.originalDueDate == nil {
		return true
	}
	if m.dueDate != nil && m.originalDueDate != nil && !m.dueDate.Equal(*m.originalDueDate) {
		return true
	}
	// Check flags
	if m.pinned != m.originalPinned {
		return true
	}
	if m.active != m.originalActive {
		return true
	}
	// Check recurrence
	if !recurrenceEqual(m.recurrence, m.originalRecurrence) {
		return true
	}
	return false
}

// recurrenceEqual compares two recurrence patterns for equality.
func recurrenceEqual(a, b *models.RecurrencePattern) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Interval != b.Interval || a.Unit != b.Unit {
		return false
	}
	// Compare weekdays
	if len(a.Weekdays) != len(b.Weekdays) {
		return false
	}
	for i, wd := range a.Weekdays {
		if wd != b.Weekdays[i] {
			return false
		}
	}
	return true
}

// updateRecurrenceFromCursor updates the recurrence pattern based on cursor position.
func (m *TaskEditorModel) updateRecurrenceFromCursor() {
	switch m.recurrenceCursor {
	case 0: // None
		m.recurrence = nil
	case 1: // Daily
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	case 2: // Weekly
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly}
	case 3: // Biweekly
		m.recurrence = &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly}
	case 4: // Monthly
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceMonthly}
	case 5: // Yearly
		m.recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceYearly}
	}
}

// parseDueDate parses the due date input string and updates m.dueDate.
func (m *TaskEditorModel) parseDueDate() error {
	value := strings.TrimSpace(m.dueDateInput)

	// Handle empty or "none" to clear due date
	if value == "" || strings.ToLower(value) == "none" || strings.ToLower(value) == "clear" {
		m.dueDate = nil
		return nil
	}

	// Parse the date using the existing ParseDate function
	date, err := ParseDate(value)
	if err != nil {
		return err
	}

	m.dueDate = &date
	return nil
}

// SetSize sets the view dimensions.
func (m *TaskEditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextarea(&m.titleInput, width)
}

func (m *TaskEditorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *TaskEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case TagSelectorDoneMsg:
		if !msg.Cancelled {
			m.tags = msg.SelectedTags
			m.hasChanges = m.checkChanges()
		}
		return m, nil

	case RecurrenceEditorDoneMsg:
		if !msg.Cancelled && msg.Pattern != nil {
			m.recurrence = msg.Pattern
			// Update cursor to "Custom..." since we have a custom pattern
			m.recurrenceCursor = 6
			m.hasChanges = m.checkChanges()
		}
		return m, nil

	case ParentSelectedMsg:
		if msg.Source == ViewTaskEditor {
			m.parentID = msg.ParentID
			if msg.ParentID == nil {
				m.parentCursor = 0
			} else {
				for i, p := range m.validParents {
					if p.ID == *msg.ParentID {
						m.parentCursor = i + 1
						break
					}
				}
			}
			m.hasChanges = m.checkChanges()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Move focus (8 fields: title, type, parent, priority, section, due date, flags, recurrence)
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 8
			} else {
				m.focusIndex = (m.focusIndex + 7) % 8
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "enter":
			// Move to next field (8 focusable fields: 0-7)
			if m.focusIndex < 7 {
				// For due date field, parse and validate on enter
				if m.focusIndex == 5 {
					if err := m.parseDueDate(); err != nil {
						m.err = err
						return m, nil
					}
					m.err = nil
				}
				m.blurCurrentField()
				m.focusIndex++
				m.focusCurrentField()
				return m, nil
			}

		case "ctrl+o":
			// Edit title and description in external editor (works from any field)
			return m, func() tea.Msg {
				return EditTaskContentMsg{Task: m.task, Project: m.project}
			}

		case "ctrl+s":
			// Save changes
			return m, m.saveTask()

		case "ctrl+t":
			// Open tag selector
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{
					Task:        m.task,
					Project:     m.project,
					CurrentTags: m.tags,
				}
			}

		case "/":
			// Open parent selector modal when on parent field
			if m.focusIndex == 2 {
				return m, func() tea.Msg {
					return OpenParentSelectorMsg{
						ValidParents: m.validParents,
						CurrentID:    m.parentID,
						AllowNone:    true,
						Source:       ViewTaskEditor,
					}
				}
			}

		case "1", "2", "3", "4", "5":
			// Direct priority selection when priority field is focused
			if m.focusIndex == 3 {
				m.priority = int(msg.String()[0] - '0')
			}
			return m, nil

		case "left", "h":
			switch m.focusIndex {
			case 1: // Type selector
				if m.typeCursor > 0 {
					m.typeCursor--
					m.taskType = m.types[m.typeCursor]
					m.updateValidParents()
				}
			case 2: // Parent selector
				if m.parentCursor > 0 {
					m.parentCursor--
					if m.parentCursor == 0 {
						m.parentID = nil
					} else {
						m.parentID = &m.validParents[m.parentCursor-1].ID
					}
				}
			case 3: // Priority
				if m.priority > 1 {
					m.priority--
				}
			case 4: // Section selector
				if m.sectionCursor > 0 {
					m.sectionCursor--
					m.section = m.sections[m.sectionCursor]
				}
			case 6: // Flags - switch between pinned and active
				if m.flagFocus > 0 {
					m.flagFocus--
				}
			case 7: // Recurrence selector
				if m.recurrenceCursor > 0 {
					m.recurrenceCursor--
					m.updateRecurrenceFromCursor()
				}
			}

		case "right", "l":
			switch m.focusIndex {
			case 1: // Type selector
				if m.typeCursor < len(m.types)-1 {
					m.typeCursor++
					m.taskType = m.types[m.typeCursor]
					m.updateValidParents()
				}
			case 2: // Parent selector
				if m.parentCursor < len(m.validParents) {
					m.parentCursor++
					if m.parentCursor == 0 {
						m.parentID = nil
					} else {
						m.parentID = &m.validParents[m.parentCursor-1].ID
					}
				}
			case 3: // Priority
				if m.priority < 5 {
					m.priority++
				}
			case 4: // Section selector
				if m.sectionCursor < len(m.sections)-1 {
					m.sectionCursor++
					m.section = m.sections[m.sectionCursor]
				}
			case 6: // Flags - switch between pinned and active
				if m.flagFocus < 1 {
					m.flagFocus++
				}
			case 7: // Recurrence selector
				if m.recurrenceCursor < len(m.recurrenceOpts)-1 {
					m.recurrenceCursor++
					if m.recurrenceCursor == 6 {
						// "Custom..." selected - open the editor
						return m, func() tea.Msg {
							return ShowRecurrenceEditorMsg{
								CurrentPattern: m.recurrence,
								DueDate:        m.dueDate,
							}
						}
					}
					m.updateRecurrenceFromCursor()
				}
			}

		case " ":
			// Toggle flags with space
			if m.focusIndex == 6 {
				if m.flagFocus == 0 {
					m.pinned = !m.pinned
				} else {
					m.active = !m.active
				}
			}

		case "r":
			// Open recurrence editor when on recurrence field
			if m.focusIndex == 7 {
				return m, func() tea.Msg {
					return ShowRecurrenceEditorMsg{
						CurrentPattern: m.recurrence,
						DueDate:        m.dueDate,
					}
				}
			}
		}
	}

	// Update the focused input
	switch m.focusIndex {
	case 0:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case 5: // Due date text input
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			key := keyMsg.String()
			// Handle backspace
			if key == "backspace" && len(m.dueDateInput) > 0 {
				m.dueDateInput = m.dueDateInput[:len(m.dueDateInput)-1]
			} else if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
				// Printable ASCII characters
				m.dueDateInput += key
			}
		}
	}

	// Check for changes
	m.hasChanges = m.checkChanges()

	return m, cmd
}

func (m *TaskEditorModel) blurCurrentField() {
	if m.focusIndex == 0 {
		m.titleInput.Blur()
	}
}

func (m *TaskEditorModel) focusCurrentField() {
	if m.focusIndex == 0 {
		m.titleInput.Focus()
	}
}

func (m *TaskEditorModel) saveTask() tea.Cmd {
	return func() tea.Msg {
		// Replace newlines with spaces since titles should be single logical lines
		title := strings.ReplaceAll(m.titleInput.Value(), "\n", " ")
		title = strings.TrimSpace(title)
		if title == "" {
			return ErrorMsg{Err: fmt.Errorf("task title cannot be empty")}
		}

		// Parse due date before saving
		if err := m.parseDueDate(); err != nil {
			return ErrorMsg{Err: err}
		}

		// Update task fields (description is edited separately via external editor)
		now := time.Now()
		m.task.Title = title
		m.task.Type = m.taskType
		m.task.Priority = m.priority
		m.task.ParentID = m.parentID
		m.task.Section = m.section
		m.task.DueDate = m.dueDate
		m.task.UpdatedAt = now

		// Handle pinned status change
		if m.pinned != m.task.Pinned {
			m.task.Pinned = m.pinned
			if m.pinned {
				m.task.PinnedAt = &now
			} else {
				m.task.PinnedAt = nil
			}
		}

		// Handle active status change
		m.task.Active = m.active

		// Handle recurrence change
		m.task.Recurrence = m.recurrence

		// Set tags from the selector
		m.task.Tags = append([]string{}, m.tags...)

		// Update the actual task in the project
		for _, t := range m.project.Tasks {
			if t.ID == m.task.ID {
				t.Tags = m.task.Tags
				t.Title = m.task.Title
				t.Type = m.task.Type
				t.Priority = m.task.Priority
				t.ParentID = m.task.ParentID
				t.Section = m.task.Section
				t.DueDate = m.task.DueDate
				t.Pinned = m.task.Pinned
				t.PinnedAt = m.task.PinnedAt
				t.Active = m.task.Active
				t.Recurrence = m.task.Recurrence
				t.UpdatedAt = m.task.UpdatedAt
				break
			}
		}

		// Rebuild task tree
		m.project.BuildTaskTree()

		// Save project with backup for undo support
		if err := saveProjectSync(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TaskUpdatedMsg{Task: m.task}
	}
}

func (m *TaskEditorModel) View() string {
	var content strings.Builder

	// Title field
	content.WriteString(m.renderFormLabel("Title", 0))
	content.WriteString("\n")
	content.WriteString(m.titleInput.View())
	content.WriteString("\n\n")

	// Type selector row
	content.WriteString(m.renderFormLabel("Type", 1))
	content.WriteString(m.renderTypeSelector())
	content.WriteString("\n")

	// Parent selector row
	content.WriteString(m.renderFormLabel("Parent", 2))
	content.WriteString(m.renderParentSelector())
	content.WriteString("\n")

	// Priority selector row
	content.WriteString(m.renderFormLabel("Priority", 3))
	content.WriteString(m.renderPrioritySelector())
	content.WriteString("\n")

	// Section selector row
	content.WriteString(m.renderFormLabel("Section", 4))
	content.WriteString(m.renderSectionSelector())
	content.WriteString("\n")

	// Due date row
	content.WriteString(m.renderFormLabel("Due", 5))
	content.WriteString(m.renderDueDateField())
	content.WriteString("\n")

	// Flags row
	content.WriteString(m.renderFormLabel("Flags", 6))
	content.WriteString(m.renderFlagsField())
	content.WriteString("\n")

	// Recurrence row
	content.WriteString(m.renderFormLabel("Recur", 7))
	content.WriteString(m.renderRecurrenceSelector())
	content.WriteString("\n\n")

	// Tags row (read-only)
	content.WriteString(m.renderFormLabel("Tags", -1))
	if len(m.tags) == 0 {
		content.WriteString(MutedStyle.Render("(none)"))
	} else {
		content.WriteString(strings.Join(m.tags, ", "))
	}
	content.WriteString("  ")
	content.WriteString(HelpStyle.Render("Ctrl+T"))
	content.WriteString("\n")

	// Description row (read-only)
	content.WriteString(m.renderFormLabel("Description", -1))
	if m.task.Description == "" {
		content.WriteString(MutedStyle.Render("(empty)"))
	} else {
		lines := strings.Count(m.task.Description, "\n") + 1
		chars := len(m.task.Description)
		content.WriteString(MutedStyle.Render(fmt.Sprintf("(%d lines, %d chars)", lines, chars)))
	}
	content.WriteString("  ")
	content.WriteString(HelpStyle.Render("Ctrl+O"))

	// Error display inside card
	if m.err != nil {
		content.WriteString("\n\n")
		content.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	// Build card title
	cardTitle := fmt.Sprintf("Edit Task #%d", m.task.ShortID)
	if m.hasChanges {
		cardTitle += " *"
	}

	// Wrap content in card
	cardWidth := m.width - 4
	if cardWidth < 40 {
		cardWidth = 40
	}
	if cardWidth > 80 {
		cardWidth = 80
	}

	// Use renderCard for proper titled border
	config := CardConfig{
		Style:      FormCardStyle,
		TitleStyle: TitleStyle,
		Focused:    false,
	}
	card := renderCard(cardTitle, content.String(), cardWidth, config)

	var result strings.Builder
	result.WriteString(card)
	result.WriteString("\n")

	// Help footer outside card
	result.WriteString(HelpStyle.Render("  Tab/↔ navigate · Space toggle · Ctrl+S save · Ctrl+O editor · Esc cancel"))

	return result.String()
}

// renderFormLabel renders a form field label with fixed width alignment.
func (m *TaskEditorModel) renderFormLabel(label string, focusIdx int) string {
	// Pad label to fixed width
	paddedLabel := fmt.Sprintf("%-*s", FormLabelWidth, label)
	if focusIdx >= 0 && m.focusIndex == focusIdx {
		return FieldLabelFocusedStyle.Render(paddedLabel)
	}
	return FieldLabelStyle.Render(paddedLabel)
}

// renderTypeSelector renders the task type selector options.
func (m *TaskEditorModel) renderTypeSelector() string {
	var b strings.Builder
	for i, t := range m.types {
		var style = SelectorOptionStyle
		if i == m.typeCursor {
			if m.focusIndex == 1 {
				style = SelectorOptionSelectedStyle
			} else {
				style = BoldStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("[%s]", t.HeadingPrefix())))
		} else {
			b.WriteString(style.Render(fmt.Sprintf(" %s ", t.HeadingPrefix())))
		}
	}
	return b.String()
}

// renderParentSelector renders the parent task selector options.
func (m *TaskEditorModel) renderParentSelector() string {
	var b strings.Builder

	// "None" option
	if m.parentCursor == 0 {
		if m.focusIndex == 2 {
			b.WriteString(SelectorOptionSelectedStyle.Render("[None]"))
		} else {
			b.WriteString(BoldStyle.Render("[None]"))
		}
	} else {
		b.WriteString(SelectorOptionStyle.Render(" None "))
	}

	// Show a few valid parents
	maxShow := 3
	startIdx := 0
	if m.parentCursor > maxShow {
		startIdx = m.parentCursor - maxShow
	}

	for i := startIdx; i < len(m.validParents) && i < startIdx+maxShow+1; i++ {
		p := m.validParents[i]
		display := util.Truncate(fmt.Sprintf("#%d %s", p.ShortID, p.Title), 18)
		if m.parentCursor == i+1 {
			if m.focusIndex == 2 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf(" [%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf(" [%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	if len(m.validParents) > maxShow+1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" +%d", len(m.validParents)-maxShow-1)))
	}
	return b.String()
}

// renderPrioritySelector renders the priority selector options.
func (m *TaskEditorModel) renderPrioritySelector() string {
	var b strings.Builder
	for i := 1; i <= 5; i++ {
		if i == m.priority {
			if m.focusIndex == 3 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%d]", i)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%d]", i)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %d ", i)))
		}
	}
	b.WriteString(MutedStyle.Render(" (1=high)"))
	return b.String()
}

// renderSectionSelector renders the section selector options.
func (m *TaskEditorModel) renderSectionSelector() string {
	var b strings.Builder
	for i, s := range m.sections {
		if i == m.sectionCursor {
			if m.focusIndex == 4 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", s)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", s)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", s)))
		}
	}
	return b.String()
}

// renderDueDateField renders the due date input field.
func (m *TaskEditorModel) renderDueDateField() string {
	var b strings.Builder
	if m.focusIndex == 5 {
		b.WriteString(SelectorOptionSelectedStyle.Render(m.dueDateInput + "█"))
	} else if m.dueDateInput == "" {
		b.WriteString(MutedStyle.Render("(none)"))
	} else {
		b.WriteString(m.dueDateInput)
	}
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("tomorrow, +3d, 2024-01-15"))
	return b.String()
}

// renderFlagsField renders the pinned and active flag toggles.
func (m *TaskEditorModel) renderFlagsField() string {
	var b strings.Builder

	// Pinned toggle
	pinnedText := "Pin"
	if m.pinned {
		pinnedText = "Pin ✓"
	}
	if m.focusIndex == 6 && m.flagFocus == 0 {
		b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", pinnedText)))
	} else if m.pinned {
		b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", pinnedText)))
	} else {
		b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", pinnedText)))
	}

	b.WriteString(" ")

	// Active toggle
	activeText := "Active"
	if m.active {
		activeText = "Active ✓"
	}
	if m.focusIndex == 6 && m.flagFocus == 1 {
		b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", activeText)))
	} else if m.active {
		b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", activeText)))
	} else {
		b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", activeText)))
	}
	return b.String()
}

// renderRecurrenceSelector renders the recurrence pattern selector options.
func (m *TaskEditorModel) renderRecurrenceSelector() string {
	var b strings.Builder
	for i, opt := range m.recurrenceOpts {
		display := opt
		// For "Custom..." option, show the actual pattern if we have one
		if i == 6 && m.recurrenceCursor == 6 && m.recurrence != nil {
			display = m.recurrence.String()
		}
		if i == m.recurrenceCursor {
			if m.focusIndex == 7 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", opt)))
		}
	}
	// Add hint for 'r' keybind
	if m.focusIndex == 7 {
		b.WriteString("  ")
		b.WriteString(HelpStyle.Render("r: edit"))
	}
	return b.String()
}
