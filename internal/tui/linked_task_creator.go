package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// LinkedTaskCreatorModel handles creation of tasks linked to a source task.
type LinkedTaskCreatorModel struct {
	quickCaptureInput
	sourceTask     *models.Task
	linkType       LinkType
	width          int
	height         int
	confirmDiscard bool
	confirmOverlay ConfirmDialog
}

// NewLinkedTaskCreatorModel creates a new linked task creator.
func NewLinkedTaskCreatorModel(sourceTask *models.Task, project *models.Project, linkType LinkType) *LinkedTaskCreatorModel {
	return &LinkedTaskCreatorModel{
		quickCaptureInput: newQuickCaptureInput(
			project,
			nil, // no cross-project capture for linked tasks
			"^type Title !priority #tag @due ~recur /section $status",
		),
		sourceTask: sourceTask,
		linkType:   linkType,
		width:      80,
	}
}

// SetSize sets the view dimensions.
func (m *LinkedTaskCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.quickCaptureInput.width = width
	m.resizeTextarea(width)
}

func (m *LinkedTaskCreatorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *LinkedTaskCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle discard confirmation
	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()

		// Handle autocomplete navigation when active
		if m.handleAutocompleteKey(key) {
			return m, nil
		}

		switch key {
		case "esc":
			if strings.TrimSpace(m.inputValue()) != "" {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			if m.ac.Mode == acModeNone {
				return m, m.createTask()
			}
		}
	}

	cmd := m.updateInput(msg)
	return m, cmd
}

func (m *LinkedTaskCreatorModel) View() tea.View {
	var b strings.Builder

	// Header showing link type and source task
	linkName := m.linkTypeName()
	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Create %s for #%d", linkName, m.sourceTask.ShortID)))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(util.Truncate(m.sourceTask.Title, 50)))
	b.WriteString("\n\n")

	// Input field
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Show autocomplete suggestions if active
	if ac := m.renderAutocomplete(); ac != "" {
		b.WriteString(ac)
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
	b.WriteString(MutedStyle.Render("^type · !N · #tag · @date · ~recur · /section · $status"))
	b.WriteString("\n")
	var helpText string
	if m.isAutocompleteActive() {
		helpText = "Tab: accept · ↑↓: navigate · Esc: dismiss"
	} else {
		helpText = "Enter: create · Esc: cancel"
	}

	title := fmt.Sprintf("New %s", linkName)

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(HelpStyle.Render(helpText), "", m.width))

	return tea.NewView(out.String())
}

// HasTextInput returns true because this view uses a textarea.
func (m *LinkedTaskCreatorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true so the view always handles Esc internally.
func (m *LinkedTaskCreatorModel) ShouldCaptureEsc() bool {
	return true
}

// GetConfirmDialog returns the confirm dialog for overlay rendering.
func (m *LinkedTaskCreatorModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}

func (m *LinkedTaskCreatorModel) IsFullHeight() bool { return false }

func (m *LinkedTaskCreatorModel) linkTypeName() string {
	switch m.linkType {
	case LinkTypeBlocker:
		return "Blocker"
	case LinkTypeDependent:
		return "Dependent"
	case LinkTypeSubtask:
		return "Subtask"
	case LinkTypeFollowUp:
		return "Follow-up"
	default:
		return "Linked Task"
	}
}

func (m *LinkedTaskCreatorModel) createTask() tea.Cmd {
	// All mutations run synchronously in the Update() goroutine to avoid
	// data races with View() reading project.Tasks concurrently.

	input := m.inputValue()
	if input == "" {
		m.err = fmt.Errorf("task title cannot be empty")
		return nil
	}

	// Parse inline metadata from input
	result, err := parseQuickCapture(input, m.project, nil)
	if err != nil {
		m.err = err
		return nil
	}

	// Create the new task with parsed metadata
	newTask := &models.Task{
		ID:         uuid.New().String(),
		ShortID:    m.project.GetNextShortID(),
		Type:       result.taskType,
		Title:      result.title,
		Status:     models.StatusTodo,
		Priority:   result.priority,
		Section:    m.sourceTask.Section, // Inherit section from source by default
		DueDate:    result.dueDate,
		Recurrence: result.recurrence,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		DependsOn:  []string{},
		RelatedTo:  []string{},
		Tags:       []string{},
	}

	// Apply parsed section if explicitly set, otherwise keep inherited
	if result.section != "" {
		newTask.Section = result.section
	}

	// Apply parsed parent if set (for non-subtask link types)
	if result.parentID != nil && m.linkType != LinkTypeSubtask {
		newTask.ParentID = result.parentID
	}

	// Apply tags from parsing
	for _, tag := range result.tags {
		newTask.AddTag(tag)
	}

	// Apply status transition
	switch result.status {
	case models.StatusInProgress:
		newTask.Start()
		if result.activate {
			newTask.Activate()
		}
	case models.StatusBlocked:
		newTask.Block()
	case models.StatusDone:
		newTask.Complete()
	case models.StatusCanceled:
		newTask.Cancel()
	}

	// Establish the relationship based on link type
	switch m.linkType {
	case LinkTypeBlocker:
		// New task blocks source: source depends on new task
		if err := m.sourceTask.AddDependency(newTask.ID); err != nil {
			m.err = err
			return nil
		}

	case LinkTypeDependent:
		// New task depends on source
		if err := newTask.AddDependency(m.sourceTask.ID); err != nil {
			m.err = err
			return nil
		}

	case LinkTypeSubtask:
		// Promote leaf types to Story so they can have subtasks
		if m.sourceTask.Type.IsLeafType() {
			m.project.MutateTask(m.sourceTask.ID, func(t *models.Task) {
				t.Type = models.TaskTypeStory
			})
		}
		// New task is a child of source
		newTask.ParentID = &m.sourceTask.ID

	case LinkTypeFollowUp:
		// Bidirectional relationship + inherit tags
		if err := m.sourceTask.AddRelatedTo(newTask.ID); err != nil {
			m.err = err
			return nil
		}
		if err := newTask.AddRelatedTo(m.sourceTask.ID); err != nil {
			m.err = err
			return nil
		}
		// Copy tags from source task (in addition to any parsed tags)
		for _, tag := range m.sourceTask.Tags {
			newTask.AddTag(tag)
		}
	}

	// Add task to project
	m.project.Tasks = append([]*models.Task{newTask}, m.project.Tasks...)
	m.project.BuildTaskTree()

	// Save with backup and reload
	if err := saveProjectSync(m.project); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)} }
	}

	return func() tea.Msg {
		return LinkedTaskCreatedMsg{
			SourceTask: m.sourceTask,
			NewTask:    newTask,
			Project:    m.project,
			LinkType:   m.linkType,
		}
	}
}

func (m *LinkedTaskCreatorModel) StatusBarLabel() string { return "New Linked Task" }
