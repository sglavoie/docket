package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func commitOfferState(t *testing.T, model *AppModel, context operation.CommandContext, state models.WorkflowState) tea.Msg {
	t.Helper()
	result := model.operations.Executor.Execute(operation.OperationExecutionRequest{ID: operation.OperationTaskSetState, Context: context, Value: state, HasValue: true})
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	cmd := result.Message.(operation.OperationCommandResultMsg).Cmd
	return runCheckInOfferCommand(checkInOfferCommandMsg{cmd: cmd, epoch: model.checkInOffers.epoch})()
}

func TestCheckInOfferCommittedSubsetAndSeparateUndo(t *testing.T) {
	first, same, scheduled := newBulkPickerTestTask(t, 1, "first"), newBulkPickerTestTask(t, 2, "same"), newBulkPickerTestTask(t, 3, "scheduled")
	same.ApplyWorkflowState(models.StateDoing)
	next := time.Now().AddDate(0, 0, 8)
	scheduled.NextCheckIn = func() *time.Time {
		d := time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, time.Local)
		return &d
	}()
	env := newAppBulkPickerEnv(t, []*models.Task{first, same, scheduled})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	context := bulkTaskActionTestContext(env.projects[0], first, scheduled)
	msg := commitOfferState(t, model, context, models.StateDoing)
	model.Update(msg)
	editor := model.operations.CheckInEditor
	if editor == nil {
		t.Fatal("committed active entry did not offer setup")
	}
	editor.HandlePaste(tea.PasteMsg{Content: "2027-01-01"})
	_, dismissed, selected := editor.HandleKey(syntheticCtrlKey('s'))
	if !dismissed {
		t.Fatal("valid save did not submit")
	}
	value := selected.(operation.OperationValueSelectedMsg)
	targets := value.Request.Context.Selection.Tasks
	if len(targets) != 1 || targets[0] != taskref.For(first, env.projects[0]) || !value.Request.Context.CheckInOffer {
		t.Fatalf("offer targets widened: %+v", value.Request.Context)
	}
	executeCheckInValue(t, model, value.Request.Context, value.Value.(operation.OperationCheckInValue))
	if stored := env.reload(t, 0).GetTaskByID(first.ID); stored.NextCheckIn == nil || stored.State() != models.StateDoing {
		t.Fatalf("save=%+v", stored)
	}
	env.undo(t)
	if stored := env.reload(t, 0).GetTaskByID(first.ID); stored.NextCheckIn != nil || stored.State() != models.StateDoing {
		t.Fatal("schedule undo changed the state commit")
	}
	env.undo(t)
	if stored := env.reload(t, 0).GetTaskByID(first.ID); stored.State() != models.StateBacklog {
		t.Fatal("state undo did not restore backlog")
	}
}

func TestCheckInOfferCancellationStaleDeliveryAndNoop(t *testing.T) {
	for _, mode := range []string{"escape", "navigation", "noninteractive", "duplicate", "noop", "failure"} {
		t.Run(mode, func(t *testing.T) {
			task := newBulkPickerTestTask(t, 1, "task")
			if mode == "noop" {
				task.ApplyWorkflowState(models.StateWaiting)
			}
			env := newAppBulkPickerEnv(t, []*models.Task{task})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			msg := commitOfferState(t, model, bulkTaskActionTestContext(env.projects[0], task), models.StateWaiting)
			switch mode {
			case "navigation":
				model.beginCheckInOfferUpdate(syntheticSpecialKey(tea.KeyDown))
			case "noninteractive":
				msg = msg.(checkInOfferOriginMsg).message
			case "failure":
				msg = coordinator.TransactionErrorMsg{Err: errors.New("refused")}
			}
			model.Update(msg)
			editor := model.operations.CheckInEditor
			if mode == "navigation" || mode == "noninteractive" || mode == "noop" || mode == "failure" {
				if editor != nil {
					t.Fatal("phantom offer")
				}
				return
			}
			if editor == nil {
				t.Fatal("missing offer")
			}
			model.Update(syntheticSpecialKey(tea.KeyEsc))
			if model.operations.CheckInEditor != nil {
				t.Fatal("Escape did not dismiss")
			}
			if mode == "duplicate" {
				model.Update(msg)
				if model.operations.CheckInEditor != nil {
					t.Fatal("duplicate result offered twice")
				}
			}
			stored := env.reload(t, 0).GetTaskByID(task.ID)
			if stored.State() != models.StateWaiting || stored.NextCheckIn != nil || stored.DueDate != nil {
				t.Fatal("dismissing setup mutated task")
			}
			env.undo(t)
			if env.reload(t, 0).GetTaskByID(task.ID).State() != models.StateBacklog {
				t.Fatal("dismissal created an undo entry")
			}
		})
	}
}

