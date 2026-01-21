package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// QuickCaptureModel handles rapid task creation with inline metadata parsing.
type QuickCaptureModel struct {
	project *models.Project
	input   textarea.Model
	err     error
	width   int
	height  int
}

// quickCaptureResult holds parsed metadata from quick capture input.
type quickCaptureResult struct {
	title      string
	priority   int
	tags       []string
	dueDate    *time.Time
	parentID   *string
	recurrence *models.RecurrencePattern
}

// NewQuickCaptureModel creates a new quick capture model.
func NewQuickCaptureModel(project *models.Project) *QuickCaptureModel {
	input := textarea.New()
	input.Placeholder = "Task title !priority #tag @due ~recur >parent"
	input.CharLimit = 200
	input.SetHeight(3)
	input.ShowLineNumbers = false
	input.SetPromptFunc(0, func(lineIdx int) string { return "" })
	input.Focus()

	return &QuickCaptureModel{
		project: project,
		input:   input,
		width:   80, // sensible default until SetSize is called
	}
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
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseQuickCaptureMsg{} }

		case "enter":
			return m, m.createTask()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *QuickCaptureModel) View() string {
	var b strings.Builder
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	b.WriteString(MutedStyle.Render("!N priority · #tag · @date · ~recur · >parent"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Enter: create · Esc: cancel"))

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

// parseQuickCapture extracts metadata from the input string.
// Syntax: "Task title !1 #backend #urgent @tomorrow >3"
func parseQuickCapture(input string, project *models.Project) (*quickCaptureResult, error) {
	result := &quickCaptureResult{
		priority: 3, // default priority
	}

	// Extract priority: !1 through !5
	priorityRe := regexp.MustCompile(`\s*!([1-5])\s*`)
	if match := priorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		input = priorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags: #word
	tagRe := regexp.MustCompile(`\s*#(\w+)\s*`)
	tagMatches := tagRe.FindAllStringSubmatch(input, -1)
	for _, match := range tagMatches {
		if len(match) > 1 {
			result.tags = append(result.tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	// Extract due date: @value
	dueDateRe := regexp.MustCompile(`\s*@(\S+)\s*`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) > 1 {
		date, err := ParseDate(match[1])
		if err != nil {
			return nil, fmt.Errorf("invalid date: %s", match[1])
		}
		result.dueDate = &date
		input = dueDateRe.ReplaceAllString(input, " ")
	}

	// Extract recurrence: ~pattern (e.g., ~daily, ~weekly, ~2w)
	recurRe := regexp.MustCompile(`\s*~(\S+)\s*`)
	if match := recurRe.FindStringSubmatch(input); len(match) > 1 {
		pattern := models.ParseRecurrence(match[1])
		if pattern == nil {
			return nil, fmt.Errorf("invalid recurrence: %s", match[1])
		}
		result.recurrence = pattern
		input = recurRe.ReplaceAllString(input, " ")
	}

	// Extract parent: >ref (short ID or title prefix)
	parentRe := regexp.MustCompile(`\s*>(\S+)\s*`)
	if match := parentRe.FindStringSubmatch(input); len(match) > 1 {
		ref := match[1]
		parent := resolveParent(ref, project)
		if parent == nil {
			return nil, fmt.Errorf("parent not found: %s", ref)
		}
		if !models.TaskTypeTask.CanBeChildOf(parent.Type) {
			return nil, fmt.Errorf("task cannot be child of %s", parent.Type.HeadingPrefix())
		}
		result.parentID = &parent.ID
		input = parentRe.ReplaceAllString(input, " ")
	}

	// Clean up title
	result.title = strings.TrimSpace(input)
	result.title = regexp.MustCompile(`\s+`).ReplaceAllString(result.title, " ")

	if result.title == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}

	return result, nil
}

// resolveParent finds a task by short ID or title prefix.
func resolveParent(ref string, project *models.Project) *models.Task {
	// Try numeric short ID first
	if shortID, err := strconv.Atoi(ref); err == nil {
		for _, task := range project.Tasks {
			if task.ShortID == shortID {
				return task
			}
		}
	}

	// Fall back to case-insensitive title prefix match
	refLower := strings.ToLower(ref)
	for _, task := range project.Tasks {
		if strings.HasPrefix(strings.ToLower(task.Title), refLower) {
			return task
		}
	}

	return nil
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

		// Ensure Inbox section exists with red color
		if !m.project.HasSection(models.SectionInbox) {
			m.project.EnsureSection(models.SectionInbox)
			m.project.SetSectionColor(models.SectionInbox, "red")
		}

		// Create task
		task := &models.Task{
			ID:         uuid.New().String(),
			ShortID:    m.project.GetNextShortID(),
			Type:       models.TaskTypeTask,
			Title:      result.title,
			Status:     models.StatusTodo,
			Priority:   result.priority,
			Section:    models.SectionInbox,
			ParentID:   result.parentID,
			DueDate:    result.dueDate,
			Recurrence: result.recurrence,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		for _, tag := range result.tags {
			task.AddTag(tag)
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
