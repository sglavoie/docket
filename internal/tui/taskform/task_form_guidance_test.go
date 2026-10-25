package taskform

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/operation"
)

func TestTaskFormGuidanceRequiresCommitAndCancelByState(t *testing.T) {
	form := newTestTaskForm()
	assertRequiredGuidance(t, "task form", form.guidanceSpec(), true, "Ctrl+S save", "Esc cancel")
	form.err = errors.New("title is required")
	assertRequiredGuidance(t, "invalid task form", form.guidanceSpec(), true, "Ctrl+S retry", "Esc cancel")

	composer := &TaskComposerModel{Errors: []grammar.LineError{{Line: 1, Message: "invalid capture"}}}
	spec, ok := composer.ContextualGuidance()
	assertRequiredGuidance(t, "invalid capture", spec, ok, "Enter create", "Esc cancel")
}

func TestTaskFormGuidanceFitsNarrowBudgets(t *testing.T) {
	form := newTestTaskForm()
	form.activeTab = formTabChildren
	form.err = errors.New("invalid task")

	spec := form.guidanceSpec()
	if spec.Rows != 2 {
		t.Errorf("task form row budget = %d, want 2", spec.Rows)
	}
	assertGuidanceFits(t, spec, 24)
}

func TestTaskFormMinimumLayoutBudgetsRenderedGuidance(t *testing.T) {
	form := newTestTaskForm()
	form.err = errors.New("title is required")
	form.SetSize(formMinWidth, formMinHeight)
	assertFitsWithin(t, form.View().Content, formMinWidth, formMinHeight)
}

func TestTaskFormInlinePickersRenderStructuredRequiredGuidance(t *testing.T) {
	form := newTestTaskForm()
	form.SetSize(formMinWidth, formMinHeight)

	renders := map[string]string{
		"priority":  newFormPriorityPicker(form.draft.priority).render(formMinWidth, form.draft.priority),
		"state":     newFormStatePicker(form.draft.state).render(formMinWidth, form.draft.state),
		"flags":     newFormFlagsPicker(form.draft.pinned, form.draft.review).render(formMinWidth),
		"due date":  newFormDueDatePicker(form.draft.dueDate).render(formMinWidth, form.draft.dueDate),
		"check-in":  NewCheckInEditorModel(operation.OperationExecutionRequest{}, "New task schedule", form.draft.checkIn, false).Render(formMinWidth),
		"checklist": form.renderChecklistEditOverlay(),
		"subtask":   form.renderSubtaskEditOverlay(),
	}
	for name, rendered := range renders {
		plain := ansi.Strip(rendered)
		if !strings.Contains(plain, "Esc cancel") {
			t.Errorf("%s picker lost required cancellation guidance:\n%s", name, plain)
		}
		commitKey := "Enter"
		if name == "check-in" {
			commitKey = "Ctrl+S save"
		}
		if !strings.Contains(plain, commitKey) {
			t.Errorf("%s picker lost required commit guidance:\n%s", name, plain)
		}
	}
}
