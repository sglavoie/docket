package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/util"
)

// formFactory is a shared factory instance for resizing form fields.
var formFactory = DefaultFormFieldFactory()

// TaskCreatorModel handles creating a new task via TUI.
type TaskCreatorModel struct {
	project       *models.Project
	config        *config.Config
	configManager *config.Manager

	// Form fields
	titleInput textarea.Model
	descValue  string
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
	recurrenceOpts   []string

	// Form state
	focusIndex int // 0=title, 1=type, 2=parent, 3=priority, 4=section, 5=due date, 6=flags, 7=recurrence, 8=tags, 9=description
	width      int
	height     int
	err        error

	// Discard confirmation state
	confirmDiscard bool

	// Template support
	fromTemplate *template.TaskTemplate // Template used to create this task (for children creation)
}

// NewTaskCreatorModel creates a new task creator.
func NewTaskCreatorModel(project *models.Project, cfg *config.Config, manager *config.Manager, suggestedParent *models.Task) *TaskCreatorModel {
	factory := DefaultFormFieldFactory()

	titleInput := factory.NewTitleInput("Enter task title...")
	titleInput.Focus()
	titleInput.SetPromptFunc(0, func(lineIdx int) string { return "" })

	types := []models.TaskType{
		models.TaskTypeTask,
		models.TaskTypeStory,
		models.TaskTypeEpic,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}

	// Get available sections from project
	sections := project.GetSections()

	// Recurrence options: None, daily, weekly, biweekly, monthly, yearly, custom
	recurrenceOpts := []string{"None", "Daily", "Weekly", "Biweekly", "Monthly", "Yearly", "Custom..."}

	m := &TaskCreatorModel{
		project:          project,
		config:           cfg,
		configManager:    manager,
		titleInput:       titleInput,
		descValue:        "",
		tags:             nil,
		taskType:         models.TaskTypeTask,
		typeCursor:       0,
		types:            types,
		parentID:         nil,
		parentCursor:     0,
		priority:         3,
		section:          models.SectionNext,
		sectionCursor:    0,
		sections:         sections,
		dueDate:          nil,
		dueDateInput:     "",
		pinned:           false,
		active:           false,
		flagFocus:        0,
		recurrence:       nil,
		recurrenceCursor: 0,
		recurrenceOpts:   recurrenceOpts,
		focusIndex:       0,
		width:            80,
		height:           24,
	}

	// Set initial parent by finding nearest valid ancestor
	if suggestedParent != nil {
		// Walk up the parent chain to find a task that can accept this child type
		candidate := suggestedParent
		for candidate != nil {
			if candidate.CanAddChild(m.taskType) {
				m.parentID = &candidate.ID
				break
			}
			candidate = candidate.Parent
		}
	}

	m.updateValidParents()

	return m
}

// NewTaskCreatorModelWithTemplate creates a new task creator pre-filled from a template.
func NewTaskCreatorModelWithTemplate(project *models.Project, cfg *config.Config, manager *config.Manager, suggestedParent *models.Task, tmpl *template.TaskTemplate) *TaskCreatorModel {
	m := NewTaskCreatorModel(project, cfg, manager, suggestedParent)

	if tmpl != nil {
		m.fromTemplate = tmpl

		// Pre-fill type
		m.taskType = tmpl.Type
		for i, t := range m.types {
			if t == tmpl.Type {
				m.typeCursor = i
				break
			}
		}

		// Pre-fill priority
		if tmpl.Priority > 0 {
			m.priority = tmpl.Priority
		}

		// Pre-fill tags
		if len(tmpl.Tags) > 0 {
			m.tags = append([]string{}, tmpl.Tags...)
		}

		// Pre-fill section
		if tmpl.Section != "" {
			project.EnsureSection(tmpl.Section)
			m.section = tmpl.Section
			// Update cursor
			for i, s := range m.sections {
				if s == tmpl.Section {
					m.sectionCursor = i
					break
				}
			}
		}

		// Pre-fill description
		if tmpl.Description != "" {
			m.descValue = tmpl.Description
		}

		// Update valid parents for the new type
		m.updateValidParents()
	}

	return m
}

// updateValidParents filters tasks that can be parents of the current type.
func (m *TaskCreatorModel) updateValidParents() {
	m.validParents = []*models.Task{}

	for _, task := range m.project.Tasks {
		if task.CanAddChild(m.taskType) {
			m.validParents = append(m.validParents, task)
		}
	}

	// Reset parent selection if current parent is no longer valid
	if m.parentID != nil {
		found := false
		for i, p := range m.validParents {
			if p.ID == *m.parentID {
				found = true
				m.parentCursor = i + 1 // +1 because 0 is "None"
				break
			}
		}
		if !found {
			m.parentID = nil
			m.parentCursor = 0
		}
	}
}

