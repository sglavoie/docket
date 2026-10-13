package chrome

import (
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// DateAdder provides the inline typed-date mode for the value pickers whose
// providers report dateEntry. It is the date sibling of TagAdder: same three
// integration points, its own validator and vocabulary.
type DateAdder struct {
	AddDateMode  bool
	AddDateInput textinput.Model
	AddDateError string

	allowsTime bool
}

// NewDateAdder creates a DateAdder with a configured text input.
func NewDateAdder() DateAdder {
	input := textinput.New()
	input.CharLimit = 30
	input.Placeholder = "e.g., 2026-01-15, dec 15, +3d, fri"
	theme.ApplyThemedTextInput(&input)
	return DateAdder{AddDateInput: input}
}

// NewInstantDateAdder creates a DateAdder whose value may carry a time of day,
// so the hour form parses instead of being refused as a due-date vocabulary.
func NewInstantDateAdder() DateAdder {
	input := textinput.New()
	input.CharLimit = 30
	input.Placeholder = "e.g., +4h, tomorrow, 2026-01-15, fri"
	theme.ApplyThemedTextInput(&input)
	return DateAdder{AddDateInput: input, allowsTime: true}
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

// HandleAddDateInputKey owns the typed-date input lifecycle. Every key belongs
// to the input while the mode is active, and clearing stays on the preset list,
// so this mode only ever produces a date.
func (a *DateAdder) HandleAddDateInputKey(
	msg tea.KeyPressMsg,
	submit func(time.Time) error,
) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.leaveAddMode()
		return nil

	case "enter":
		date, err := a.parse(a.AddDateInput.Value())
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
		previous := a.AddDateInput.Value()
		var cmd tea.Cmd
		a.AddDateInput, cmd = a.AddDateInput.Update(msg)
		if a.AddDateInput.Value() != previous {
			a.AddDateError = ""
		}
		return cmd
	}
}

// DatePreview resolves through the submit parser, quietly omitting incomplete
// input. Deferrals carry their clock because they represent an instant.
func (a *DateAdder) DatePreview() string {
	date, err := a.parse(a.AddDateInput.Value())
	if err != nil {
		return ""
	}
	return FormatWeekdayDate(date, time.Now(), a.allowsTime)
}

// parse resolves the typed expression against the granularity this adder
// stores, so the hour form is accepted or refused by name from one fact.
func (a *DateAdder) parse(input string) (time.Time, error) {
	if a.allowsTime {
		return util.ParseDeferInstant(input)
	}
	return util.ParseCalendarDate(input)
}

// DateAdderFormats is the accepted-format reference shown beneath the input.
func (a *DateAdder) DateAdderFormats() []string {
	formats := []string{
		"2026-01-15    Exact date (YYYY-MM-DD)",
		"tomorrow      Tomorrow's date",
		"+3d, +1w      Relative offset",
		"mon, next fri Next occurrence of weekday",
		"dec 15        Month day (next year if past)",
	}
	if a.allowsTime {
		formats = append(formats, "+4h           Hours from now (defer only)")
	}
	return formats
}
