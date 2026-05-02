package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *RecurrenceEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
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

		case "space":
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

		case "m":
			// Toggle recurrence mode: from-due ↔ from-completion
			if m.mode == models.RecurModeFromCompletion {
				m.mode = models.RecurModeFromDue
			} else {
				m.mode = models.RecurModeFromCompletion
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
		Mode:     m.mode,
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
