package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// BulkCaptureModel handles rapid multi-task creation with inline metadata parsing.
// Each line is parsed using the same syntax as quick capture.
type BulkCaptureModel struct {
	project *models.Project
	capture CaptureCoordinator
	input   textarea.Model
	errors  []lineError
	created int
	width   int
	height  int
	discardConfirmState
}

// SetCaptureCoordinator injects the shared capture coordinator (captureWireable).
func (m *BulkCaptureModel) SetCaptureCoordinator(c CaptureCoordinator) { m.capture = c }

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
	// Parsing runs synchronously so failed lines can be surfaced and retained in
	// the input; the creation itself commits through the application layer
	// (CaptureCoordinator.CaptureBulk) as one atomic ChangeSet.

	content := strings.TrimSpace(m.input.Value())
	if content == "" {
		m.errors = []lineError{{line: 1, message: "no tasks to create"}}
		return nil
	}

	lines := strings.Split(content, "\n")
	var reqs []app.CaptureRequest
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
			continue
		}

		// Determine target section. Non-GTD section input is preserved as a tag.
		tags := result.tags
		targetSection := result.section
		if targetSection == "" {
			targetSection = models.SectionInbox
		} else if !models.IsGTDSection(targetSection) {
			tags = append(tags, targetSection)
			targetSection = models.SectionInbox
		}

		reqs = append(reqs, app.CaptureRequest{
			ProjectID:  m.project.ID,
			Title:      result.title,
			Type:       result.taskType,
			Priority:   result.priority,
			Tags:       tags,
			Section:    targetSection,
			Due:        result.dueDate,
			ParentID:   result.parentID,
			Recurrence: result.recurrence,
			State:      captureStateForStatus(result.status),
			Prepend:    true,
		})
	}

	// Update textarea with only failed lines and show errors if any.
	closeOnSuccess := len(errors) == 0
	if len(errors) > 0 {
		m.errors = errors
		m.input.SetValue(strings.Join(failedLines, "\n"))
	}

	if len(reqs) == 0 {
		return nil
	}

	// Optimistically bump the created count for the View; a rejected commit
	// surfaces an ErrorMsg (matching the legacy save-failure path, which also
	// bumped the count before saving).
	m.created += len(reqs)
	return m.capture.CaptureBulk(reqs, closeOnSuccess)
}

func (m *BulkCaptureModel) StatusBarLabel() string { return "Bulk Capture" }
