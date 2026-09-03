package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

func TestFormEditorGuidanceRequiresCommitAndCancelByState(t *testing.T) {
	form := newTestTaskForm()
	assertRequiredGuidance(t, "task form", form.guidanceSpec(), true, "Ctrl+S save", "Esc cancel")
	form.err = errors.New("title is required")
	assertRequiredGuidance(t, "invalid task form", form.guidanceSpec(), true, "Ctrl+S retry", "Esc cancel")

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

	composer := &TaskComposerModel{errors: []grammar.LineError{{Line: 1, Message: "invalid capture"}}}
	spec, ok := composer.ContextualGuidance()
	assertRequiredGuidance(t, "invalid capture", spec, ok, "Enter retry", "Esc cancel")
}

func TestFormEditorGuidanceFitsNarrowBudgets(t *testing.T) {
	form := newTestTaskForm()
	form.activeTab = formTabChildren
	form.err = errors.New("invalid task")

	specs := []chrome.GuidanceSpec{
		form.guidanceSpec(),
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
	form := newTestTaskForm()
	form.err = errors.New("title is required")
	form.SetSize(formMinWidth, formMinHeight)
	assertFitsWithin(t, form.View().Content, formMinWidth, formMinHeight)

	textEditor := chrome.NewTextEditorModel("description", "draft")
	textEditor.SetSize(minTerminalWidth, formMinHeight)
	textView := textEditor.View().Content
	assertFitsWithin(t, textView, minTerminalWidth, formMinHeight)
	if !strings.Contains(ansi.Strip(textView), "Ctrl+S save") || !strings.Contains(ansi.Strip(textView), "Esc cancel") {
		t.Fatalf("text editor minimum layout lost required guidance:\n%s", ansi.Strip(textView))
	}
}

func TestTaskFormInlinePickersRenderStructuredRequiredGuidance(t *testing.T) {
	form := newTestTaskForm()
	form.SetSize(formMinWidth, formMinHeight)

	renders := map[string]string{
		"priority":  newFormPriorityPicker(form.draft.priority).render(formMinWidth, form.draft.priority),
		"state":     newFormStatePicker(form.draft.state).render(formMinWidth, form.draft.state),
		"flags":     newFormFlagsPicker(form.draft.pinned, form.draft.review).render(formMinWidth),
		"due date":  newFormDueDatePicker(form.draft.dueDate).render(formMinWidth, form.draft.dueDate),
		"checklist": form.renderChecklistEditOverlay(),
		"subtask":   form.renderSubtaskEditOverlay(),
	}
	for name, rendered := range renders {
		plain := ansi.Strip(rendered)
		if !strings.Contains(plain, "Esc cancel") {
			t.Errorf("%s picker lost required cancellation guidance:\n%s", name, plain)
		}
		if !strings.Contains(plain, "Enter") {
			t.Errorf("%s picker lost required commit guidance:\n%s", name, plain)
		}
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