func TestCheckInOfferDoesNotOverwriteConcurrentSchedule(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "task")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	model.Update(commitOfferState(t, model, bulkTaskActionTestContext(env.projects[0], task), models.StateDoing))
	editor := model.operations.CheckInEditor
	if editor == nil {
		t.Fatal("missing offer")
	}
	editor.HandlePaste(tea.PasteMsg{Content: "2027-01-01"})
	_, _, selected := editor.HandleKey(syntheticCtrlKey('s'))
	value := selected.(operation.OperationValueSelectedMsg)
	concurrent := time.Date(2027, 2, 1, 0, 0, 0, 0, time.Local)
	if _, err := model.domain.service.BulkSetCheckIn([]app.TaskLocator{taskref.For(task, env.projects[0]).Locator()}, models.CheckInSchedule{Next: &concurrent, RepeatDays: 3}); err != nil {
		t.Fatal(err)
	}
	result := model.operations.Executor.Execute(operation.OperationExecutionRequest{ID: operation.OperationTaskSetCheckIn, Context: value.Request.Context, Value: value.Value, HasValue: true})
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	rejection := result.Message.(operation.OperationCommandResultMsg).Cmd()
	if _, ok := rejection.(CheckInRejectedMsg); !ok {
		t.Fatalf("concurrent schedule was not refused: %T", rejection)
	}
	stored := env.reload(t, 0).GetTaskByID(task.ID)
	if stored.NextCheckIn == nil || !stored.NextCheckIn.Equal(concurrent) || stored.CheckInRepeatDays != 3 {
		t.Fatal("offer overwrote concurrent schedule")
	}
}

func TestCheckInOfferCreationEffectsAndOpenAfter(t *testing.T) {
	for _, path := range []string{"capture", "bulk", "form", "open-after"} {
		t.Run(path, func(t *testing.T) {
			env := newAppBulkPickerEnv(t, []*models.Task{})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			features := coordinator.NewFeatureCoordinators(model.domain.service)
			req := app.CaptureRequest{ProjectID: env.projects[0].ID, Title: "new", Destination: app.CaptureDestination(models.StateDoing)}
			var cmd tea.Cmd
			switch path {
			case "capture", "open-after":
				cmd = features.Capture.CaptureCreate(req, path == "open-after")
			case "bulk":
				second := req
				second.Title = "second"
				cmd = features.Capture.CaptureBulk([]app.CaptureRequest{req, second}, true)
			case "form":
				cmd = features.TaskForm.CreateForm(app.AddTaskRequest{ProjectID: req.ProjectID, Title: req.Title, Destination: req.Destination}, -1, false)
			}
			msg := runCheckInOfferCommand(checkInOfferCommandMsg{cmd: cmd, epoch: model.checkInOffers.epoch})()
			model.Update(msg)
			if path == "open-after" {
				if model.operations.CheckInEditor != nil || len(model.checkInOffers.pending) != 1 {
					t.Fatal("offer interrupted open-after")
				}
				// An unchanged external buffer returns before the optional editor opens.
				stored := env.reload(t, 0).Tasks[0]
				model.Update(TaskContentEditedMsg{Task: stored, Project: env.projects[0], NewTitle: stored.Title})
			}
			if model.operations.CheckInEditor == nil {
				t.Fatal("creation did not offer")
			}
			want := 1
			if path == "bulk" {
				want = 2
			}
			model.operations.CheckInEditor.HandlePaste(tea.PasteMsg{Content: "2027-01-01"})
			_, _, value := model.operations.CheckInEditor.HandleKey(syntheticCtrlKey('s'))
			if got := len(value.(operation.OperationValueSelectedMsg).Request.Context.Selection.Tasks); got != want {
				t.Fatalf("targets=%d want %d", got, want)
			}
		})
	}
}

