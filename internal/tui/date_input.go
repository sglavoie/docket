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

// DateInputModel handles setting a due date for a task.
type DateInputModel struct {
	task         *models.Task
	project      *models.Project
	input        textinput.Model
	err          error
	width        int
	height       int
	canceled     bool
	initialValue string
	weekEndDay   string
	deferApply   bool
	discardConfirmState
}

// NewDateInputModel creates a new date input model.
// weekEndDay controls how "eow" is resolved and must be "fri", "sat", or "sun".
// Pass an empty string to default to "sun".
func NewDateInputModel(task *models.Task, project *models.Project, weekEndDay string) *DateInputModel {
	input := textinput.New()
	input.Placeholder = "e.g., 2024-01-15, dec 15, tomorrow, +3d, fri, none"
	input.Focus()
	input.CharLimit = 30
	input.SetWidth(40)
	applyThemedTextInput(&input)

	// Pre-fill with current due date if set (preserve time component when present)
	if task.DueDate != nil {
		if task.HasTime {
			input.SetValue(task.DueDate.Format(util.DateTimeDueFormat))
		} else {
			input.SetValue(task.DueDate.Format(util.DateFormat))
		}
	}

	if weekEndDay == "" {
		weekEndDay = "sun"
	}
	return &DateInputModel{
		task:         task,
		project:      project,
		input:        input,
		initialValue: input.Value(),
		weekEndDay:   weekEndDay,
	}
}

// SetSize sets the view dimensions.
func (m *DateInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.SetWidth(min(width-20, 40))
}

func (m *DateInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *DateInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.canceled = true
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			value := strings.TrimSpace(m.input.Value())

			// Handle "none" or empty to clear due date
			if value == "" || strings.ToLower(value) == "none" || strings.ToLower(value) == "clear" {
				if m.deferApply {
					return m, dueDateChangedCmd(m.task, m.project, nil, false, true)
				}
				if !m.project.MutateTask(m.task.ID, func(t *models.Task) {
					t.DueDate = nil
					t.HasTime = false
				}) {
					m.err = fmt.Errorf("task no longer exists in project")
					return m, func() tea.Msg { return BackMsg{} }
				}
				return m, m.saveAndReturn()
			}

			// Parse the date (supports optional time-of-day suffix like "5pm")
			date, hasTime, err := util.ParseRelativeDateWithTime(value, m.weekEndDay)
			if err != nil {
				m.err = err
				return m, nil
			}

			if m.deferApply {
				return m, dueDateChangedCmd(m.task, m.project, &date, hasTime, true)
			}
			if !m.project.MutateTask(m.task.ID, func(t *models.Task) {
				t.DueDate = &date
				t.HasTime = hasTime
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
	project := m.project
	task := m.task
	dueDate := task.DueDate
	hasTime := task.HasTime
	explicit := true
	return func() tea.Msg {
		// Save project with backup for undo support
		if err := saveProjectSync(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return DueDateChangedMsg{Task: task, Project: project, DueDate: dueDate, HasTime: hasTime, HasExplicitValue: explicit}
	}
}

func dueDateChangedCmd(task *models.Task, project *models.Project, dueDate *time.Time, hasTime, explicit bool) tea.Cmd {
	var dueDateCopy *time.Time
	if dueDate != nil {
		d := *dueDate
		dueDateCopy = &d
	}
	return func() tea.Msg {
		return DueDateChangedMsg{
			Task:             task,
			Project:          project,
			DueDate:          dueDateCopy,
			HasTime:          hasTime,
			HasExplicitValue: explicit,
		}
	}
}

func (m *DateInputModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Set Due Date for #%d", m.task.ShortID), m.task.Title))
	b.WriteString("\n\n")

	// Current due date
	if m.task.DueDate != nil {
		b.WriteString(FieldLabelStyle.Render("Current: "))
		dateStr := m.task.DueDate.Format("2006-01-02 (Monday)")
		if m.task.HasTime {
			dateStr += " " + m.task.DueDate.Format("15:04")
		}
		b.WriteString(dateStr)
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

	// Help text. The box is clamped against the remaining terminal height so it
	// degrades gracefully (collapsing to "… N more") instead of overflowing on
	// short screens; the fixed-size render otherwise pushes past the terminal.
	bodyLines := []string{
		BoldStyle.Render("Accepted formats"),
		"  2024-01-15     Exact date (YYYY-MM-DD)",
		"  today          Today's date",
		"  tomorrow       Tomorrow's date",
		"  +3d            3 days from now",
		"  +1w            1 week from now",
		"  mon, tue, ...  Next occurrence of weekday",
		"  next mon       Next Monday (skips this week)",
		"  in 2 weeks     2 weeks from now",
		"  dec 15         Dec 15 (next year if past)",
		"  next year      1 year from today",
		"  in 3 years     3 years from today",
		"  none           Clear due date",
		"",
		BoldStyle.Render("Controls"),
		"  Enter          Save",
		"  Esc            Cancel",
	}
	if help := clampHelpBoxToHeight(bodyLines, m.height, b.String()); help != "" {
		b.WriteString(help)
	} else {
		return tea.NewView(strings.TrimRight(b.String(), "\n"))
	}

	return tea.NewView(b.String())
}

// HasTextInput returns true because the date input is always active.
func (m *DateInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view (cancels via BackMsg).
func (m *DateInputModel) ShouldCaptureEsc() bool { return true }

func (m *DateInputModel) IsFullHeight() bool { return false }

// DueDateChangedMsg is sent when a due date is changed.
type DueDateChangedMsg struct {
	Task             *models.Task
	Project          *models.Project
	DueDate          *time.Time
	HasTime          bool
	HasExplicitValue bool
}

// ParseDate parses a natural language date string into a time.Time.
// This is a convenience wrapper around util.ParseRelativeDate.
func ParseDate(input string) (time.Time, error) {
	return util.ParseRelativeDate(input)
}

// ParseFlexibleDate is like ParseDate but also accepts bare shorthand without
// the leading +: "3d" (3 days), "2w" (2 weeks), "1m" (1 month).
func ParseFlexibleDate(input string) (time.Time, error) {
	return util.ParseFlexibleDate(input)
}

func (m *DateInputModel) StatusBarLabel() string { return "Set Due Date" }
