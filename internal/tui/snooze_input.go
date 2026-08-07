package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// SnoozeInputModel handles setting a snooze date for a task.
type SnoozeInputModel struct {
	workspacePulseState
	task         *models.Task
	project      *models.Project
	input        textinput.Model
	err          error
	width        int
	height       int
	initialValue string
	discardConfirmState
}

// NewSnoozeInputModel creates a new snooze input model.
func NewSnoozeInputModel(task *models.Task, project *models.Project) *SnoozeInputModel {
	input := textinput.New()
	input.Placeholder = "e.g., YYYY-MM-DD, tomorrow, +3d, +1w, none"
	input.Focus()
	input.CharLimit = 30
	input.SetWidth(40)
	applyThemedTextInput(&input)

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
	m.input.SetWidth(min(width-20, 40))
}

func (m *SnoozeInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SnoozeInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				return m, snoozeDateChangedCmd(m.task, m.project, nil, true)
			}

			// Parse the date
			date, err := ParseDate(value)
			if err != nil {
				m.err = err
				return m, nil
			}

			return m, snoozeDateChangedCmd(m.task, m.project, &date, true)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func snoozeDateChangedCmd(task *models.Task, project *models.Project, snoozedUntil *time.Time, explicit bool) tea.Cmd {
	var snoozeCopy *time.Time
	if snoozedUntil != nil {
		d := *snoozedUntil
		snoozeCopy = &d
	}
	return func() tea.Msg {
		return SnoozeDateChangedMsg{
			Task:             task,
			Project:          project,
			SnoozedUntil:     snoozeCopy,
			HasExplicitValue: explicit,
		}
	}
}

func (m *SnoozeInputModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderTaskScopedFullscreenTitleWithPulse(
		fmt.Sprintf("Snooze Task #%d  ", m.task.ShortID),
		m.task,
		"",
		m.currentWorkspacePulse(),
		m.width))
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

	guidance := renderGuidance(m.guidanceSpec(), m.width)

	// Help text. The box is clamped against the remaining terminal height so it
	// degrades gracefully (collapsing to "… N more") instead of overflowing on
	// short screens; the fixed-size render otherwise pushes past the terminal.
	bodyLines := []string{
		BoldStyle.Render("Accepted formats"),
		"  YYYY-MM-DD     Exact date",
		"  today          Today's date",
		"  tomorrow       Tomorrow's date",
		"  +3d            3 days from now",
		"  +1w            1 week from now",
		"  +1m            1 month from now",
		"  mon, tue, ...  Next occurrence of weekday",
		"  dec 15         Dec 15 (next year if past)",
		"  next year      1 year from today",
		"  none           Clear snooze",
	}
	if help := clampHelpBoxToHeight(bodyLines, m.height, b.String(), guidance); help != "" {
		b.WriteString(help)
	} else {
		trimmed := strings.TrimRight(b.String(), "\n")
		b.Reset()
		b.WriteString(trimmed)
	}
	b.WriteString("\n")
	b.WriteString(guidance)

	return tea.NewView(b.String())
}

func (m *SnoozeInputModel) guidanceSpec() GuidanceSpec {
	if m.confirmDiscard {
		return confirmationGuidanceSpec()
	}
	commit := "Enter save"
	if m.err != nil {
		commit = "Enter retry"
	}
	return requiredActionGuidanceSpec(commit)
}

// HasTextInput returns true because the snooze input is always active.
func (m *SnoozeInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view (cancels via BackMsg).
func (m *SnoozeInputModel) ShouldCaptureEsc() bool { return true }

func (m *SnoozeInputModel) IsFullHeight() bool { return false }

// SnoozeDateChangedMsg is sent when a snooze date is changed.
type SnoozeDateChangedMsg struct {
	Task             *models.Task
	Project          *models.Project
	SnoozedUntil     *time.Time
	HasExplicitValue bool
}

func (m *SnoozeInputModel) StatusBarLabel() string { return "Snooze" }
