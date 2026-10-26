package projecthub

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/status"
)

// TestProjectCreatorIgnoresEnterWhileCreating pins the in-flight guard: a
// second Enter before the first creation returns dispatches nothing, and the
// guard lifts once that creation has answered, refusal included.
func TestProjectCreatorIgnoresEnterWhileCreating(t *testing.T) {
	creator := NewProjectCreatorModel(nil, nil, nil)
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}

	_, first := creator.Update(enter)
	if first == nil {
		t.Fatal("expected the first Enter to dispatch a creation")
	}
	if _, second := creator.Update(enter); second != nil {
		t.Fatal("a second Enter while the creation runs must dispatch nothing")
	}

	// The empty name is refused; the refusal ends the in-flight creation.
	if _, ok := first().(status.ErrorMsg); !ok {
		t.Fatal("expected the empty name to be refused")
	}
	if _, retry := creator.Update(enter); retry == nil {
		t.Fatal("expected Enter to dispatch again once the creation answered")
	}
}
