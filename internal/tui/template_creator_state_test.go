package tui

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

func TestTaskTemplateCreatorStagesCanonicalStateInput(t *testing.T) {
	appModel, _ := newTestAppModelWithProject()
	creator := NewTaskTemplateCreatorModel(appModel.runtime)

	if got := creator.stateInput.Value(); got != models.StateInbox.String() {
		t.Fatalf("new template state input = %q, want %q", got, models.StateInbox)
	}
	var wantStates []string
	for _, state := range models.AllWorkflowStates() {
		wantStates = append(wantStates, state.String())
	}
	if !reflect.DeepEqual(creator.stateCache, wantStates) {
		t.Fatalf("state suggestions = %v, want %v", creator.stateCache, wantStates)
	}

	editor := NewTaskTemplateEditorModel(appModel.runtime, template.TaskTemplate{
		Name:  "Review template",
		Type:  models.TaskTypeTask,
		State: models.StateReview,
	})
	if got := editor.stateInput.Value(); got != models.StateReview.String() {
		t.Fatalf("editor state input = %q, want %q", got, models.StateReview)
	}
}

func TestTaskTemplateCreatorRejectsNoncanonicalState(t *testing.T) {
	appModel, _ := newTestAppModelWithProject()
	creator := NewTaskTemplateCreatorModel(appModel.runtime)
	creator.nameInput.SetValue("Invalid state template")
	creator.stateInput.SetValue("Backlog")

	cmd := creator.saveTemplate()
	if cmd == nil {
		t.Fatal("invalid state returned nil command")
	}
	msg := cmd()
	if _, ok := msg.(ErrorMsg); !ok {
		t.Fatalf("invalid state result = %T, want ErrorMsg", msg)
	}
}
