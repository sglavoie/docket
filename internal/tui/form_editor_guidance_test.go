package tui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestFormEditorGuidanceRequiresCommitAndCancelByState(t *testing.T) {
	creator := &ProjectCreatorModel{}
	assertRequiredGuidance(t, "project creator", creator.guidanceSpec(), true, "Enter create", "Esc cancel")
	creator.ConfirmDiscard = true
	assertRequiredGuidance(t, "project creator discard", creator.guidanceSpec(), true, "y confirm", "n/Esc cancel")

	editor := &ProjectEditorModel{hasChanges: true}
	assertRequiredGuidance(t, "dirty project editor", editor.guidanceSpec(), true, "Ctrl+S save", "Esc cancel")
	editor.err = errors.New("invalid project")
	assertRequiredGuidance(t, "invalid project editor", editor.guidanceSpec(), true, "Ctrl+S retry", "Esc cancel")

	textEditor := chrome.NewTextEditorModel("description", "")
	assertRequiredGuidance(t, "text editor", textEditor.GuidanceSpec(), true, "Ctrl+S save", "Esc cancel")
	textEditor.ExternalEdit = true
	if got := textEditor.GuidanceSpec().Supplemental; got != "External editor open" {
		t.Fatalf("external-editor supplemental = %q", got)
	}
}

func TestFormEditorGuidanceFitsNarrowBudgets(t *testing.T) {
	specs := []chrome.GuidanceSpec{
		(&ProjectCreatorModel{}).guidanceSpec(),
		(&ProjectEditorModel{hasChanges: true}).guidanceSpec(),
		chrome.NewTextEditorModel("description", "").GuidanceSpec(),
	}
	for index, spec := range specs {
		if spec.Rows != 2 {
			t.Errorf("spec %d row budget = %d, want 2", index, spec.Rows)
		}
		assertGuidanceFits(t, spec, 24)
	}
}

func TestFormEditorMinimumLayoutsBudgetRenderedGuidance(t *testing.T) {
	// The text editor derives its floor the same way the task form does, two
	// rows below the terminal minimum, so the two budgets stay one number even
	// though the form's own coverage now lives beside it in internal/tui/taskform.
	const minHeight = chrome.MinTerminalHeight - 2

	textEditor := chrome.NewTextEditorModel("description", "draft")
	textEditor.SetSize(chrome.MinTerminalWidth, minHeight)
	textView := textEditor.View().Content
	assertFitsWithin(t, textView, chrome.MinTerminalWidth, minHeight)
	if !strings.Contains(ansi.Strip(textView), "Ctrl+S save") || !strings.Contains(ansi.Strip(textView), "Esc cancel") {
		t.Fatalf("text editor minimum layout lost required guidance:\n%s", ansi.Strip(textView))
	}
}

func TestQuickEditGuidanceOwnsGrammarAndLocalBarDoesNotDuplicateIt(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Task", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	out := NewOutlinerModel(project, nil, nil)
	out.quickEdit.Enter(task, false)

	spec, ok := out.ContextualGuidance()
	assertRequiredGuidance(t, "quick edit", spec, ok, "Enter save", "Esc cancel")
	got := guidanceEntryText(spec)
	for _, want := range []string{"!1-3/!0 priority", "#tag add tag", "@date/@0 due", "$destination destination", "*pin/*unpin pin", "%project move", ">parent/>0 parent", "<child adopt"} {
		if !strings.Contains(got, want) {
			t.Errorf("quick-edit guidance = %q, want %q", got, want)
		}
	}
}

// assertFitsWithin fails if any rendered line exceeds width or the line count
// exceeds height.
func assertFitsWithin(t *testing.T, rendered string, width, height int) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(lines) > height {
		t.Fatalf("render has %d lines, exceeds height %d:\n\n%s",
			len(lines), height, ansi.Strip(rendered))
	}
	for _, line := range lines {
		if w := lipgloss.Width(ansi.Strip(line)); w > width {
			t.Fatalf("line exceeds width %d: %q (width=%d)\n\n%s",
				width, ansi.Strip(line), w, ansi.Strip(rendered))
		}
	}
}
