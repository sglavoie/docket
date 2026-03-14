package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// RecurrenceEditorModel handles editing a custom recurrence pattern.
type RecurrenceEditorModel struct {
	// Pattern components
	interval   int          // 1-99
	unitCursor int          // 0=day, 1=week, 2=month, 3=year
	weekdays   map[int]bool // 0=Sun, 1=Mon, ..., 6=Sat (only for weekly)
	units      []models.RecurrenceUnit

	// For preview calculation
	dueDate *time.Time
	streak  int // Current streak from existing pattern (display only)

	// Focus: 0=interval, 1=unit, 2=weekdays (weekday cursor when focused)
	focusSection  int
	weekdayCursor int // 0-6 when in weekdays section

	// intervalEditing tracks if user has started typing digits (first digit replaces, subsequent append)
	intervalEditing bool

	width  int
	height int
}

// NewRecurrenceEditorModel creates a new recurrence editor.
func NewRecurrenceEditorModel(current *models.RecurrencePattern, dueDate *time.Time) *RecurrenceEditorModel {
	m := &RecurrenceEditorModel{
		interval: 1,
		units: []models.RecurrenceUnit{
			models.RecurrenceDaily,
			models.RecurrenceWeekly,
			models.RecurrenceMonthly,
			models.RecurrenceYearly,
		},
		unitCursor:    0,
		weekdays:      make(map[int]bool),
		dueDate:       dueDate,
		focusSection:  0,
		weekdayCursor: 0,
		width:         80,
		height:        24,
	}

	// Load from existing pattern if provided
	if current != nil {
		m.interval = min(max(current.Interval, 1), 99)

		// Find unit cursor
		for i, u := range m.units {
			if u == current.Unit {
				m.unitCursor = i
				break
			}
		}

		// Load weekdays
		for _, wd := range current.Weekdays {
			if wd >= 0 && wd < 7 {
				m.weekdays[wd] = true
			}
		}

		m.streak = current.Streak
	}

	return m
}

// SetSize sets the view dimensions.
func (m *RecurrenceEditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *RecurrenceEditorModel) Init() tea.Cmd {
	return nil
}

func (m *RecurrenceEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return RecurrenceEditorDoneMsg{Pattern: nil, Canceled: true}
			}

		case "enter":
			// Confirm and return pattern
			pattern := m.buildPattern()
			return m, func() tea.Msg {
				return RecurrenceEditorDoneMsg{Pattern: pattern, Canceled: false}
			}

		case "tab":
			m.focusSection = m.nextFocusSection()
			m.intervalEditing = false // Reset editing state when leaving interval
			return m, nil

		case "shift+tab":
			m.focusSection = m.prevFocusSection()
			m.intervalEditing = false // Reset editing state when leaving interval
			return m, nil

		case "left", "h":
			return m, m.handleLeft()

		case "right", "l":
			return m, m.handleRight()

		case "up", "k":
			return m, m.handleUp()

		case "down", "j":
			return m, m.handleDown()

		case " ":
			// Toggle weekday
			if m.focusSection == 2 && m.isWeeklyUnit() {
				m.weekdays[m.weekdayCursor] = !m.weekdays[m.weekdayCursor]
			}
			return m, nil

		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if m.focusSection == 0 {
				digit := int(msg.String()[0] - '0')
				if !m.intervalEditing {
					// First digit clears existing value and starts fresh
					m.interval = digit
					m.intervalEditing = true
				} else {
					// Subsequent digits append
					newInterval := m.interval*10 + digit
					if newInterval <= 99 {
						m.interval = newInterval
					}
				}
			}
			return m, nil

		case "backspace":
			if m.focusSection == 0 {
				// Remove last digit
				m.interval = m.interval / 10
				if m.interval == 0 {
					// Allow 0 temporarily to enable fresh input
					m.intervalEditing = false
				}
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *RecurrenceEditorModel) isWeeklyUnit() bool {
	return m.units[m.unitCursor] == models.RecurrenceWeekly
}