// SetSize sets the view dimensions.
func (m *TaskCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextarea(&m.titleInput, width)
}

const creatorFieldCount = 10 // title, type, parent, priority, section, due date, flags, recurrence, tags, description

func (m *TaskCreatorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *TaskCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ParentSelectedMsg:
		// Handle parent selection from modal
		if msg.Source == ViewTaskCreator {
			m.parentID = msg.ParentID
			// Update cursor position to match
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
		}
		return m, nil

	case RecurrenceEditorDoneMsg:
		if !msg.Canceled && msg.Pattern != nil {
			m.recurrence = msg.Pattern
			// Update cursor to "Custom..." since we have a custom pattern
			m.recurrenceCursor = 6
		}
		return m, nil

	case TagSelectorDoneMsg:
		if !msg.Canceled {
			m.tags = msg.SelectedTags
		}
		return m, nil

	case TextEditorDoneMsg:
		if !msg.Canceled {
			if msg.FieldName == "description" {
				m.descValue = msg.Value
			}
		}
		return m, nil

	case tea.KeyMsg:
		// Handle discard confirmation
		if m.confirmDiscard {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg {
					return BackMsg{}
				}
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "esc":
			// Show confirmation if title has content
			title := strings.TrimSpace(m.titleInput.Value())
			if title != "" {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % creatorFieldCount
			} else {
				m.focusIndex = (m.focusIndex + creatorFieldCount - 1) % creatorFieldCount
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "enter":
			// For due date field, parse and validate on enter
			if m.focusIndex == 5 {
				if err := m.parseDueDate(); err != nil {
					m.err = err
					return m, nil
				}
				m.err = nil
			}
			if m.focusIndex == 8 {
				// Open tag selector modal
				return m, func() tea.Msg {
					return ShowTagSelectorMsg{
						Task:        nil,
						Project:     m.project,
						CurrentTags: m.tags,
					}
				}
			}
			if m.focusIndex == 9 {
				// Open description editor popup
				return m, func() tea.Msg {
					return ShowTextEditorMsg{
						FieldName:    "description",
						CurrentValue: m.descValue,
					}
				}
			}
			// Move to next field for other fields
			if m.focusIndex < creatorFieldCount-1 {
				m.blurCurrentField()
				m.focusIndex++
				m.focusCurrentField()
				return m, nil
			}

		case "ctrl+s":
			// Submit form
			return m, m.createTask()

		case "ctrl+t":
			// Open tag selector from any field
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{
					Task:        nil,
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
						Source:       ViewTaskCreator,
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
					m.parentID = &m.validParents[m.parentCursor-1].ID
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

	return m, cmd
}

func (m *TaskCreatorModel) blurCurrentField() {
	if m.focusIndex == 0 {
		m.titleInput.Blur()
	}
}

func (m *TaskCreatorModel) focusCurrentField() {
	if m.focusIndex == 0 {
		m.titleInput.Focus()
	}
}

// parseDueDate parses the due date input string and updates m.dueDate.
func (m *TaskCreatorModel) parseDueDate() error {
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

// updateRecurrenceFromCursor updates the recurrence pattern based on cursor position.
func (m *TaskCreatorModel) updateRecurrenceFromCursor() {
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

func (m *TaskCreatorModel) createTask() tea.Cmd {
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

		now := time.Now()

		// Create task
		task := &models.Task{
			ID:          uuid.New().String(),
			ShortID:     m.project.GetNextShortID(),
			Type:        m.taskType,
			Title:       title,
			Description: strings.TrimSpace(m.descValue),
			Status:      models.StatusTodo,
			Priority:    m.priority,
			ParentID:    m.parentID,
			Section:     m.section,
			DueDate:     m.dueDate,
			Pinned:      m.pinned,
			Active:      m.active,
			Recurrence:  m.recurrence,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		// Set PinnedAt if pinned
		if m.pinned {
			task.PinnedAt = &now
		}

		// Set tags from the selector
		task.Tags = append([]string{}, m.tags...)

		// Add task to project
		m.project.Tasks = append(m.project.Tasks, task)

		// Create checklist from template (prefer Checklist over legacy Children)
		if m.fromTemplate != nil {
			checklistCreated := false
			if len(m.fromTemplate.Checklist) > 0 {
				for _, item := range m.fromTemplate.Checklist {
					task.Checklist = append(task.Checklist, models.ChecklistItem{
						Text: item,
						Done: false,
					})
				}
				checklistCreated = true
			} else if len(m.fromTemplate.Children) > 0 {
				// Legacy support: convert Children to checklist items
				for _, childTmpl := range m.fromTemplate.Children {
					task.Checklist = append(task.Checklist, models.ChecklistItem{
						Text: childTmpl.Title,
						Done: false,
					})
				}
				checklistCreated = true
			}

			// Increment template use count
			if checklistCreated && m.config != nil && m.configManager != nil {
				m.config.IncrementTemplateUseCount(m.fromTemplate.ID)
				_ = m.configManager.Save(m.config)
			}
		}

		m.project.BuildTaskTree()
		m.project.ReorderRootTasksByCompletion()

		// Save project with backup for undo support
		if err := saveProjectSync(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TaskCreatedMsg{Task: task}
	}
}

func (m *TaskCreatorModel) View() string {
	var b strings.Builder

	// Title field (inline textarea)
	b.WriteString(renderFormLabelShared("Title", 0, m.focusIndex))
	b.WriteString("\n")
	b.WriteString(m.titleInput.View())
	b.WriteString("\n")

	// Type selector
	b.WriteString(renderFormLabelShared("Type", 1, m.focusIndex))
	b.WriteString(renderTypeSelectorShared(m.types, m.typeCursor, m.focusIndex == 1))
	b.WriteString("\n")

	// Parent selector
	b.WriteString(renderFormLabelShared("Parent", 2, m.focusIndex))
	b.WriteString(renderParentSelectorShared(m.validParents, m.parentCursor, m.focusIndex == 2))
	b.WriteString("\n")

	// Priority selector
	b.WriteString(renderFormLabelShared("Priority", 3, m.focusIndex))
	b.WriteString(renderPrioritySelectorShared(m.priority, m.focusIndex == 3))
	b.WriteString("\n")

	// Section selector
	b.WriteString(renderFormLabelShared("Section", 4, m.focusIndex))
	b.WriteString(renderSectionSelectorShared(m.sections, m.sectionCursor, m.focusIndex == 4))
	b.WriteString("\n")

	// Due date row
	b.WriteString(renderFormLabelShared("Due", 5, m.focusIndex))
	b.WriteString(renderDueDateFieldShared(m.dueDateInput, m.focusIndex == 5))
	b.WriteString("\n")

	// Flags row
	b.WriteString(renderFormLabelShared("Flags", 6, m.focusIndex))
	b.WriteString(renderFlagsSelectorShared(m.pinned, m.active, m.flagFocus, m.focusIndex == 6))
	b.WriteString("\n")

	// Recurrence selector
	b.WriteString(renderFormLabelShared("Recur", 7, m.focusIndex))
	b.WriteString(renderRecurrenceSelectorShared(m.recurrenceOpts, m.recurrenceCursor, m.focusIndex == 7, m.recurrence))
	b.WriteString("\n")

	// Tags row (read-only, modal)
	b.WriteString(renderFormLabelShared("Tags", 8, m.focusIndex))
	if len(m.tags) == 0 {
		b.WriteString(MutedStyle.Render("(none)"))
	} else {
		b.WriteString(strings.Join(m.tags, ", "))
	}
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("Ctrl+T"))
	b.WriteString("\n")

	// Description field (single line, truncated, Enter to edit)
	b.WriteString(renderFormLabelShared("Desc", 9, m.focusIndex))
	descDisplay := m.descValue
	if descDisplay == "" {
		descDisplay = MutedStyle.Render("(press Enter to edit)")
	} else {
		descDisplay = strings.ReplaceAll(descDisplay, "\n", " ")
		descDisplay = util.Truncate(descDisplay, 50)
	}
	if m.focusIndex == 9 {
		b.WriteString(SelectorOptionSelectedStyle.Render(descDisplay))
	} else {
		b.WriteString(descDisplay)
	}

	// Discard confirmation
	if m.confirmDiscard {
		b.WriteString("\n\n")
		b.WriteString(PreviewWarningStyle.Render("Discard unsaved changes? (y/n)"))
	}

	// Error display
	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	// Footer
	left := HelpStyle.Render("Tab/↔ navigate · Space toggle · Ctrl+S save · Ctrl+T tags · /: search parents · Esc cancel")
	right := MutedStyle.Render("Create Task")
	content := b.String()

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Create Task", ""))
	out.WriteString("\n\n")
	out.WriteString(content)
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return out.String()
}

// HasTextInput returns true because the task creator always has inputs.
func (m *TaskCreatorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true when confirmation is active.
func (m *TaskCreatorModel) ShouldCaptureEsc() bool {
	return m.confirmDiscard
}
