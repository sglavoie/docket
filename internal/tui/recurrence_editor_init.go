package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

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
		m.mode = current.Mode
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

// ShouldCaptureEsc returns true to let the editor handle esc internally.
func (m *RecurrenceEditorModel) ShouldCaptureEsc() bool { return true }

// HasTextInput returns false; the recurrence editor uses selector inputs, not text fields.
func (m *RecurrenceEditorModel) HasTextInput() bool { return false }

// IsFullHeight returns false; the recurrence editor is a panel overlay, not full-screen.
func (m *RecurrenceEditorModel) IsFullHeight() bool { return false }

func (m *RecurrenceEditorModel) StatusBarLabel() string { return "Recurrence" }

// HandledKeys returns all keys the recurrence editor handles for keybinding sync.
func (m *RecurrenceEditorModel) HandledKeys() []string {
	return []string{
		"tab", "shift+tab",
		"up", "down", "j", "k",
		"left", "right", "h", "l",
		"space",
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"backspace",
		"m",
		"enter",
		"esc",
	}
}
