package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// TaskEditorModel handles editing an existing task via TUI.
type TaskEditorModel struct {
	project       *models.Project
	task          *models.Task
	config        *config.Config
	configManager *config.Manager

	// Original values for change detection
	originalTitle    string
	originalTags     string
	originalType     models.TaskType
	originalParentID *string
	originalPriority int
	originalSection  string
	originalDueDate  *time.Time
	originalPinned   bool
	originalActive   bool

	// Form fields
	titleInput textarea.Model
	tags       []string

	// Selectable fields
	taskType      models.TaskType
	typeCursor    int
	types         []models.TaskType
	parentID      *string
	parentCursor  int
	validParents  []*models.Task
	priority      int
	section       string
	sectionCursor int
	sections      []string
	dueDate       *time.Time
	dueDateInput  string
	pinned        bool
	active        bool
	flagFocus     int // 0=pinned, 1=active (within the flags field)

	// Form state
	focusIndex int // 0=title, 1=type, 2=parent, 3=priority, 4=section, 5=due date, 6=flags
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

	m := &TaskEditorModel{
		project:          project,
		task:             task,
		config:           cfg,
		configManager:    manager,
		originalTitle:    task.Title,
		originalTags:     strings.Join(task.Tags, ", "),
		originalType:     task.Type,
		originalParentID: task.ParentID,
		originalPriority: task.Priority,
		originalSection:  task.Section,
		originalDueDate:  originalDueDate,
		originalPinned:   task.Pinned,
		originalActive:   task.Active,
		titleInput:       titleInput,
		tags:             tags,
		taskType:         task.Type,
		typeCursor:       typeCursor,
		types:            types,
		parentID:         task.ParentID,
		parentCursor:     0,
		priority:         task.Priority,
		section:          task.Section,
		sectionCursor:    sectionCursor,
		sections:         sections,
		dueDate:          dueDate,
		dueDateInput:     dueDateInput,
		pinned:           task.Pinned,
		active:           task.Active,
		flagFocus:        0,
		focusIndex:       0,
		width:            80,
		height:           24,
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
	return false
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

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Move focus (7 fields: title, type, parent, priority, section, due date, flags)
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 7
			} else {
				m.focusIndex = (m.focusIndex + 6) % 7
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "enter":
			// Move to next field (7 focusable fields: 0-6)
			if m.focusIndex < 6 {
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
				t.UpdatedAt = m.task.UpdatedAt
				break
			}
		}

		// Rebuild task tree
		m.project.BuildTaskTree()

		// Create backup before saving for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Save project
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TaskUpdatedMsg{Task: m.task}
	}
}

func (m *TaskEditorModel) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render(fmt.Sprintf("Edit Task #%d", m.task.ShortID)))
	if m.hasChanges {
		b.WriteString(" ")
		b.WriteString(UnsavedChangesStyle.Render("(unsaved changes)"))
	}
	b.WriteString("\n\n")

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

	// Due date input
	dueDateLabel := "Due Date: "
	if m.focusIndex == 5 {
		dueDateLabel = FieldLabelFocusedStyle.Render("Due Date: ")
	} else {
		dueDateLabel = FieldLabelStyle.Render("Due Date: ")
	}
	b.WriteString(dueDateLabel)
	if m.focusIndex == 5 {
		// Show editable input with cursor
		b.WriteString(SelectorOptionSelectedStyle.Render(m.dueDateInput + "█"))
	} else if m.dueDateInput == "" {
		b.WriteString(MutedStyle.Render("(none)"))
	} else {
		b.WriteString(m.dueDateInput)
	}
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("e.g., tomorrow, +3d, 2024-01-15, none"))
	b.WriteString("\n\n")

	// Flags (pinned and active)
	flagsLabel := "Flags: "
	if m.focusIndex == 6 {
		flagsLabel = FieldLabelFocusedStyle.Render("Flags: ")
	} else {
		flagsLabel = FieldLabelStyle.Render("Flags: ")
	}
	b.WriteString(flagsLabel)

	// Pinned toggle
	pinnedText := "Pinned"
	if m.pinned {
		pinnedText = "Pinned ✓"
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
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("Space to toggle"))
	b.WriteString("\n\n")

	// Tags display (read-only, edited via Ctrl+T)
	b.WriteString(FieldLabelStyle.Render("Tags: "))
	if len(m.tags) == 0 {
		b.WriteString(MutedStyle.Render("(none)"))
	} else {
		b.WriteString(strings.Join(m.tags, ", "))
	}
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("Press Ctrl+T to edit"))
	b.WriteString("\n\n")

	// Description preview (edited via external editor)
	b.WriteString(FieldLabelStyle.Render("Description: "))
	if m.task.Description == "" {
		b.WriteString(MutedStyle.Render("(empty)"))
	} else {
		lines := strings.Count(m.task.Description, "\n") + 1
		chars := len(m.task.Description)
		b.WriteString(MutedStyle.Render(fmt.Sprintf("(%d lines, %d chars)", lines, chars)))
	}
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("Press Ctrl+O to edit in external editor"))
	b.WriteString("\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Help
	help := BoxStyle.Render(
		BoldStyle.Render("Controls") + "\n" +
			"  Tab/Shift+Tab  Switch field\n" +
			"  Left/Right     Select option\n" +
			"  Space          Toggle flags\n" +
			"  Ctrl+T         Edit tags\n" +
			"  Ctrl+O         Edit description\n" +
			"  Ctrl+S         Save changes\n" +
			"  Esc            Cancel",
	)
	b.WriteString(help)

	return b.String()
}
