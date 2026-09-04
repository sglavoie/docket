package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// The project creator and editor moved to internal/tui/projecthub with the
// forms themselves, so their guidance is asserted there; the text editor is
// still the shell's, and its budget stays here.
func TestTextEditorGuidanceRequiresCommitAndCancelByState(t *testing.T) {
	textEditor := chrome.NewTextEditorModel("description", "")
	assertRequiredGuidance(t, "text editor", textEditor.GuidanceSpec(), true, "Ctrl+S save", "Esc cancel")
	textEditor.ExternalEdit = true
	if got := textEditor.GuidanceSpec().Supplemental; got != "External editor open" {
		t.Fatalf("external-editor supplemental = %q", got)
	}
}

func TestTextEditorGuidanceFitsNarrowBudgets(t *testing.T) {
	spec := chrome.NewTextEditorModel("description", "").GuidanceSpec()
	if spec.Rows != 2 {
		t.Errorf("text editor row budget = %d, want 2", spec.Rows)
	}
	assertGuidanceFits(t, spec, 24)
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
	out := outliner.NewOutlinerModel(project, nil, nil)
	out.QuickEdit.Enter(task, false)

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
