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

// BulkCaptureModel handles rapid multi-task creation with inline metadata parsing.
// Each line is parsed using the same syntax as quick capture.
type BulkCaptureModel struct {
	project *models.Project
	input   textarea.Model
	errors  []lineError
	created int
	width   int
	height  int
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
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseBulkCaptureMsg{} }

		case "ctrl+s":
			return m, m.createTasks()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *BulkCaptureModel) View() string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Bulk Capture", ""))
	b.WriteString("\n\n")

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

	return b.String()
}

// HasTextInput returns true because bulk capture uses a textarea input.
func (m *BulkCaptureModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns false so the global handler closes the view on Esc.
func (m *BulkCaptureModel) ShouldCaptureEsc() bool {
	return false
}

func (m *BulkCaptureModel) createTasks() tea.Cmd {
	return func() tea.Msg {
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

			result, err := parseQuickCapture(line, m.project, nil)
			if err != nil {
				errors = append(errors, lineError{
					line:    len(failedLines) + 1, // Line number in the remaining content
					message: err.Error(),
				})
				failedLines = append(failedLines, lines[i]) // Keep original line
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
				task := &models.Task{
					ID:        uuid.New().String(),
					ShortID:   m.project.GetNextShortID(),
					Type:      result.taskType,
					Title:     result.title,
					Status:    models.StatusTodo,
					Priority:  result.priority,
					Section:   models.SectionInbox,
					ParentID:  result.parentID,
					DueDate:   result.dueDate,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				for _, tag := range result.tags {
					task.AddTag(tag)
				}

				newTasks = append(newTasks, task)
			}

			// Prepend all new tasks at once (newest first)
			m.project.Tasks = append(newTasks, m.project.Tasks...)
			m.project.BuildTaskTree()
			m.project.ReorderRootTasksByCompletion()

			// Update created count
			m.created += len(validResults)

			// Backup and save
			if err := storage.BackupProject(m.project); err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
			}
			if err := storage.SaveProject(m.project); err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
			}

			// Reload to rebuild tree properly
			_ = storage.LoadProject(m.project)
		}

		// Update textarea with only failed lines and show errors if any
		if len(errors) > 0 {
			m.errors = errors
			m.input.SetValue(strings.Join(failedLines, "\n"))
			return nil
		}

		// All succeeded, close the view
		return TaskCreatedMsg{Task: nil}
	}
}
