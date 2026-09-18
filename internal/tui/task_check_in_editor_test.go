package tui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/operationhost"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func registerCheckInTestOperations(t *testing.T, model *AppModel) {
	t.Helper()
	model.operations.Registry = operation.NewOperationRegistry()
	model.operations.Executor = operation.NewOperationExecutor(model.operations.Registry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
}

func executeCheckInValue(
	t *testing.T,
	model *AppModel,
	context operation.CommandContext,
	value operation.OperationCheckInValue,
) coordinator.BatchTransactionResultMsg {
	t.Helper()
	result := model.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID: operation.OperationTaskSetCheckIn, Context: context, Value: value, HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("execute set check-in: %v", result.Err)
	}
	command, ok := result.Message.(operation.OperationCommandResultMsg)
	if !ok {
		t.Fatalf("result message = %T", result.Message)
	}
	return runBatchResult(t, command.Cmd)
}

func TestCheckInTaskActionCommitsBulkReplacementAndUndo(t *testing.T) {
	first := newBulkPickerTestTask(t, 1, "first")
	second := newBulkPickerTestTask(t, 1, "second")
	oldFirst := time.Date(2026, 9, 15, 0, 0, 0, 0, time.Local)
	oldSecond := time.Date(2026, 9, 16, 0, 0, 0, 0, time.Local)
	first.NextCheckIn, first.CheckInRepeatDays = &oldFirst, 3
	second.NextCheckIn, second.CheckInRepeatDays = &oldSecond, 14
	env := newAppBulkPickerEnv(t, []*models.Task{first}, []*models.Task{second})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	context := bulkTaskActionTestContext(env.projects[0], first)
	context.Selection.Tasks = append(context.Selection.Tasks, taskref.For(second, env.projects[1]))
	context.Selection.Entities = append(context.Selection.Entities, outliner.TaskOperationEntity(second, env.projects[1]))

	next := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	batch := executeCheckInValue(t, model, context, operation.OperationCheckInValue{
		Schedule:  models.CheckInSchedule{Next: &next, RepeatDays: 7},
		DateInput: "2026-10-01", RepeatInput: "7",
	})
	if len(batch.TouchedProjectIDs) != 2 || batch.TransactionID == "" {
		t.Fatalf("batch = %+v, want one two-project transaction", batch)
	}
	for index := range 2 {
		stored := env.reload(t, index).Tasks[0]
		if stored.NextCheckIn == nil || stored.NextCheckIn.Format(time.DateOnly) != "2026-10-01" || stored.CheckInRepeatDays != 7 {
			t.Fatalf("project %d schedule = %v/%d", index, stored.NextCheckIn, stored.CheckInRepeatDays)
		}
	}

	// Clear is the same operation and the newest ledger entry restores it whole.
	executeCheckInValue(t, model, context, operation.OperationCheckInValue{Clear: true})
	for index := range 2 {
		if stored := env.reload(t, index).Tasks[0]; !stored.CheckIn().IsEmpty() {
			t.Fatalf("project %d clear left %+v", index, stored.CheckIn())
		}
	}
	env.undo(t)
	for index := range 2 {
		stored := env.reload(t, index).Tasks[0]
		if stored.NextCheckIn == nil || stored.NextCheckIn.Format(time.DateOnly) != "2026-10-01" || stored.CheckInRepeatDays != 7 {
			t.Fatalf("project %d undo restored %v/%d", index, stored.NextCheckIn, stored.CheckInRepeatDays)
		}
	}
}

func TestCheckInOperationHostPrefillsSingleAndLeavesBulkBlank(t *testing.T) {
	first := newBulkPickerTestTask(t, 1, "first")
	second := newBulkPickerTestTask(t, 2, "second")
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	first.NextCheckIn, first.CheckInRepeatDays = &day, 14
	env := newAppBulkPickerEnv(t, []*models.Task{first, second})
	model := env.appModel()
	registerCheckInTestOperations(t, model)

	open := func(context operation.CommandContext) {
		result := model.operations.Executor.Execute(operation.OperationExecutionRequest{
			ID: operation.OperationTaskSetCheckIn, Context: context,
		})
		if result.ValueRequired == nil {
			t.Fatalf("operation result = %+v, want editor request", result)
		}
		model.operations.OpenValueStep(model, *result.ValueRequired)
	}
	open(taskOperationTestContext(first, env.projects[0]))
	if editor := model.operations.CheckInEditor; editor == nil || editor.DateInput() != "2026-09-20" || editor.RepeatInput() != "14" {
		t.Fatalf("single editor = %#v", editor)
	}
	model.operations.ClearValueStep()

	open(bulkTaskActionTestContext(env.projects[0], first, second))
	if editor := model.operations.CheckInEditor; editor == nil || editor.DateInput() != "" || editor.RepeatInput() != "" {
		t.Fatalf("bulk editor borrowed a target value: %#v", editor)
	}
}

