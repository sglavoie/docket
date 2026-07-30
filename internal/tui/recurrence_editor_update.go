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

		case "c":
			return m, func() tea.Msg {
				return RecurrenceEditorDoneMsg{Pattern: nil, Canceled: false}
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

func (m *RecurrenceEditorModel) nextFocusSection() int {
	next := m.focusSection + 1
	if next > 1 {
		next = 0
	}
	return next
}

func (m *RecurrenceEditorModel) prevFocusSection() int {
	prev := m.focusSection - 1
	if prev < 0 {
		prev = 1
	}
	return prev
}

func (m *RecurrenceEditorModel) handleLeft() tea.Cmd {
	switch m.focusSection {
	case 0: // interval - no left/right navigation
	case 1: // unit selector
		if m.unitCursor > 0 {
			m.unitCursor--
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

	return pattern
}
