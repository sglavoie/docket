package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
	descInput  textarea.Model
	tagsInput  textinput.Model

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
	recurrence       *models.RecurrencePattern
	recurrenceCursor int
	recurrenceOpts   []string

	// Form state
	focusIndex int // 0=title, 1=type, 2=parent, 3=priority, 4=section, 5=recurrence, 6=tags, 7=description
	width      int
	height     int
	err        error
}

// NewTaskCreatorModel creates a new task creator.
func NewTaskCreatorModel(project *models.Project, cfg *config.Config, manager *config.Manager, suggestedParent *models.Task) *TaskCreatorModel {
	factory := DefaultFormFieldFactory()

	titleInput := factory.NewTitleInputFocused("Task title")
	titleInput.SetPromptFunc(0, func(lineIdx int) string { return "" })

	tagsInput := factory.NewTextInput("tag1, tag2, tag3")

	descInput := factory.NewDescriptionInput("Description (optional)")
	descInput.SetHeight(4)
	descInput.CharLimit = 5000
	descInput.SetPromptFunc(0, func(lineIdx int) string { return "" })

	types := []models.TaskType{
		models.TaskTypeTask,
		models.TaskTypeStory,
		models.TaskTypeEpic,
		models.TaskTypeBug,
		models.TaskTypeIdea,
		models.TaskTypeNote,
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
		descInput:        descInput,
		tagsInput:        tagsInput,
		taskType:         models.TaskTypeTask,
		typeCursor:       0,
		types:            types,
		parentID:         nil,
		parentCursor:     0,
		priority:         3,
		section:          models.SectionBacklog,
		sectionCursor:    0,
		sections:         sections,
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
	formFactory.ResizeTextInput(&m.tagsInput, width)
	formFactory.ResizeTextarea(&m.descInput, width)
}

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
		if !msg.Cancelled && msg.Pattern != nil {
			m.recurrence = msg.Pattern
			// Update cursor to "Custom..." since we have a custom pattern
			m.recurrenceCursor = 6
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

			// Move focus (8 fields: title, type, parent, priority, section, recurrence, tags, description)
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 8
			} else {
				m.focusIndex = (m.focusIndex + 7) % 8
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "enter":
			if m.focusIndex == 7 {
				// In description, allow newlines
				m.descInput, cmd = m.descInput.Update(msg)
				return m, cmd
			}
			// Move to next field or submit if on last navigable field
			if m.focusIndex < 7 {
				m.blurCurrentField()
				m.focusIndex++
				m.focusCurrentField()
				return m, nil
			}

		case "ctrl+s":
			// Submit form
			return m, m.createTask()

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
			case 5: // Recurrence selector
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
			case 5: // Recurrence selector
				if m.recurrenceCursor < len(m.recurrenceOpts)-1 {
					m.recurrenceCursor++
					if m.recurrenceCursor == 6 {
						// "Custom..." selected - open the editor
						return m, func() tea.Msg {
							return ShowRecurrenceEditorMsg{
								CurrentPattern: m.recurrence,
								DueDate:        nil, // No due date set during creation
							}
						}
					}
					m.updateRecurrenceFromCursor()
				}
			}

		case "r":
			// Open recurrence editor when on recurrence field
			if m.focusIndex == 5 {
				return m, func() tea.Msg {
					return ShowRecurrenceEditorMsg{
						CurrentPattern: m.recurrence,
						DueDate:        nil,
					}
				}
			}
		}
	}

	// Update the focused input
	switch m.focusIndex {
	case 0:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case 6: // Tags
		m.tagsInput, cmd = m.tagsInput.Update(msg)
	case 7: // Description
		m.descInput, cmd = m.descInput.Update(msg)
	}

	return m, cmd
}

func (m *TaskCreatorModel) blurCurrentField() {
	switch m.focusIndex {
	case 0:
		m.titleInput.Blur()
	case 6: // Tags
		m.tagsInput.Blur()
	case 7: // Description
		m.descInput.Blur()
	}
}

func (m *TaskCreatorModel) focusCurrentField() {
	switch m.focusIndex {
	case 0:
		m.titleInput.Focus()
	case 6: // Tags
		m.tagsInput.Focus()
	case 7: // Description
		m.descInput.Focus()
	}
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

		// Create task
		task := &models.Task{
			ID:          uuid.New().String(),
			ShortID:     m.project.GetNextShortID(),
			Type:        m.taskType,
			Title:       title,
			Description: strings.TrimSpace(m.descInput.Value()),
			Status:      models.StatusTodo,
			Priority:    m.priority,
			ParentID:    m.parentID,
			Section:     m.section,
			Recurrence:  m.recurrence,
		}

		// Parse and set tags
		tagsStr := strings.TrimSpace(m.tagsInput.Value())
		if tagsStr != "" {
			tags := strings.Split(tagsStr, ",")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					task.AddTag(tag)
				}
			}
		}

		// Add task to project
		m.project.Tasks = append(m.project.Tasks, task)
		m.project.BuildTaskTree()
		m.project.ReorderRootTasksByCompletion()

		// Create backup before saving for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Save project
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reload to rebuild tree properly
		storage.LoadProject(m.project)

		return TaskCreatedMsg{Task: task}
	}
}

