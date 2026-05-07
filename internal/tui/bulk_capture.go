package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// BulkCaptureModel handles rapid multi-task creation with inline metadata parsing.
// Each line is parsed using the same syntax as quick capture.
type BulkCaptureModel struct {
	project *models.Project
	input   textarea.Model
	errors  []lineError
	created int
	width   int
	height  int
	discardConfirmState
}

// lineError represents a parsing error for a specific line.
type lineError struct {
	line    int
	message string
}

// NewBulkCaptureModel creates a new bulk capture model.
func NewBulkCaptureModel(project *models.Project) *BulkCaptureModel {
	input := textarea.New()
	input.Placeholder = "Enter tasks, one per line\n^type !priority #tag @due >parent"
	input.Focus()
	input.CharLimit = 2000
	input.SetWidth(60)
	input.SetHeight(8)
	input.ShowLineNumbers = false
	applyThemedTextarea(&input)

	return &BulkCaptureModel{
		project: project,
		input:   input,
	}
}

// SetSize sets the view dimensions.
func (m *BulkCaptureModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	inputWidth := min(width-10, 70)
	inputWidth = max(inputWidth, 40)
	m.input.SetWidth(inputWidth)
}

func (m *BulkCaptureModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *BulkCaptureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle discard confirmation
	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return CloseBulkCaptureMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.input.Value()) != "" {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg { return CloseBulkCaptureMsg{} }

		case "ctrl+s":
			return m, m.createTasks()

		case "shift+enter":
			m.input.InsertString("\n")
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *BulkCaptureModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Bulk Capture", ""))
	b.WriteString("\n\n")

	applyThemedTextarea(&m.input)
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	if m.created > 0 {
		successMsg := fmt.Sprintf("Created %d task", m.created)
		if m.created > 1 {
			successMsg += "s"
		}
		b.WriteString(SuccessStyle.Render(successMsg))
		b.WriteString("\n\n")
	}

	if len(m.errors) > 0 {
		for _, err := range m.errors {
			errMsg := fmt.Sprintf("Line %d: %s", err.line, err.message)
			b.WriteString(ValidationErrorStyle.Render(errMsg))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	hint := MutedStyle.Render("One task per line: ^type  !N priority  #tag  @date  >parent")
	b.WriteString(hint)
	b.WriteString("\n")

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Ctrl+S: create"), HelpStyle.Render("Esc: cancel"), m.width))

	return tea.NewView(b.String())
}

// HasTextInput returns true because bulk capture uses a textarea input.
func (m *BulkCaptureModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true so the view always handles Esc internally.
func (m *BulkCaptureModel) ShouldCaptureEsc() bool {
	return true
}

func (m *BulkCaptureModel) IsFullHeight() bool { return false }

func (m *BulkCaptureModel) createTasks() tea.Cmd {
	// All mutations run synchronously in the Update() goroutine to avoid
	// data races with View() reading project.Tasks concurrently.

	content := strings.TrimSpace(m.input.Value())
	if content == "" {
		m.errors = []lineError{{line: 1, message: "no tasks to create"}}
		return nil
	}

	lines := strings.Split(content, "\n")
	var validResults []*quickCaptureResult
	var failedLines []string
	var errors []lineError

	// Parse all lines
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		result, err := parseQuickCapture(line, m.project, nil, true)
		if err != nil {
			errors = append(errors, lineError{
				line:    len(failedLines) + 1, // Line number in the remaining content
				message: err.Error(),
			})
			failedLines = append(failedLines, lines[i]) // Keep original line
			continue
		}

		// Validate title to prevent storage corruption from meta-characters
		if valErr := models.ValidateTaskTitle(result.title); valErr != nil {
			errors = append(errors, lineError{
				line:    len(failedLines) + 1,
				message: valErr.Error(),
			})
			failedLines = append(failedLines, lines[i])
		} else {
			validResults = append(validResults, result)
		}
	}

	// If we have valid tasks, create them
	if len(validResults) > 0 {
		// Ensure Inbox section exists with red color
		if !m.project.HasSection(models.SectionInbox) {
			m.project.EnsureSection(models.SectionInbox)
			m.project.SetSectionColor(models.SectionInbox, "red")
		}

		// Create all valid tasks
		var newTasks []*models.Task
		for _, result := range validResults {
			// Determine target section and custom section
			targetSection := result.section
			var customSection string
			if targetSection == "" {
				targetSection = models.SectionInbox
			} else if !models.IsGTDSection(targetSection) {
				customSection = targetSection
				targetSection = models.SectionInbox
			}

			// Ensure target section exists
			if targetSection != models.SectionInbox && !m.project.HasSection(targetSection) {
				m.project.EnsureSection(targetSection)
			}
			if customSection != "" && !m.project.HasSection(customSection) {
				m.project.EnsureSection(customSection)
			}

			task := &models.Task{
				ID:              uuid.New().String(),
				ShortID:         m.project.GetNextShortID(),
				Type:            result.taskType,
				Title:           result.title,
				Status:          models.StatusTodo,
				Priority:        result.priority,
				Section:         targetSection,
				CustomSection:   customSection,
				ParentID:        result.parentID,
				DueDate:         result.dueDate,
				Recurrence:      result.recurrence,
				DependsOn:       []string{},
				RelatedTo:       []string{},
				ScoreDimensions: map[string]int{},
				Children:        []*models.Task{},
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}

			for _, tag := range result.tags {
				task.AddTag(tag)
			}

			// Apply status transition using Task methods (handles auto-triage)
			switch result.status {
			case models.StatusInProgress:
				task.Start()
				if result.activate {
					task.Activate()
				}
			case models.StatusBlocked:
				task.Block()
			case models.StatusDone:
				task.Complete()
			case models.StatusCanceled:
				task.Cancel()
			case models.StatusReview:
				task.Review()
			case models.StatusStandby:
				task.Standby()
			case models.StatusMaybe:
				task.Maybe()
			default:
			}

			newTasks = append(newTasks, task)
		}

		// Prepend all new tasks at once (newest first)
		m.project.Tasks = append(newTasks, m.project.Tasks...)
		m.project.BuildTaskTree()
		m.project.ReorderRootTasksByCompletion()

		// Update created count
		m.created += len(validResults)

		// Save with backup and reload
		if err := saveProjectSync(m.project); err != nil {
			return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)} }
		}
	}

	// Update textarea with only failed lines and show errors if any
	if len(errors) > 0 {
		m.errors = errors
		m.input.SetValue(strings.Join(failedLines, "\n"))
		return nil
	}

	// All succeeded, close the view
	return func() tea.Msg { return TaskCreatedMsg{Task: nil} }
}

func (m *BulkCaptureModel) StatusBarLabel() string { return "Bulk Capture" }
