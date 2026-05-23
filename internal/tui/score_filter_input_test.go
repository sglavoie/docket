package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestScoreFilterInputHandlesLeadingSpace(t *testing.T) {
	var s ScoreFilterInput
	s.Enter(nil)
	s.Input.SetValue(" 5")

	val, done, _ := s.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !done {
		t.Fatal("expected input to be done after enter")
	}
	if val == nil {
		t.Error("expected val=5.0 for input ' 5', got nil (silent discard)")
	} else if *val != 5.0 {
		t.Errorf("expected val=5.0, got %v", *val)
	}
}
