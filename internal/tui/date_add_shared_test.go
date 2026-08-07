package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestDateAdderSubmitsTheParsedDateAndLeavesAddMode(t *testing.T) {
	adder := NewDateAdder()
	adder.EnterAddMode()
	adder.AddDateInput.SetValue("2027-03-15")

	var got *time.Time
	adder.handleAddDateInputKey(tea.KeyPressMsg{Code: tea.KeyEnter}, func(date time.Time) error {
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
	adder := NewDateAdder()
	adder.EnterAddMode()
	adder.AddDateInput.SetValue("nonsense")

	submitted := false
	adder.handleAddDateInputKey(tea.KeyPressMsg{Code: tea.KeyEnter}, func(time.Time) error {
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
	adder := NewDateAdder()
	adder.EnterAddMode()
	adder.AddDateError = "stale"

	adder.handleAddDateInputKey(tea.KeyPressMsg{Code: tea.KeyEscape}, func(time.Time) error {
		t.Fatal("escape submitted a date")
		return nil
	})
	if adder.AddDateMode || adder.AddDateError != "" {
		t.Errorf("escape left mode %v error %q, want a clean preset list", adder.AddDateMode, adder.AddDateError)
	}
}
