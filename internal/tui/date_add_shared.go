package tui

import (
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// DateAdder provides the inline typed-date mode for the value pickers whose
// providers report dateEntry. It is the date sibling of TagAdder: same three
// integration points, its own validator and vocabulary.
type DateAdder struct {
	AddDateMode  bool
	AddDateInput textinput.Model
	AddDateError string
}

// NewDateAdder creates a DateAdder with a configured text input.
func NewDateAdder() DateAdder {
	input := textinput.New()
	input.CharLimit = 30
	input.Placeholder = "e.g., 2026-01-15, dec 15, +3d, fri"
	applyThemedTextInput(&input)
	return DateAdder{AddDateInput: input}
}

// HasTextInput returns true when the typed-date input is active.
func (a *DateAdder) HasTextInput() bool { return a.AddDateMode }

// ShouldCaptureEsc returns true when the typed-date input is active.
func (a *DateAdder) ShouldCaptureEsc() bool { return a.AddDateMode }

// EnterAddMode activates typed-date mode and returns the blink command.
func (a *DateAdder) EnterAddMode() tea.Cmd {
	a.AddDateMode = true
	a.AddDateError = ""
	a.AddDateInput.Reset()
	a.AddDateInput.SetValue("")
	a.AddDateInput.Focus()
	return textinput.Blink
}

func (a *DateAdder) leaveAddMode() {
	a.AddDateMode = false
	a.AddDateError = ""
	a.AddDateInput.Blur()
}

// handleAddDateInputKey owns the typed-date input lifecycle. Every key belongs
// to the input while the mode is active, and clearing stays on the preset list,
// so this mode only ever produces a date.
func (a *DateAdder) handleAddDateInputKey(
	msg tea.KeyPressMsg,
	submit func(time.Time) error,
) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.leaveAddMode()
		return nil

	case "enter":
		date, err := ParseDate(a.AddDateInput.Value())
		if err != nil {
			a.AddDateError = err.Error()
			return nil
		}
		if err := submit(date); err != nil {
			a.AddDateError = err.Error()
			return nil
		}
		a.leaveAddMode()
		return nil

	default:
		var cmd tea.Cmd
		a.AddDateInput, cmd = a.AddDateInput.Update(msg)
		return cmd
	}
}

// dateAdderFormats is the accepted-format reference shown beneath the input.
func dateAdderFormats() []string {
	return []string{
		"2026-01-15    Exact date (YYYY-MM-DD)",
		"tomorrow      Tomorrow's date",
		"+3d, +1w      Relative offset",
		"mon, next fri Next occurrence of weekday",
		"dec 15        Month day (next year if past)",
	}
}
