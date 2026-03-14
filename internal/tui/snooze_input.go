package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// SnoozeInputModel handles setting a snooze date for a task.
type SnoozeInputModel struct {
	task           *models.Task
	project        *models.Project
	input          textinput.Model
	err            error
	width          int
	height         int
	initialValue   string
	confirmDiscard bool
	confirmOverlay ConfirmDialog
}

// NewSnoozeInputModel creates a new snooze input model.
func NewSnoozeInputModel(task *models.Task, project *models.Project) *SnoozeInputModel {
	input := textinput.New()
	input.Placeholder = "e.g., YYYY-MM-DD, tomorrow, +3d, +1w, none"
	input.Focus()
	input.CharLimit = 30
	input.Width = 40

	// Pre-fill with current snooze date if set
	if task.SnoozedUntil != nil {
		input.SetValue(task.SnoozedUntil.Format(util.DateFormat))
	}

	return &SnoozeInputModel{
		task:         task,
		project:      project,
		input:        input,
		initialValue: input.Value(),
	}
}

// SetSize sets the view dimensions.
func (m *SnoozeInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = min(width-20, 40)
}

func (m *SnoozeInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SnoozeInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyMsg); ok {
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
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.input.Value() != m.initialValue {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			value := strings.TrimSpace(m.input.Value())

			// Handle "none" or empty to clear snooze
			if value == "" || strings.ToLower(value) == "none" || strings.ToLower(value) == "clear" {
				if !m.project.MutateTask(m.task.ID, func(t *models.Task) {
					t.SnoozedUntil = nil
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
				t.SnoozedUntil = &date
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

func (m *SnoozeInputModel) saveAndReturn() tea.Cmd {
	return func() tea.Msg {
		if err := saveProjectSync(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return SnoozeDateChangedMsg{Task: m.task}
	}
}

func (m *SnoozeInputModel) View() string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Snooze Task #%d", m.task.ShortID), m.task.Title))
	b.WriteString("\n\n")

	// Current snooze date
	if m.task.SnoozedUntil != nil {
		b.WriteString(FieldLabelStyle.Render("Current: "))
		b.WriteString(m.task.SnoozedUntil.Format("2006-01-02 (Monday)"))
		b.WriteString("\n\n")
	}

	// Input field
	b.WriteString(FieldLabelFocusedStyle.Render("Snooze until: "))
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
			"  YYYY-MM-DD     Exact date\n" +
			"  today          Today's date\n" +
			"  tomorrow       Tomorrow's date\n" +
			"  +3d            3 days from now\n" +
			"  +1w            1 week from now\n" +
			"  +1m            1 month from now\n" +
			"  mon, tue, ...  Next occurrence of weekday\n" +
			"  none           Clear snooze\n\n" +
			BoldStyle.Render("Controls") + "\n" +
			"  Enter          Save\n" +
			"  Esc            Cancel",
	)
	b.WriteString(help)

	return b.String()
}

// HasTextInput returns true because the snooze input is always active.
func (m *SnoozeInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view (cancels via BackMsg).
func (m *SnoozeInputModel) ShouldCaptureEsc() bool { return true }

func (m *SnoozeInputModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}

func (m *SnoozeInputModel) IsFullHeight() bool { return false }

// SnoozeDateChangedMsg is sent when a snooze date is changed.
type SnoozeDateChangedMsg struct {
	Task *models.Task
}

func (m *SnoozeInputModel) StatusBarLabel() string { return "Snooze" }
