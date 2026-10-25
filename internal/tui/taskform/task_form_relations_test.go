package taskform

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/status"
)

func TestExpandedCapturePreservesRelationsInOneUndoEntry(t *testing.T) {
	env := newScenarioEnv(t, "expansion-relations")
	child, err := env.svc.Capture(app.CaptureRequest{ProjectID: env.project.ID, Title: "Adopt me"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := env.svc.Capture(app.CaptureRequest{ProjectID: env.project.ID, Title: "Reference me"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := newTestConfig()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: env.project}, testRuntimeAt(cfg.DataDir(), cfg))
	composer.SetTaskFormCoordinator(env.features.TaskForm)
	composer.Input.SetValue("Umbrella @@+4h <1 ~2 @tomorrow !1 *pin =review #release ##office &https://example.com/spec ;Discuss this;")
	requests, ok := composer.ParseCompactLines()
	if !ok {
		t.Fatal(composer.Errors)
	}
	// Expand the already-resolved draft so a relative deferral is not re-timed.
	composer.ExpandRequest(requests[0])
	form := composer.Expanded
	if form == nil {
		t.Fatal(composer.Errors)
	}
	form.draft.checklist = []models.ChecklistItem{{Text: "Check"}}
	form.draft.subtasks = []formSubtaskDraft{{title: "New child"}}
	result := runCreate(t, form)
	fresh := env.reload(t)
	task := fresh.GetTaskByID(result.Result.TaskID)
	if fresh.Tasks[0].ID != task.ID {
		t.Fatal("expansion lost capture insertion order")
	}
	if task.DeferredUntil == nil || !task.DeferredUntil.Equal(*requests[0].Defer) || task.State() != models.StateBacklog {
		t.Fatalf("deferral changed: %+v", task)
	}
	if len(task.Dependencies) != 1 || task.Dependencies[0].TaskID != other.TaskID {
		t.Fatalf("dependencies changed: %+v", task.Dependencies)
	}
	adopted := fresh.GetTaskByID(child.TaskID)
	if adopted.ParentID == nil || *adopted.ParentID != task.ID {
		t.Fatal("adoption missing")
	}
	if len(task.Attachments) != 1 || len(task.Comments) != 1 || len(task.Checklist) != 1 || !task.Pinned || !task.Review || task.Context != "office" || task.Priority != 1 {
		t.Fatalf("metadata lost: %+v", task)
	}
	env.undo(t)
	fresh = env.reload(t)
	if len(fresh.Tasks) != 2 || fresh.GetTaskByID(child.TaskID).ParentID != nil {
		t.Fatal("undo did not restore whole creation and adoption")
	}
	env.redo(t)
	if env.reload(t).GetTaskByID(task.ID).DeferredUntil == nil {
		t.Fatal("redo lost deferral")
	}
}

func TestExpandedRelationsEditorKeepsInvalidDraftAndSupportsClear(t *testing.T) {
	env := newScenarioEnv(t, "relation-editor")
	_, err := env.svc.Capture(app.CaptureRequest{ProjectID: env.project.ID, Title: "Existing"})
	if err != nil {
		t.Fatal(err)
	}
	form := scenarioCreateForm(t, env)
	for _, kind := range []formRelationKind{formRelationChildren, formRelationDependencies, formRelationDefer} {
		form.openRelationEditor(kind)
		form.relationInput.SetValue("missing")
		form.handleRelationEditor(syntheticSpecialKey(tea.KeyEnter))
		if form.activeOverlay != formOverlayRelation || form.relationError == nil || form.relationInput.Value() != "missing" {
			t.Fatal("invalid edit was lost")
		}
		value := "1"
		if kind == formRelationDefer {
			value = "+4h"
		}
		form.relationInput.SetValue(value)
		form.handleRelationEditor(syntheticSpecialKey(tea.KeyEnter))
		if form.activeOverlay != formOverlayNone || form.relationValue(kind, false) == "" {
			t.Fatalf("valid edit refused: %v", form.relationError)
		}
		form.openRelationEditor(kind)
		form.relationInput.SetValue("")
		form.handleRelationEditor(syntheticSpecialKey(tea.KeyEscape))
		if form.relationValue(kind, false) == "" {
			t.Fatal("cancel cleared stored draft")
		}
		form.openRelationEditor(kind)
		form.relationInput.SetValue("")
		form.handleRelationEditor(syntheticSpecialKey(tea.KeyEnter))
		if form.relationValue(kind, false) != "" {
			t.Fatal("empty apply failed to clear")
		}
	}
}

func TestExpandedCaptureDoesNotInferTodayAgain(t *testing.T) {
	env := newScenarioEnv(t, "explicit-status")
	cfg := newTestConfig()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: env.project}, testRuntimeAt(cfg.DataDir(), cfg))
	composer.SetTaskFormCoordinator(env.features.TaskForm)
	today := time.Now()
	composer.ExpandRequest(app.CaptureRequest{ProjectID: env.project.ID, Title: "Wait", Due: &today, Destination: app.CaptureDestination(models.StateWaiting)})
	result := runCreate(t, composer.Expanded)
	if task := env.reload(t).GetTaskByID(result.Result.TaskID); task.State() != models.StateWaiting {
		t.Fatalf("expansion changed destination: %s", task.State())
	}
}

func TestExpandedCaptureRejectsTerminalAdoptionWithoutPartialWrite(t *testing.T) {
	env := newScenarioEnv(t, "invalid-adoption")
	child, err := env.svc.Capture(app.CaptureRequest{ProjectID: env.project.ID, Title: "Open child"})
	if err != nil {
		t.Fatal(err)
	}
	form := scenarioCreateForm(t, env)
	form.titleInput.SetValue("Done parent")
	form.draft.state = models.StateDone
	form.draft.childIDs = []string{child.TaskID}
	msg := form.handleSave()()
	if _, ok := msg.(status.ErrorMsg); !ok {
		t.Fatalf("expected refusal, got %T", msg)
	}
	form.Update(msg)
	if !strings.Contains(form.titleInput.Value(), "Done parent") || len(form.draft.childIDs) != 1 {
		t.Fatal("refusal lost draft")
	}
	fresh := env.reload(t)
	if len(fresh.Tasks) != 1 || fresh.GetTaskByID(child.TaskID).ParentID != nil {
		t.Fatal("failed save partially committed")
	}
}

func TestPortableCaptureNewlineDoesNotSubmit(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, runtime)
	composer.Input.SetValue("First")
	composer.Input.CursorEnd()
	composer.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	if composer.Input.Value() != "First\n" || composer.SubmissionInFlight {
		t.Fatalf("newline submitted: %q", composer.Input.Value())
	}
}

func TestExpandedCaptureProtectsDraftOnEveryTab(t *testing.T) {
	for _, tab := range []int{formTabBasics, formTabSchedule, formTabChildren, formTabExtras} {
		runtime, project := newTestRuntimeWithProject()
		composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, runtime)
		composer.Input.SetValue("Keep me @@tomorrow")
		composer.ExpandSingle()
		form := composer.Expanded
		form.activeTab = tab
		form.focusIndex = 0
		form.syncInputFocus()
		form.Update(syntheticSpecialKey(tea.KeyEsc))
		if !form.ConfirmDiscard {
			t.Fatalf("tab %d abandoned expanded capture", tab)
		}
	}
}
