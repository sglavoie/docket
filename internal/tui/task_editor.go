package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
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

	// Form fields
	titleInput textarea.Model
	tagsInput  textinput.Model

	// Selectable fields
	taskType     models.TaskType
	typeCursor   int
	types        []models.TaskType
	parentID     *string
	parentCursor int
	validParents []*models.Task
	priority     int

	// Form state
	focusIndex int // 0=title, 1=type, 2=parent, 3=priority, 4=tags
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

	tagsStr := strings.Join(task.Tags, ", ")
	// DEBUG: Log task tags when editor opens
	fmt.Fprintf(os.Stderr, "[TaskEditor] Opening editor for task %s, tags: %v, tagsStr: %q\n", task.ID, task.Tags, tagsStr)
	tagsInput := factory.NewTextInputWithValue(tagsStr, "tag1, tag2, tag3")

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

	m := &TaskEditorModel{
		project:          project,
		task:             task,
		config:           cfg,
		configManager:    manager,
		originalTitle:    task.Title,
		originalTags:     tagsStr,
		originalType:     task.Type,
		originalParentID: task.ParentID,
		originalPriority: task.Priority,
		titleInput:       titleInput,
		tagsInput:        tagsInput,
		taskType:         task.Type,
		typeCursor:       typeCursor,
		types:            types,
		parentID:         task.ParentID,
		parentCursor:     0,
		priority:         task.Priority,
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
	if strings.TrimSpace(m.tagsInput.Value()) != m.originalTags {
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
	return false
}

// SetSize sets the view dimensions.
func (m *TaskEditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextarea(&m.titleInput, width)
	formFactory.ResizeTextInput(&m.tagsInput, width)
}

func (m *TaskEditorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *TaskEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Move focus (5 fields: title, type, parent, priority, tags)
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 5
			} else {
				m.focusIndex = (m.focusIndex + 4) % 5
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "enter":
			// Move to next field
			if m.focusIndex < 4 {
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
			}
		}
	}

	// Update the focused input
	switch m.focusIndex {
	case 0:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case 4:
		m.tagsInput, cmd = m.tagsInput.Update(msg)
	}

	// Check for changes
	m.hasChanges = m.checkChanges()

	return m, cmd
}

func (m *TaskEditorModel) blurCurrentField() {
	switch m.focusIndex {
	case 0:
		m.titleInput.Blur()
	case 4:
		m.tagsInput.Blur()
	}
}

func (m *TaskEditorModel) focusCurrentField() {
	switch m.focusIndex {
	case 0:
		m.titleInput.Focus()
	case 4:
		m.tagsInput.Focus()
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

		// Update task fields (description is edited separately via external editor)
		m.task.Title = title
		m.task.Type = m.taskType
		m.task.Priority = m.priority
		m.task.ParentID = m.parentID
		m.task.UpdatedAt = time.Now()

		// Parse and set tags
		m.task.Tags = nil
		tagsStr := strings.TrimSpace(m.tagsInput.Value())
		// DEBUG: Log tags being saved
		fmt.Fprintf(os.Stderr, "[TaskEditor] Saving task %s, input tagsStr: %q\n", m.task.ID, tagsStr)
		if tagsStr != "" {
			tags := strings.Split(tagsStr, ",")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					m.task.AddTag(tag)
				}
			}
		}
		// DEBUG: Log final tags after parsing
		fmt.Fprintf(os.Stderr, "[TaskEditor] Task %s final tags: %v\n", m.task.ID, m.task.Tags)

		// DEBUG: Verify task is in project
		foundInProject := false
		for _, t := range m.project.Tasks {
			if t.ID == m.task.ID {
				foundInProject = true
				fmt.Fprintf(os.Stderr, "[TaskEditor] Found task in project.Tasks, tags: %v\n", t.Tags)
				fmt.Fprintf(os.Stderr, "[TaskEditor] m.task pointer: %p, project task pointer: %p, SAME: %v\n", m.task, t, m.task == t)
				// FIX: Update the actual task in the project
				t.Tags = m.task.Tags
				t.Title = m.task.Title
				t.Type = m.task.Type
				t.Priority = m.task.Priority
				t.ParentID = m.task.ParentID
				t.UpdatedAt = m.task.UpdatedAt
				fmt.Fprintf(os.Stderr, "[TaskEditor] Copied changes to project task, tags now: %v\n", t.Tags)
				break
			}
		}
		if !foundInProject {
			fmt.Fprintf(os.Stderr, "[TaskEditor] WARNING: Task %s NOT found in project.Tasks!\n", m.task.ID)
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

		// DEBUG: Log after save
		fmt.Fprintf(os.Stderr, "[TaskEditor] After SaveProject, checking task in project again...\n")
		for _, t := range m.project.Tasks {
			if t.ID == m.task.ID {
				fmt.Fprintf(os.Stderr, "[TaskEditor] Task %s in project now has tags: %v\n", t.ID, t.Tags)
				break
			}
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

	// Tags input
	tagsLabel := "Tags: "
	if m.focusIndex == 4 {
		tagsLabel = FieldLabelFocusedStyle.Render("Tags: ")
	} else {
		tagsLabel = FieldLabelStyle.Render("Tags: ")
	}
	b.WriteString(tagsLabel)
	b.WriteString(m.tagsInput.View())
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
			"  Ctrl+O         Edit description\n" +
			"  Ctrl+S         Save changes\n" +
			"  Esc            Cancel",
	)
	b.WriteString(help)

	return b.String()
}
