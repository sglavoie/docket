package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// LinkedTaskCreatorModel handles creation of tasks linked to a source task.
type LinkedTaskCreatorModel struct {
	sourceTask *models.Task
	project    *models.Project
	linkType   LinkType
	input      textarea.Model
	err        error
	width      int
	height     int
}

// NewLinkedTaskCreatorModel creates a new linked task creator.
func NewLinkedTaskCreatorModel(sourceTask *models.Task, project *models.Project, linkType LinkType) *LinkedTaskCreatorModel {
	input := textarea.New()
	input.Placeholder = "Enter task title..."
	input.CharLimit = 200
	input.SetHeight(2)
	input.ShowLineNumbers = false
	input.SetPromptFunc(0, func(lineIdx int) string { return "" })
	input.Focus()

	return &LinkedTaskCreatorModel{
		sourceTask: sourceTask,
		project:    project,
		linkType:   linkType,
		input:      input,
		width:      80,
	}
}

// SetSize sets the view dimensions.
func (m *LinkedTaskCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextarea(&m.input, width)
}

func (m *LinkedTaskCreatorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *LinkedTaskCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		switch key {
		case "esc":
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			return m, m.createTask()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *LinkedTaskCreatorModel) View() string {
	var b strings.Builder

	// Header showing link type and source task
	linkName := m.linkTypeName()
	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Create %s for #%d", linkName, m.sourceTask.ShortID)))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(truncateTitle(m.sourceTask.Title, 50)))
	b.WriteString("\n\n")

	// Input field
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Error display
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Enter: create · Esc: cancel"))

	cardWidth := m.width - 4
	if cardWidth > 70 {
		cardWidth = 70
	}
	config := OverlayCardConfig{
		Title:        fmt.Sprintf("New %s", linkName),
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, config)
}

// HasTextInput returns true because this view uses a textarea.
func (m *LinkedTaskCreatorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns false so the app can close the view on esc.
func (m *LinkedTaskCreatorModel) ShouldCaptureEsc() bool {
	return false
}

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
	return func() tea.Msg {
		title := strings.ReplaceAll(m.input.Value(), "\n", " ")
		title = strings.TrimSpace(title)
		if title == "" {
			m.err = fmt.Errorf("task title cannot be empty")
			return nil
		}

		// Determine task type based on link type
		taskType := models.TaskTypeTask
		if m.linkType == LinkTypeSubtask {
			// Subtasks default to task type
			taskType = models.TaskTypeTask
		}

		// Create the new task
		newTask := &models.Task{
			ID:        uuid.New().String(),
			ShortID:   m.project.GetNextShortID(),
			Type:      taskType,
			Title:     title,
			Status:    models.StatusTodo,
			Priority:  models.PriorityDefault,
			Section:   m.sourceTask.Section, // Inherit section from source
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			DependsOn: []string{},
			RelatedTo: []string{},
			Tags:      []string{},
		}

		// Establish the relationship based on link type
		switch m.linkType {
		case LinkTypeBlocker:
			// New task blocks source: source depends on new task
			m.sourceTask.AddDependency(newTask.ID)

		case LinkTypeDependent:
			// New task depends on source
			newTask.AddDependency(m.sourceTask.ID)

		case LinkTypeSubtask:
			// Promote leaf types to Story so they can have subtasks
			if m.sourceTask.Type.IsLeafType() && !m.sourceTask.Type.IsNoteType() {
				m.sourceTask.Type = models.TaskTypeStory
				m.sourceTask.UpdatedAt = time.Now()
			}
			// New task is a child of source
			newTask.ParentID = &m.sourceTask.ID

		case LinkTypeFollowUp:
			// Bidirectional relationship + inherit tags
			m.sourceTask.AddRelatedTo(newTask.ID)
			newTask.AddRelatedTo(m.sourceTask.ID)
			// Copy tags from source task
			for _, tag := range m.sourceTask.Tags {
				newTask.AddTag(tag)
			}
		}

		// Add task to project
		m.project.Tasks = append([]*models.Task{newTask}, m.project.Tasks...)
		m.project.BuildTaskTree()

		// Backup and save
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reload to rebuild tree properly
		_ = storage.LoadProject(m.project)

		return LinkedTaskCreatedMsg{
			SourceTask: m.sourceTask,
			NewTask:    newTask,
			Project:    m.project,
			LinkType:   m.linkType,
		}
	}
}

// truncateTitle truncates a title to the specified length with ellipsis.
func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	return title[:maxLen-3] + "..."
}
