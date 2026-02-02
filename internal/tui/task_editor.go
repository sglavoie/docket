package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
		if !msg.Canceled {
			m.tags = msg.SelectedTags
			m.hasChanges = m.checkChanges()
		}
		return m, nil

	case RecurrenceEditorDoneMsg:
		if !msg.Canceled && msg.Pattern != nil {
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
			return m, func() tea.Msg {
				if err := m.saveTask(); err != nil {
					return ErrorMsg{Err: err}
				}
				return TaskUpdatedMsg{Task: m.task}
			}

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

func (m *TaskEditorModel) View() string {
	var content strings.Builder

	// Title field
	content.WriteString(m.renderFormLabel("Title", 0))
	content.WriteString("\n")
	content.WriteString(m.titleInput.View())
	content.WriteString("\n")

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
	content.WriteString("\n")

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

	// Build title
	cardTitle := fmt.Sprintf("Edit Task #%d", m.task.ShortID)
	if m.hasChanges {
		cardTitle += " *"
	}

	// Footer
	left := HelpStyle.Render("Tab/↔ navigate · Space toggle · Ctrl+S save · Ctrl+O editor · Esc cancel")
	right := MutedStyle.Render(cardTitle)

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(cardTitle, ""))
	out.WriteString("\n\n")
	out.WriteString(content.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return out.String()
}

// HasTextInput returns true because the task editor contains inputs.
func (m *TaskEditorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns false so the app can close the view on esc.
func (m *TaskEditorModel) ShouldCaptureEsc() bool {
	return false
}
