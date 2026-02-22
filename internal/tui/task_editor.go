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
	flagCursor       int // 0=None, 1=Pin, 2=Active (mutually exclusive)
	recurrence       *models.RecurrencePattern
	recurrenceCursor int
	recurrenceOpts   []string // Display labels for recurrence options

	// URL and Source fields
	urlInput    string
	sourceInput string

	// Original URL/Source for change detection
	originalURL    string
	originalSource string

	// Type-specific attributes
	// Bug attributes
	bugReproducibleCursor int
	bugSeverityCursor     int
	bugWorkaround         string

	// Idea attributes
	ideaCategoryCursor int
	ideaEffortCursor   int
	ideaValueCursor    int
	ideaValidated      bool

	// Task attributes
	taskEffortCursor   int
	taskWorkTypeCursor int

	// Story attributes
	storyPointsCursor int
	storyPersona      string

	// Epic attributes
	epicScopeCursor    int
	epicMilestone      string
	epicSuccessMetrics string

	// Original attribute values for change detection
	originalBugAttrs   *models.BugAttrs
	originalIdeaAttrs  *models.IdeaAttrs
	originalTaskAttrs  *models.TaskAttrs
	originalStoryAttrs *models.StoryAttrs
	originalEpicAttrs  *models.EpicAttrs

	// Type change confirmation
	confirmTypeChange bool
	pendingTypeCursor int
	pendingTaskType   models.TaskType

	// Form state
	focusIndex int // 0=title, 1=type, 2=parent, 3=priority, 4=section, 5=due date, 6=flags, 7=recurrence, 8=url, 9=source, 10+=attrs
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
		models.TaskTypeAction,
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
		flagCursor:         flagCursorFromFlags(task.Pinned, task.Active),
		recurrence:         recurrence,
		recurrenceCursor:   recurrenceCursor,
		recurrenceOpts:     recurrenceOpts,
		originalRecurrence: originalRecurrence,
		urlInput:           task.URL,
		sourceInput:        task.Source,
		originalURL:        task.URL,
		originalSource:     task.Source,
		focusIndex:         0,
		width:              80,
		height:             24,
	}

	// Load existing type-specific attributes
	m.loadExistingAttrs(task)

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

	case TextEditorDoneMsg:
		if !msg.Canceled {
			switch msg.FieldName {
			case "bugWorkaround":
				m.bugWorkaround = msg.Value
			case "storyPersona":
				m.storyPersona = msg.Value
			case "epicMilestone":
				m.epicMilestone = msg.Value
			case "epicSuccessMetrics":
				m.epicSuccessMetrics = msg.Value
			}
			m.hasChanges = m.checkChanges()
		}
		return m, nil

	case tea.KeyMsg:
		// Handle type change confirmation
		if m.confirmTypeChange {
			switch msg.String() {
			case "y", "Y":
				m.confirmTypeChange = false
				m.clearCurrentAttrs()
				m.typeCursor = m.pendingTypeCursor
				m.taskType = m.pendingTaskType
				m.updateValidParents()
				m.hasChanges = m.checkChanges()
				return m, nil
			case "n", "N", "esc":
				m.confirmTypeChange = false
				return m, nil
			}
			return m, nil
		}
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Move focus (8 base fields + attribute fields)
			totalFields := m.totalFieldCount()
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % totalFields
			} else {
				m.focusIndex = (m.focusIndex + totalFields - 1) % totalFields
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
			// Handle text attribute fields on enter
			attrsStart := m.attrsStartIndex()
			if m.focusIndex >= attrsStart {
				fieldIdx := m.focusIndex - attrsStart
				if msgFn := m.handleAttrEnter(fieldIdx); msgFn != nil {
					return m, func() tea.Msg {
						return msgFn().(tea.Msg)
					}
				}
			}
			// Move to next field
			totalFields := m.totalFieldCount()
			if m.focusIndex < totalFields-1 {
				m.blurCurrentField()
				m.focusIndex++
				m.focusCurrentField()
				return m, nil
			}

		case "ctrl+e":
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
			// Check if in attribute fields
			attrsStart := m.attrsStartIndex()
			if m.focusIndex >= attrsStart {
				fieldIdx := m.focusIndex - attrsStart
				m.handleAttrLeft(fieldIdx)
				m.hasChanges = m.checkChanges()
				return m, nil
			}
			switch m.focusIndex {
			case 1: // Type selector
				if m.typeCursor > 0 {
					newCursor := m.typeCursor - 1
					newType := m.types[newCursor]
					// Check if changing type would lose attrs
					if m.hasNonEmptyAttrs() && newType != m.taskType {
						m.confirmTypeChange = true
						m.pendingTypeCursor = newCursor
						m.pendingTaskType = newType
						return m, nil
					}
					m.typeCursor = newCursor
					m.taskType = newType
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
			case 6: // Flags - switch between None, Pin, Active
				if m.flagCursor > 0 {
					m.flagCursor--
					m.updateFlagsFromCursor()
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
			// Check if in attribute fields
			attrsStart := m.attrsStartIndex()
			if m.focusIndex >= attrsStart {
				fieldIdx := m.focusIndex - attrsStart
				m.handleAttrRight(fieldIdx)
				m.hasChanges = m.checkChanges()
				return m, nil
			}
			switch m.focusIndex {
			case 1: // Type selector
				if m.typeCursor < len(m.types)-1 {
					newCursor := m.typeCursor + 1
					newType := m.types[newCursor]
					// Check if changing type would lose attrs
					if m.hasNonEmptyAttrs() && newType != m.taskType {
						m.confirmTypeChange = true
						m.pendingTypeCursor = newCursor
						m.pendingTaskType = newType
						return m, nil
					}
					m.typeCursor = newCursor
					m.taskType = newType
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
			case 6: // Flags - switch between None, Pin, Active
				if m.flagCursor < 2 {
					m.flagCursor++
					m.updateFlagsFromCursor()
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
			// Space is a no-op for flags (selection happens via left/right)
			if m.focusIndex == 6 {
				return m, nil
			}
			// Space toggles validated for idea attrs
			attrsStart := m.attrsStartIndex()
			if m.focusIndex >= attrsStart && m.taskType == models.TaskTypeIdea {
				fieldIdx := m.focusIndex - attrsStart
				if fieldIdx == 3 { // Validated field
					m.ideaValidated = !m.ideaValidated
					m.hasChanges = m.checkChanges()
					return m, nil
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
			if key == "backspace" && len(m.dueDateInput) > 0 {
				m.dueDateInput = m.dueDateInput[:len(m.dueDateInput)-1]
			} else if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
				m.dueDateInput += key
			}
		}
	case 8: // URL text input
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			key := keyMsg.String()
			if key == "backspace" && len(m.urlInput) > 0 {
				m.urlInput = m.urlInput[:len(m.urlInput)-1]
			} else if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
				m.urlInput += key
			}
		}
	case 9: // Source text input
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			key := keyMsg.String()
			if key == "backspace" && len(m.sourceInput) > 0 {
				m.sourceInput = m.sourceInput[:len(m.sourceInput)-1]
			} else if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
				m.sourceInput += key
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
	if len(m.validParents) > 0 {
		content.WriteString("  ")
		content.WriteString(HelpStyle.Render("/ to search"))
	}
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

	// URL row
	content.WriteString(m.renderFormLabel("URL", 8))
	content.WriteString(m.renderURLField())
	content.WriteString("\n")

	// Source row
	content.WriteString(m.renderFormLabel("Source", 9))
	content.WriteString(m.renderSourceField())
	content.WriteString("\n")

	// Type-specific attributes section
	content.WriteString(m.renderAttributesSection())

	// Score summary (read-only, when scoring profile is active)
	content.WriteString(m.renderScoreSummary())

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
	content.WriteString(HelpStyle.Render("Ctrl+E"))

	// Type change confirmation
	if m.confirmTypeChange {
		content.WriteString("\n")
		content.WriteString(PreviewWarningStyle.Render(fmt.Sprintf("Changing type will clear %s attributes. Continue? (y/n)", m.taskType.HeadingPrefix())))
	}

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
	left := HelpStyle.Render("Tab/↔ navigate · Space toggle · Ctrl+S save · Ctrl+E editor · Esc cancel")
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

// ShouldCaptureEsc returns true when confirmation is active.
func (m *TaskEditorModel) ShouldCaptureEsc() bool {
	return m.confirmTypeChange
}

func (m *TaskEditorModel) SupportsHelpShortcut() bool { return false }

func (m *TaskEditorModel) StatusBarLabel() string { return "Edit Task" }