func (m *RecurrenceEditorModel) nextFocusSection() int {
	maxSection := 1 // interval, unit
	if m.isWeeklyUnit() {
		maxSection = 2 // + weekdays
	}
	next := m.focusSection + 1
	if next > maxSection {
		next = 0
	}
	return next
}

func (m *RecurrenceEditorModel) prevFocusSection() int {
	maxSection := 1
	if m.isWeeklyUnit() {
		maxSection = 2
	}
	prev := m.focusSection - 1
	if prev < 0 {
		prev = maxSection
	}
	return prev
}

func (m *RecurrenceEditorModel) handleLeft() tea.Cmd {
	switch m.focusSection {
	case 0: // interval - no left/right navigation
	case 1: // unit selector
		if m.unitCursor > 0 {
			m.unitCursor--
			// Reset weekdays section focus if no longer weekly
			if !m.isWeeklyUnit() && m.focusSection == 2 {
				m.focusSection = 1
			}
		}
	case 2: // weekdays
		if m.weekdayCursor > 0 {
			m.weekdayCursor--
		}
	}
	return nil
}

func (m *RecurrenceEditorModel) handleRight() tea.Cmd {
	switch m.focusSection {
	case 0: // interval
	case 1: // unit selector
		if m.unitCursor < len(m.units)-1 {
			m.unitCursor++
		}
	case 2: // weekdays
		if m.weekdayCursor < 6 {
			m.weekdayCursor++
		}
	}
	return nil
}

func (m *RecurrenceEditorModel) handleUp() tea.Cmd {
	switch m.focusSection {
	case 0: // interval - increment
		if m.interval < 99 {
			m.interval++
			m.intervalEditing = true
		}
	case 2: // weekdays
		if m.weekdayCursor >= 4 {
			// Move from bottom row to top row
			m.weekdayCursor -= 4
		}
	}
	return nil
}

func (m *RecurrenceEditorModel) handleDown() tea.Cmd {
	switch m.focusSection {
	case 0: // interval - decrement
		if m.interval > 1 {
			m.interval--
			m.intervalEditing = true
		}
	case 2: // weekdays
		if m.weekdayCursor < 4 {
			// Move from top row to bottom row
			newCursor := min(m.weekdayCursor+4, 6)
			m.weekdayCursor = newCursor
		}
	}
	return nil
}

func (m *RecurrenceEditorModel) buildPattern() *models.RecurrencePattern {
	interval := max(m.interval,
		// Ensure minimum interval of 1
		1)
	pattern := &models.RecurrencePattern{
		Interval: interval,
		Unit:     m.units[m.unitCursor],
	}

	// Add weekdays if weekly and any selected
	if m.isWeeklyUnit() {
		var weekdays []int
		for i := range 7 {
			if m.weekdays[i] {
				weekdays = append(weekdays, i)
			}
		}
		if len(weekdays) > 0 {
			pattern.Weekdays = weekdays
		}
	}

	return pattern
}

