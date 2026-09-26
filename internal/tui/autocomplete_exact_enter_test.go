package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

// A token that already spells an offered value is complete, so Enter submits
// rather than spending a keypress accepting a suggestion that restates it.
func TestCaptureEnterSubmitsATokenThatSpellsAnOfferedValue(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}

	exact := draftedCaptureInput(t, "Read book %routed", home, projects)
	if !exact.AC.IsActive() {
		t.Fatal("the exact project token closed the suggestions; the test needs them open")
	}
	if !exact.AutocompleteLiteralComplete() {
		t.Fatal("an exact %routed token must let Enter submit")
	}

	partial := draftedCaptureInput(t, "Read book %rou", home, projects)
	if !partial.AC.IsActive() {
		t.Fatal("a partial project token must still offer suggestions")
	}
	if partial.AutocompleteLiteralComplete() {
		t.Fatal("a partial project token must leave Enter accepting the suggestion")
	}
}

func TestRowEditorEnterSavesATokenThatSpellsAnOfferedValue(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]

	exact := draftedRowEditor(t, "Edited task %routed", task, home, projects)
	if !exact.AC.IsActive() {
		t.Fatal("the exact project token closed the suggestions; the test needs them open")
	}
	transition, result, _ := exact.Handle(tea.KeyPressMsg{Code: tea.KeyEnter})
	if transition != grammar.QuickEditCommit || result == nil {
		t.Fatalf("Enter on an exact token = %v, want a commit", transition)
	}

	partial := draftedRowEditor(t, "Edited task %rou", task, home, projects)
	transition, _, _ = partial.Handle(tea.KeyPressMsg{Code: tea.KeyEnter})
	if transition != grammar.QuickEditContinue {
		t.Fatalf("Enter on a partial token = %v, want the suggestion accepted", transition)
	}
	if got := partial.Input.Value(); got != "Edited task %routed " {
		t.Fatalf("accepted draft = %q, want the completed project token", got)
	}
}