func TestCheckInOfferLineAndOutlineEdits(t *testing.T) {
	for _, path := range []string{"line", "outline"} {
		t.Run(path, func(t *testing.T) {
			task := newBulkPickerTestTask(t, 1, "before")
			env := newAppBulkPickerEnv(t, []*models.Task{task})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			if path == "line" {
				state := app.CaptureDestination(models.StateDoing)
				cmd := model.secondaryFeatures.TaskItem.EditTaskLine(app.TaskLineEditRequest{Source: taskref.For(task, env.projects[0]).Locator(), Title: task.Title, Destination: &state}, nil)
				model.Update(runCheckInOfferCommand(checkInOfferCommandMsg{cmd: cmd, epoch: model.checkInOffers.epoch})())
			} else {
				model.features = coordinator.NewFeatureCoordinators(model.domain.service)
				project, revision, err := model.features.Outline.Project(env.projects[0].ID)
				if err != nil {
					t.Fatal(err)
				}
				model.applyOutlineBuffer(outlineApplyPending{project: project, revision: revision, scopeTaskIDs: []string{task.ID}, lines: []app.OutlinePlanLine{
					{SourceLine: 1, TaskID: task.ID, Title: task.Title, Destination: "doing"},
					{SourceLine: 2, Title: "outline child", Destination: "waiting"},
				}})
				model.drainCheckInOffer()
			}
			if model.operations.CheckInEditor == nil {
				t.Fatal("edit omitted offer")
			}
		})
	}
}

func TestCheckInOfferRechecksFrozenTargetsBeforeOpening(t *testing.T) {
	first, second := newBulkPickerTestTask(t, 1, "first"), newBulkPickerTestTask(t, 2, "second")
	env := newAppBulkPickerEnv(t, []*models.Task{first, second})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	msg := commitOfferState(t, model, bulkTaskActionTestContext(env.projects[0], first, second), models.StateDoing)
	// A different writer schedules one target before this commit result arrives.
	next := time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local)
	if _, err := model.domain.service.BulkSetCheckIn([]app.TaskLocator{taskref.For(second, env.projects[0]).Locator()}, models.CheckInSchedule{Next: &next}); err != nil {
		t.Fatal(err)
	}
	model.Update(msg)
	editor := model.operations.CheckInEditor
	if editor == nil {
		t.Fatal("eligible remainder did not offer")
	}
	editor.HandlePaste(tea.PasteMsg{Content: "2027-01-02"})
	_, _, selected := editor.HandleKey(syntheticCtrlKey('s'))
	targets := selected.(operation.OperationValueSelectedMsg).Request.Context.Selection.Tasks
	if len(targets) != 1 || targets[0].TaskID != first.ID {
		t.Fatalf("stale targets=%+v", targets)
	}
}