func TestRejectedCheckInSaveReopensExactDraft(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "first")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	model := env.appModel()
	request := operation.OperationExecutionRequest{
		ID: operation.OperationTaskSetCheckIn, Context: taskOperationTestContext(task, env.projects[0]),
	}
	wantErr := errors.New("task changed while the editor was open")
	want := operation.OperationCheckInValue{
		Schedule:  models.CheckInSchedule{Next: func() *time.Time { d := time.Date(2026, 9, 21, 0, 0, 0, 0, time.Local); return &d }(), RepeatDays: 45},
		DateInput: "Sep 21 2026", RepeatInput: "45",
	}
	cmd := operationhost.RetainCheckInDraftOnFailure(request, want, func() tea.Msg {
		return coordinator.TransactionErrorMsg{Err: wantErr}
	})
	msg, ok := cmd().(operationhost.CheckInRejectedMsg)
	if !ok {
		t.Fatalf("failure wrapper returned %T", cmd())
	}
	handled, _ := model.handleOperationOverlayMessage(msg)
	editor := model.operations.CheckInEditor
	if !handled || editor == nil {
		t.Fatal("rejected save did not reopen the editor")
	}
	if editor.DateInput() != want.DateInput || editor.RepeatInput() != want.RepeatInput || editor.Notice() != wantErr.Error() {
		t.Fatalf("restored draft = %q/%q notice %q", editor.DateInput(), editor.RepeatInput(), editor.Notice())
	}
}

// TestExplicitCheckInRejectionReopensOnlyItsOwnInteraction drives a refused
// explicit save through the shell, where interaction ownership alone decides
// whether the draft may come back. Explicit editing is now the only way into
// this editor, so the protection this pins is the whole of what ownership buys.
func TestExplicitCheckInRejectionReopensOnlyItsOwnInteraction(t *testing.T) {
	for _, movedOn := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "moved-on"}[movedOn], func(t *testing.T) {
			task := newBulkPickerTestTask(t, 1, "first")
			env := newAppBulkPickerEnv(t, []*models.Task{task})
			model := env.appModel()
			registerCheckInTestOperations(t, model)
			request := operation.OperationExecutionRequest{
				ID: operation.OperationTaskSetCheckIn, Context: taskOperationTestContext(task, env.projects[0]),
			}
			wantErr := errors.New("task changed while the editor was open")
			day := time.Date(2026, 9, 21, 0, 0, 0, 0, time.Local)
			want := operation.OperationCheckInValue{
				Schedule:  models.CheckInSchedule{Next: &day, RepeatDays: 45},
				DateInput: "2026-09-21", RepeatInput: "45",
			}
			submit := withCommandOrigin(operationhost.RetainCheckInDraftOnFailure(request, want, func() tea.Msg {
				return coordinator.TransactionErrorMsg{Err: wantErr}
			}), model.interactionEpoch())
			tagged, ok := submit().(originTaggedCommandMsg)
			if !ok {
				t.Fatalf("submitting command lost its interaction: %T", submit())
			}
			refusal := runOriginTaggedCommand(tagged)()
			if _, carriesOrigin := refusal.(commandOriginMsg); !carriesOrigin {
				t.Fatalf("delayed refusal arrived untagged: %T", refusal)
			}
			if movedOn {
				model.Update(syntheticSpecialKey(tea.KeyDown))
			}
			model.Update(refusal)
			editor := model.operations.CheckInEditor
			if movedOn {
				if editor != nil {
					t.Fatal("a stale refusal reopened the editor after the user moved on")
				}
				return
			}
			if editor == nil {
				t.Fatal("an owned refusal did not reopen the editor")
			}
			if editor.DateInput() != want.DateInput || editor.RepeatInput() != want.RepeatInput || editor.Notice() != wantErr.Error() {
				t.Fatalf("restored draft = %q/%q notice %q", editor.DateInput(), editor.RepeatInput(), editor.Notice())
			}
			// The frozen targets travel with the draft, so resubmitting the restored
			// editor still names the task the refused save was aimed at.
			_, dismissed, selected := editor.HandleKey(syntheticCtrlKey('s'))
			resubmitted, ok := selected.(operation.OperationValueSelectedMsg)
			if !dismissed || !ok {
				t.Fatalf("restored draft did not resubmit: dismissed=%v msg=%T", dismissed, selected)
			}
			if resubmitted.Request.ID != request.ID || resubmitted.Request.Context.Entity != request.Context.Entity {
				t.Fatalf("resubmission lost the frozen targets: %+v", resubmitted.Request.Context.Entity)
			}
		})
	}
}