func (m *RecurrenceEditorModel) View() string {
	var b strings.Builder

	// Interval row
	var intervalLabel string
	if m.focusSection == 0 {
		intervalLabel = FieldLabelFocusedStyle.Render("Interval: ")
	} else {
		intervalLabel = FieldLabelStyle.Render("Interval: ")
	}
	b.WriteString(intervalLabel)

	// Interval value
	intervalStr := fmt.Sprintf("%d", m.interval)
	if m.interval == 0 {
		intervalStr = " " // Show empty placeholder when cleared
	}
	if m.focusSection == 0 {
		b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", intervalStr)))
	} else {
		b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", intervalStr)))
	}
	b.WriteString("  ")

	// Unit selector
	var unitLabel string
	if m.focusSection == 1 {
		unitLabel = FieldLabelFocusedStyle.Render("Unit: ")
	} else {
		unitLabel = FieldLabelStyle.Render("Unit: ")
	}
	b.WriteString(unitLabel)
	b.WriteString(m.renderUnitSelector())
	b.WriteString("\n\n")

	// Weekdays section (only for weekly)
	if m.isWeeklyUnit() {
		var weekdaysLabel string
		if m.focusSection == 2 {
			weekdaysLabel = FieldLabelFocusedStyle.Render("Weekdays: ")
		} else {
			weekdaysLabel = FieldLabelStyle.Render("Weekdays: ")
		}
		b.WriteString(weekdaysLabel)
		b.WriteString("\n")
		b.WriteString(m.renderWeekdays())
		b.WriteString("\n")
	}

	// Separator
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("─────────────────────────────────────────"))
	b.WriteString("\n\n")

	// Preview
	pattern := m.buildPattern()
	b.WriteString(FieldLabelStyle.Render("Preview: "))
	if pattern != nil {
		b.WriteString(BoldStyle.Render(pattern.String()))
	}
	b.WriteString("\n")

	// Streak info (from existing pattern)
	if m.streak > 0 {
		b.WriteString(FieldLabelStyle.Render("Streak:  "))
		b.WriteString(MutedStyle.Render(fmt.Sprintf("%d", m.streak)))
		b.WriteString("\n")
	}

	// Next occurrences
	tempTask := &models.Task{
		DueDate:    m.dueDate,
		Recurrence: pattern,
	}
	upcoming := tempTask.NextNDueDates(3)
	if len(upcoming) > 0 {
		for i, d := range upcoming {
			label := fmt.Sprintf("Next %d:  ", i+1)
			b.WriteString(FieldLabelStyle.Render(label))
			if i == 0 {
				b.WriteString(d.Format("Mon Jan 2, 2006"))
			} else {
				b.WriteString(MutedStyle.Render(d.Format("Mon Jan 2, 2006")))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString(MutedStyle.Render("Set a due date to see next occurrences"))
		b.WriteString("\n")
	}

	left := HelpStyle.Render("Tab: nav · ←→: select · Space: toggle · Enter: confirm")

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Custom Recurrence", ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, "", m.width))

	return out.String()
}

// ShouldCaptureEsc returns true to let the editor handle esc internally.
func (m *RecurrenceEditorModel) ShouldCaptureEsc() bool {
	return true
}

func (m *RecurrenceEditorModel) renderUnitSelector() string {
	var b strings.Builder

	unitNames := []string{"day", "week", "month", "year"}

	for i, name := range unitNames {
		if i == m.unitCursor {
			if m.focusSection == 1 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", name)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf("[%s]", name)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", name)))
		}
	}

	return b.String()
}

func (m *RecurrenceEditorModel) renderWeekdays() string {
	var b strings.Builder

	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

	// First row: Sun, Mon, Tue, Wed (0-3)
	b.WriteString("  ")
	for i := range 4 {
		b.WriteString(m.renderWeekdayToggle(i, dayNames[i]))
		if i < 3 {
			b.WriteString("  ")
		}
	}
	b.WriteString("\n")

	// Second row: Thu, Fri, Sat (4-6)
	b.WriteString("  ")
	for i := 4; i < 7; i++ {
		b.WriteString(m.renderWeekdayToggle(i, dayNames[i]))
		if i < 6 {
			b.WriteString("  ")
		}
	}

	return b.String()
}

func (m *RecurrenceEditorModel) renderWeekdayToggle(dayNum int, name string) string {
	selected := m.weekdays[dayNum]
	focused := m.focusSection == 2 && m.weekdayCursor == dayNum

	icon := "○"
	if selected {
		icon = "●"
	}

	display := fmt.Sprintf("%s %s", icon, name)

	if focused {
		return SelectorOptionSelectedStyle.Render(fmt.Sprintf("[%s]", display))
	} else if selected {
		return BoldStyle.Render(display)
	}
	return SelectorOptionStyle.Render(display)
}

// ShowRecurrenceEditorMsg triggers showing the recurrence editor.
type ShowRecurrenceEditorMsg struct {
	CurrentPattern *models.RecurrencePattern
	DueDate        *time.Time
}

// RecurrenceEditorDoneMsg signals the recurrence editor has finished.
type RecurrenceEditorDoneMsg struct {
	Pattern  *models.RecurrencePattern
	Canceled bool
}

func (m *RecurrenceEditorModel) StatusBarLabel() string { return "Recurrence" }
