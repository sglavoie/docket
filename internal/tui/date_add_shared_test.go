package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestDateAdderSubmitsTheParsedDateAndLeavesAddMode(t *testing.T) {
	adder := chrome.NewDateAdder()
	adder.EnterAddMode()
	adder.AddDateInput.SetValue("2027-03-15")

	var got *time.Time
	adder.HandleAddDateInputKey(tea.KeyPressMsg{Code: tea.KeyEnter}, func(date time.Time) error {
		got = &date
		return nil
	})
	if got == nil {
		t.Fatal("a parseable date produced no submission")
	}
	if want := "2027-03-15"; got.Format("2006-01-02") != want {
		t.Errorf("submitted date = %s, want %s", got.Format("2006-01-02"), want)
	}
	if adder.AddDateMode {
		t.Error("a committed date left typed-date mode active")
	}
	if adder.AddDateError != "" {
		t.Errorf("committed date reported error %q", adder.AddDateError)
	}
}

func TestDateAdderKeepsTheInputOpenOnAParseFailure(t *testing.T) {
	adder := chrome.NewDateAdder()
	adder.EnterAddMode()
	adder.AddDateInput.SetValue("nonsense")

	submitted := false
	adder.HandleAddDateInputKey(tea.KeyPressMsg{Code: tea.KeyEnter}, func(time.Time) error {
		submitted = true
		return nil
	})
	if submitted {
		t.Fatal("an unparseable expression reached the submit callback")
	}
	if !adder.AddDateMode {
		t.Error("a parse failure closed typed-date mode instead of letting the user retry")
	}
	if adder.AddDateError == "" {
		t.Error("a parse failure produced no message")
	}
}

func TestDateAdderEscapeReturnsToThePresetList(t *testing.T) {
	adder := chrome.NewDateAdder()
	adder.EnterAddMode()
	adder.AddDateError = "stale"

	adder.HandleAddDateInputKey(tea.KeyPressMsg{Code: tea.KeyEscape}, func(time.Time) error {
		t.Fatal("escape submitted a date")
		return nil
	})
	if adder.AddDateMode || adder.AddDateError != "" {
		t.Errorf("escape left mode %v error %q, want a clean preset list", adder.AddDateMode, adder.AddDateError)
	}
}

func TestInstantDateAdderAcceptsTheHourForm(t *testing.T) {
	adder := chrome.NewInstantDateAdder()
	adder.EnterAddMode()
	adder.AddDateInput.SetValue("+4h")

	before := time.Now()
	var got *time.Time
	adder.HandleAddDateInputKey(tea.KeyPressMsg{Code: tea.KeyEnter}, func(date time.Time) error {
		got = &date
		return nil
	})
	if got == nil {
		t.Fatalf("the instant adder refused +4h with %q", adder.AddDateError)
	}
	offset := got.Sub(before)
	if offset < 4*time.Hour-time.Minute || offset > 4*time.Hour+time.Minute {
		t.Errorf("+4h resolved %s from now, want about 4h", offset)
	}
	if adder.AddDateMode {
		t.Error("a committed instant left typed-date mode active")
	}
}

func TestDayDateAdderRefusesTheHourFormByName(t *testing.T) {
	adder := chrome.NewDateAdder()
	adder.EnterAddMode()
	adder.AddDateInput.SetValue("+4h")

	adder.HandleAddDateInputKey(tea.KeyPressMsg{Code: tea.KeyEnter}, func(time.Time) error {
		t.Fatal("the day adder submitted an hour offset")
		return nil
	})
	if !strings.Contains(adder.AddDateError, "hours are defer-only") {
		t.Errorf("day adder refused +4h with %q, want the defer-only explanation", adder.AddDateError)
	}
}

func TestInstantDateAdderFormatsAddExactlyTheHoursLine(t *testing.T) {
	day := chrome.NewDateAdder()
	instant := chrome.NewInstantDateAdder()

	dayFormats := day.DateAdderFormats()
	instantFormats := instant.DateAdderFormats()
	if len(instantFormats) != len(dayFormats)+1 {
		t.Fatalf("instant formats = %d lines, want one more than the day adder's %d", len(instantFormats), len(dayFormats))
	}
	for i, line := range dayFormats {
		if instantFormats[i] != line {
			t.Errorf("format line %d = %q, want the shared %q", i, instantFormats[i], line)
		}
	}
	if want := "+4h           Hours from now (defer only)"; instantFormats[len(dayFormats)] != want {
		t.Errorf("extra format line = %q, want %q", instantFormats[len(dayFormats)], want)
	}
}