func TestCheckInOfferWaitsForNestedOverlayAndCancelsOnInput(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "close", true: "navigate"}[cancel], func(t *testing.T) {
			task := newBulkPickerTestTask(t, 1, "task")
			env := newAppBulkPickerEnv(t, []*models.Task{task})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			context := bulkTaskActionTestContext(env.projects[0], task)
			blocker, err := model.NewCheckInEditor(operation.OperationExecutionRequest{ID: operation.OperationTaskSetCheckIn, Context: context})
			if err != nil {
				t.Fatal(err)
			}
			model.operations.CheckInEditor = blocker
			model.Update(commitOfferState(t, model, context, models.StateDoing))
			if model.operations.CheckInEditor != blocker || len(model.checkInOffers.pending) != 1 {
				t.Fatal("offer replaced the existing overlay")
			}
			if cancel {
				model.beginCheckInOfferUpdate(syntheticSpecialKey(tea.KeyDown))
			}
			model.operations.CheckInEditor = nil
			model.Update(nil)
			if (model.operations.CheckInEditor == nil) != cancel {
				t.Fatalf("offer after closing overlay: cancel=%v", cancel)
			}
		})
	}
}

func TestCheckInOfferOpenAfterFailureCancelsPendingSetup(t *testing.T) {
	env := newAppBulkPickerEnv(t, []*models.Task{})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	features := coordinator.NewFeatureCoordinators(model.domain.service)
	cmd := features.Capture.CaptureCreate(app.CaptureRequest{ProjectID: env.projects[0].ID, Title: "new", Destination: app.CaptureDestination(models.StateDoing)}, true)
	model.Update(runCheckInOfferCommand(checkInOfferCommandMsg{cmd: cmd, epoch: model.checkInOffers.epoch})())
	if len(model.checkInOffers.pending) != 1 {
		t.Fatal("missing pending setup")
	}
	model.Update(TaskContentEditedMsg{Err: errors.New("editor canceled")})
	if len(model.checkInOffers.pending) != 0 || model.operations.CheckInEditor != nil {
		t.Fatal("failed open-after produced a delayed offer")
	}
	if len(env.reload(t, 0).Tasks) != 1 {
		t.Fatal("canceling setup removed committed creation")
	}
}

func TestCheckInOfferUnreadableCommittedProjectDoesNotUseStaleCache(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "task")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	msg := commitOfferState(t, model, bulkTaskActionTestContext(env.projects[0], task), models.StateDoing)
	if err := os.Remove(filepath.Join(env.projects[0].Path, "tasks.md")); err != nil {
		t.Fatal(err)
	}
	model.Update(msg)
	if model.operations.CheckInEditor != nil {
		t.Fatal("failed reload offered against a stale cached task")
	}
}

func TestCheckInOfferCommandOriginSurvivesDelayedExecution(t *testing.T) {
	for _, navigate := range []bool{false, true} {
		t.Run(map[bool]string{false: "offer", true: "navigation"}[navigate], func(t *testing.T) {
			task := newBulkPickerTestTask(t, 1, "task")
			env := newAppBulkPickerEnv(t, []*models.Task{task})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			result := model.operations.Executor.Execute(operation.OperationExecutionRequest{ID: operation.OperationTaskSetState, Context: bulkTaskActionTestContext(env.projects[0], task), Value: models.StateDoing, HasValue: true})
			if result.Err != nil {
				t.Fatal(result.Err)
			}
			command := result.Message.(operation.OperationCommandResultMsg).Cmd
			pending := withCheckInOfferOrigin(command, model.checkInOffers.epoch)()
			if navigate {
				model.beginCheckInOfferUpdate(syntheticSpecialKey(tea.KeyDown))
			}
			_, execute := model.Update(pending)
			if env.reload(t, 0).GetTaskByID(task.ID).State() != models.StateBacklog {
				t.Fatal("scheduling command committed eagerly")
			}
			model.Update(execute())
			if env.reload(t, 0).GetTaskByID(task.ID).State() != models.StateDoing {
				t.Fatal("navigation canceled the authorized state change")
			}
			if (model.operations.CheckInEditor == nil) != navigate {
				t.Fatalf("offer after delayed command: navigate=%v", navigate)
			}
		})
	}
}
