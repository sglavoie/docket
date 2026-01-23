package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// DateInputModel handles setting a due date for a task.
type DateInputModel struct {
	task      *models.Task
	project   *models.Project
	input     textinput.Model
	err       error
	width     int
	height    int
	cancelled bool
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
			m.cancelled = true
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			value := strings.TrimSpace(m.input.Value())

			// Handle "none" or empty to clear due date
			if value == "" || strings.ToLower(value) == "none" || strings.ToLower(value) == "clear" {
				m.task.DueDate = nil
				m.task.UpdatedAt = time.Now()
				return m, m.saveAndReturn()
			}

			// Parse the date
			date, err := ParseDate(value)
			if err != nil {
				m.err = err
				return m, nil
			}

			m.task.DueDate = &date
			m.task.UpdatedAt = time.Now()
			return m, m.saveAndReturn()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *DateInputModel) saveAndReturn() tea.Cmd {
	return func() tea.Msg {
		// Create backup before saving for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		storage.LoadProject(m.project)
		return DueDateChangedMsg{Task: m.task}
	}
}

func (m *DateInputModel) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render(fmt.Sprintf("Set Due Date for #%d", m.task.ShortID)))
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render(m.task.Title))
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
			"  none           Clear due date\n\n" +
			BoldStyle.Render("Controls") + "\n" +
			"  Enter          Save\n" +
			"  Esc            Cancel",
	)
	b.WriteString(help)

	return b.String()
}

// DueDateChangedMsg is sent when a due date is changed.
type DueDateChangedMsg struct {
	Task *models.Task
}

// ParseDate parses a natural language date string into a time.Time.
// Supported formats:
//   - YYYY-MM-DD (e.g., 2024-01-15)
//   - today, tomorrow
//   - +Nd (N days from now)
//   - +Nw (N weeks from now)
//   - mon, tue, wed, thu, fri, sat, sun (next occurrence)
//   - monday, tuesday, etc. (next occurrence)
func ParseDate(input string) (time.Time, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Exact date: YYYY-MM-DD
	if date, err := time.ParseInLocation(util.DateFormat, input, time.Local); err == nil {
		return date, nil
	}

	// Exact date: MM/DD/YYYY
	if date, err := time.ParseInLocation("01/02/2006", input, time.Local); err == nil {
		return date, nil
	}

	// Exact date: DD.MM.YYYY (European format)
	if date, err := time.ParseInLocation("02.01.2006", input, time.Local); err == nil {
		return date, nil
	}

	// Keywords
	switch input {
	case "today":
		return today, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	}

	// Relative: +Nd (days)
	if match := regexp.MustCompile(`^\+(\d+)d$`).FindStringSubmatch(input); len(match) == 2 {
		days, _ := strconv.Atoi(match[1])
		return today.AddDate(0, 0, days), nil
	}

	// Relative: +Nw (weeks)
	if match := regexp.MustCompile(`^\+(\d+)w$`).FindStringSubmatch(input); len(match) == 2 {
		weeks, _ := strconv.Atoi(match[1])
		return today.AddDate(0, 0, weeks*7), nil
	}

	// Relative: +Nm (months)
	if match := regexp.MustCompile(`^\+(\d+)m$`).FindStringSubmatch(input); len(match) == 2 {
		months, _ := strconv.Atoi(match[1])
		return today.AddDate(0, months, 0), nil
	}

	// Weekday names
	weekdays := map[string]time.Weekday{
		"sun": time.Sunday, "sunday": time.Sunday,
		"mon": time.Monday, "monday": time.Monday,
		"tue": time.Tuesday, "tuesday": time.Tuesday,
		"wed": time.Wednesday, "wednesday": time.Wednesday,
		"thu": time.Thursday, "thursday": time.Thursday,
		"fri": time.Friday, "friday": time.Friday,
		"sat": time.Saturday, "saturday": time.Saturday,
	}

	if targetDay, ok := weekdays[input]; ok {
		// Find next occurrence of this weekday
		daysUntil := int(targetDay) - int(today.Weekday())
		if daysUntil <= 0 {
			daysUntil += 7
		}
		return today.AddDate(0, 0, daysUntil), nil
	}

	// "next week" / "next month"
	if input == "next week" {
		return today.AddDate(0, 0, 7), nil
	}
	if input == "next month" {
		return today.AddDate(0, 1, 0), nil
	}

	return time.Time{}, fmt.Errorf("unrecognized date format: %s", input)
}
