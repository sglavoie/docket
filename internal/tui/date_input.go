package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// DateInputModel handles setting a due date for a task.
type DateInputModel struct {
	task     *models.Task
	project  *models.Project
	input    textinput.Model
	err      error
	width    int
	height   int
	canceled bool
}

// NewDateInputModel creates a new date input model.
func NewDateInputModel(task *models.Task, project *models.Project) *DateInputModel {
	input := textinput.New()
	input.Placeholder = "e.g., 2024-01-15, tomorrow, +3d, fri, none"
	input.Focus()
	input.CharLimit = 30
	input.Width = 40

	// Pre-fill with current due date if set
	if task.DueDate != nil {
		input.SetValue(task.DueDate.Format(util.DateFormat))
	}

	return &DateInputModel{
		task:    task,
		project: project,
		input:   input,
	}
}

// SetSize sets the view dimensions.
func (m *DateInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = min(width-20, 40)
}

func (m *DateInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *DateInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.canceled = true
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			value := strings.TrimSpace(m.input.Value())

			// Handle "none" or empty to clear due date
			if value == "" || strings.ToLower(value) == "none" || strings.ToLower(value) == "clear" {
				if !m.project.MutateTask(m.task.ID, func(t *models.Task) {
					t.DueDate = nil
				}) {
					m.err = fmt.Errorf("task no longer exists in project")
					return m, func() tea.Msg { return BackMsg{} }
				}
				return m, m.saveAndReturn()
			}

			// Parse the date
			date, err := ParseDate(value)
			if err != nil {
				m.err = err
				return m, nil
			}

			if !m.project.MutateTask(m.task.ID, func(t *models.Task) {
				t.DueDate = &date
			}) {
				m.err = fmt.Errorf("task no longer exists in project")
				return m, func() tea.Msg { return BackMsg{} }
			}
			return m, m.saveAndReturn()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *DateInputModel) saveAndReturn() tea.Cmd {
	return func() tea.Msg {
		// Save project with backup for undo support
		if err := saveProjectSync(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return DueDateChangedMsg{Task: m.task}
	}
}

func (m *DateInputModel) View() string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Set Due Date for #%d", m.task.ShortID), m.task.Title))
	b.WriteString("\n\n")

	// Current due date
	if m.task.DueDate != nil {
		b.WriteString(FieldLabelStyle.Render("Current: "))
		b.WriteString(m.task.DueDate.Format("2006-01-02 (Monday)"))
		b.WriteString("\n\n")
	}

	// Input field
	b.WriteString(FieldLabelFocusedStyle.Render("New date: "))
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Help text
	help := BoxStyle.Render(
		BoldStyle.Render("Accepted formats") + "\n" +
			"  2024-01-15     Exact date (YYYY-MM-DD)\n" +
			"  today          Today's date\n" +
			"  tomorrow       Tomorrow's date\n" +
			"  +3d            3 days from now\n" +
			"  +1w            1 week from now\n" +
			"  mon, tue, ...  Next occurrence of weekday\n" +
			"  next mon       Next Monday (skips this week)\n" +
			"  in 2 weeks     2 weeks from now\n" +
			"  none           Clear due date\n\n" +
			BoldStyle.Render("Controls") + "\n" +
			"  Enter          Save\n" +
			"  Esc            Cancel",
	)
	b.WriteString(help)

	return b.String()
}

// HasTextInput returns true because the date input is always active.
func (m *DateInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view (cancels via BackMsg).
func (m *DateInputModel) ShouldCaptureEsc() bool { return true }

func (m *DateInputModel) IsFullHeight() bool { return false }

// DueDateChangedMsg is sent when a due date is changed.
type DueDateChangedMsg struct {
	Task *models.Task
}

// ParseDate parses a natural language date string into a time.Time.
// This is a convenience wrapper around util.ParseRelativeDate.
func ParseDate(input string) (time.Time, error) {
	return util.ParseRelativeDate(input)
}

func (m *DateInputModel) StatusBarLabel() string { return "Set Due Date" }