func (m *TaskCreatorModel) View() string {
	var b strings.Builder

	// Title input (textarea spans 2 lines)
	titleLabel := "Title:\n"
	if m.focusIndex == 0 {
		titleLabel = FieldLabelFocusedStyle.Render("Title:\n")
	} else {
		titleLabel = FieldLabelStyle.Render("Title:\n")
	}
	b.WriteString(titleLabel)
	b.WriteString(m.titleInput.View())
	b.WriteString("\n\n")

	// Type selector
	typeLabel := "Type: "
	if m.focusIndex == 1 {
		typeLabel = FieldLabelFocusedStyle.Render("Type: ")
	} else {
		typeLabel = FieldLabelStyle.Render("Type: ")
	}
	b.WriteString(typeLabel)
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
	b.WriteString("\n\n")

	// Parent selector
	parentLabel := "Parent: "
	if m.focusIndex == 2 {
		parentLabel = FieldLabelFocusedStyle.Render("Parent: ")
	} else {
		parentLabel = FieldLabelStyle.Render("Parent: ")
	}
	b.WriteString(parentLabel)

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
		display := util.Truncate(fmt.Sprintf("#%d %s", p.ShortID, p.Title), 20)
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
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" ... (%d more)", len(m.validParents)-maxShow-1)))
	}
	b.WriteString("\n\n")

	// Priority selector
	priorityLabel := "Priority: "
	if m.focusIndex == 3 {
		priorityLabel = FieldLabelFocusedStyle.Render("Priority: ")
	} else {
		priorityLabel = FieldLabelStyle.Render("Priority: ")
	}
	b.WriteString(priorityLabel)
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
	b.WriteString(MutedStyle.Render(" (1=highest)"))
	b.WriteString("\n\n")

	// Section selector
	sectionLabel := "Section: "
	if m.focusIndex == 4 {
		sectionLabel = FieldLabelFocusedStyle.Render("Section: ")
	} else {
		sectionLabel = FieldLabelStyle.Render("Section: ")
	}
	b.WriteString(sectionLabel)
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
	b.WriteString("\n\n")

	// Recurrence selector
	recurLabel := "Recurrence: "
	if m.focusIndex == 5 {
		recurLabel = FieldLabelFocusedStyle.Render("Recurrence: ")
	} else {
		recurLabel = FieldLabelStyle.Render("Recurrence: ")
	}
	b.WriteString(recurLabel)
	for i, opt := range m.recurrenceOpts {
		display := opt
		// For "Custom..." option, show the actual pattern if we have one
		if i == 6 && m.recurrenceCursor == 6 && m.recurrence != nil {
			display = m.recurrence.String()
		}
		if i == m.recurrenceCursor {
			if m.focusIndex == 5 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", opt)))
		}
	}
	// Add hint for 'r' keybind
	if m.focusIndex == 5 {
		b.WriteString("  ")
		b.WriteString(HelpStyle.Render("r: edit"))
	}
	b.WriteString("\n\n")

	// Tags input
	tagsLabel := "Tags: "
	if m.focusIndex == 6 {
		tagsLabel = FieldLabelFocusedStyle.Render("Tags: ")
	} else {
		tagsLabel = FieldLabelStyle.Render("Tags: ")
	}
	b.WriteString(tagsLabel)
	b.WriteString(m.tagsInput.View())
	b.WriteString("\n\n")

	// Description textarea
	descLabel := "Description:\n"
	if m.focusIndex == 7 {
		descLabel = FieldLabelFocusedStyle.Render("Description:\n")
	} else {
		descLabel = FieldLabelStyle.Render("Description:\n")
	}
	b.WriteString(descLabel)
	b.WriteString(m.descInput.View())

	// Error display
	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	// Help footer
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("Tab: fields · ←/→: options · /: search parents · Ctrl+S: save · Esc: cancel"))

	// Wrap in overlay card
	cardWidth := m.width - 4
	if cardWidth > 80 {
		cardWidth = 80
	}
	config := OverlayCardConfig{
		Title:        "Create Task",
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, config)
}
