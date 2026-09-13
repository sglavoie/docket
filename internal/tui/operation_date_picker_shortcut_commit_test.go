package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
	"github.com/sglavoie/tt/internal/util"
)

// datePickerShortcutEnv opens a real date picker over one real task, through the
// same factory and the same executor the shell uses. What a preset key is worth
// is what it writes to disk, and nothing short of the whole path proves that.
type datePickerShortcutEnv struct {
	env    *appBulkPickerEnv
	model  *AppModel
	task   *models.Task
	picker *valuepicker.OperationValuePickerModel
}

func newDatePickerShortcutEnv(t *testing.T, provider operation.ValueProviderID) *datePickerShortcutEnv {
	t.Helper()
	task := newBulkPickerTestTask(t, 1, "dated task")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	model := env.appModel()
	model.operations.Registry = operation.NewOperationRegistry()
	model.operations.Executor = operation.NewOperationExecutor(model.operations.Registry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}

	context := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	project := env.projects[0]
	context.Selection.Tasks = append(context.Selection.Tasks, taskref.For(task, project))
	context.Selection.Entities = append(
		context.Selection.Entities, outliner.TaskOperationEntity(task, project),
	)
	context.Entity = context.Selection.Entities[0]

	picker, err := model.newOperationValuePicker(operation.OperationExecutionRequest{
		ID:      operation.OperationTaskSetDueDate,
		Context: context,
	}, provider)
	if err != nil {
		t.Fatalf("newOperationValuePicker: %v", err)
	}
	return &datePickerShortcutEnv{env: env, model: model, task: task, picker: picker}
}

// press sends one key to the open picker and commits whatever it answers with,
// through the executor the shell would hand the message to.
func (e *datePickerShortcutEnv) press(t *testing.T, key string) {
	t.Helper()
	e.pressBatch(t, key)
}

// pressBatch is press for the callers that need the committed batch itself.
// Which projects one keystroke touched is the only evidence that a selection
// spanning two of them was committed as one transaction.
func (e *datePickerShortcutEnv) pressBatch(
	t *testing.T,
	key string,
) coordinator.BatchTransactionResultMsg {
	t.Helper()
	return runBatchResult(t, e.selectCmd(t, key))
}

// pressRefusal is press for the keys the application layer must refuse. It
// returns the refusal so a caller can hold it to naming the reason, and it
// fails the test if the transaction committed instead.
func (e *datePickerShortcutEnv) pressRefusal(t *testing.T, key string) error {
	t.Helper()
	msg := e.selectCmd(t, key)()
	failure, ok := msg.(coordinator.TransactionErrorMsg)
	if !ok {
		t.Fatalf("%q committed instead of refusing: %#v", key, msg)
	}
	if failure.Err == nil {
		t.Fatalf("%q refused with no error", key)
	}
	return failure.Err
}

// selectCmd presses one key, requires the picker to answer with a value, and
// resumes the frozen request through the one invocation pipeline. The returned
// command is the transaction: nothing is written until it runs.
func (e *datePickerShortcutEnv) selectCmd(t *testing.T, key string) tea.Cmd {
	t.Helper()
	_, dismissed, result := e.picker.HandleKey(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
	if !dismissed {
		t.Fatalf("%q left the picker open: %#v", key, result)
	}
	selected, ok := result.(operation.OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("%q answered with %#v, want a selected value", key, result)
	}
	request := selected.Request
	request.Value = selected.Value
	request.HasValue = true
	outcome := e.model.operations.Executor.Execute(request)
	if outcome.Err != nil {
		t.Fatalf("executing the %q shortcut: %v", key, outcome.Err)
	}
	command, ok := outcome.Message.(operation.OperationCommandResultMsg)
	if !ok {
		t.Fatalf("%q produced %T, want an OperationCommandResultMsg", key, outcome.Message)
	}
	return command.Cmd
}

// pressRefusedRow presses a key whose row the picker itself refuses. The
// refusal is the picker's own status and the picker stays open, so no request
// reaches the executor and there is nothing to roll back.
func (e *datePickerShortcutEnv) pressRefusedRow(t *testing.T, key string) string {
	t.Helper()
	_, dismissed, result := e.picker.HandleKey(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
	if dismissed {
		t.Fatalf("%q closed the picker instead of refusing: %#v", key, result)
	}
	message, ok := result.(status.StatusMsg)
	if !ok {
		t.Fatalf("%q answered with %#v, want a refusal status", key, result)
	}
	return string(message)
}

// crossField moves to the other date field. Crossing is a move rather than a
// choice, so the picker stays open and nothing is committed.
func (e *datePickerShortcutEnv) crossField(t *testing.T, shift bool) {
	t.Helper()
	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	if shift {
		msg.Mod = tea.ModShift
	}
	if _, dismissed, result := e.picker.HandleKey(msg); dismissed || result != nil {
		t.Fatalf("crossing fields dismissed=%v result=%#v", dismissed, result)
	}
}

func (e *datePickerShortcutEnv) stored(t *testing.T) *models.Task {
	t.Helper()
	stored := e.env.reload(t, 0).GetTaskByID(e.task.ID)
	if stored == nil {
		t.Fatal("the task is gone from its project")
	}
	return stored
}

// The combined picker's two fields each commit their own preset, to their own
// field, and the other field is left exactly as it was.
func TestDatePresetShortcutsCommitTheFieldTheyName(t *testing.T) {
	tomorrow := util.StartOfDay(time.Now()).AddDate(0, 0, 1)

	due := newDatePickerShortcutEnv(t, operation.ValueProviderDate)
	due.press(t, "m")
	stored := due.stored(t)
	if stored.DueDate == nil || !stored.DueDate.Equal(tomorrow) {
		t.Fatalf("`m` stored due date %v, want tomorrow %s", stored.DueDate, tomorrow)
	}
	if stored.DeferredUntil != nil {
		t.Fatalf("`m` on the due field also wrote a deferral: %s", stored.DeferredUntil)
	}

	defer8 := newDatePickerShortcutEnv(t, operation.ValueProviderDate)
	if _, _, result := defer8.picker.HandleKey(tea.KeyPressMsg{Code: tea.KeyTab}); result != nil {
		t.Fatalf("tab into the defer field = %#v", result)
	}
	before := time.Now()
	defer8.press(t, "8")
	stored = defer8.stored(t)
	if stored.DeferredUntil == nil {
		t.Fatal("`8` on the defer field stored no deferral")
	}
	if stored.DeferredUntil.Before(before.Add(7*time.Hour + 55*time.Minute)) {
		t.Fatalf("`8` stored %s, want roughly eight hours from %s", stored.DeferredUntil, before)
	}
	if stored.DueDate != nil {
		t.Fatalf("`8` on the defer field also wrote a due date: %s", stored.DueDate)
	}
	// Deferring claims Backlog, which is the existing scheduling side effect and
	// not something a keyboard shortcut may quietly skip.
	if got := stored.State(); got != models.StateBacklog {
		t.Fatalf("`8` left the task in %s, want the deferral's Backlog", got)
	}
}

// `0` is bound in both fields, which is exactly why it has to be proved twice:
// clearing a deadline must leave a stored deferral alone, and clearing a
// deferral must leave a stored deadline alone.
func TestTheClearShortcutReachesOnlyTheFieldUnderTheCursor(t *testing.T) {
	clearDue := newDatePickerShortcutEnv(t, operation.ValueProviderDate)
	clearDue.press(t, "m")

	clearDue.picker = reopenDatePicker(t, clearDue)
	if _, _, result := clearDue.picker.HandleKey(tea.KeyPressMsg{Code: tea.KeyTab}); result != nil {
		t.Fatalf("tab into the defer field = %#v", result)
	}
	clearDue.press(t, "m")
	stored := clearDue.stored(t)
	if stored.DueDate == nil || stored.DeferredUntil == nil {
		t.Fatalf("setup left due = %v and defer = %v, want both set", stored.DueDate, stored.DeferredUntil)
	}
	deferral := *stored.DeferredUntil

	clearDue.picker = reopenDatePicker(t, clearDue)
	clearDue.press(t, "0")
	stored = clearDue.stored(t)
	if stored.DueDate != nil {
		t.Fatalf("`0` in the due field left due date %s standing", stored.DueDate)
	}
	if stored.DeferredUntil == nil || !stored.DeferredUntil.Equal(deferral) {
		t.Fatalf("`0` in the due field moved the deferral to %v, want %s", stored.DeferredUntil, deferral)
	}

	clearDue.picker = reopenDatePicker(t, clearDue)
	if _, _, result := clearDue.picker.HandleKey(tea.KeyPressMsg{Code: tea.KeyTab}); result != nil {
		t.Fatalf("tab into the defer field = %#v", result)
	}
	clearDue.press(t, "0")
	if stored := clearDue.stored(t); stored.DeferredUntil != nil {
		t.Fatalf("`0` in the defer field left deferral %s standing", stored.DeferredUntil)
	}
}

// A committed picker is spent, so the next key needs a fresh one over the task
// as it now stands — which is also what the shell does between two operations.
// It reopens the combined picker specifically: the due-only projection is a
// chained step the executor opens for itself, never one a test reopens by hand.
func reopenDatePicker(
	t *testing.T,
	env *datePickerShortcutEnv,
) *valuepicker.OperationValuePickerModel {
	t.Helper()
	request := env.picker.ExecutionRequest()
	picker, err := env.model.newOperationValuePicker(operation.OperationExecutionRequest{
		ID:      request.ID,
		Context: request.Context,
	}, operation.ValueProviderDate)
	if err != nil {
		t.Fatalf("reopening the picker: %v", err)
	}
	return picker
}

// The due-only projection answers the due keys and commits a real deadline.
func TestDueOnlyPickerShortcutCommitsADeadline(t *testing.T) {
	week := util.StartOfDay(time.Now()).AddDate(0, 0, 7)
	env := newDatePickerShortcutEnv(t, operation.ValueProviderDueDate)
	env.press(t, "w")
	stored := env.stored(t)
	if stored.DueDate == nil || !stored.DueDate.Equal(week) {
		t.Fatalf("`w` stored due date %v, want one week out %s", stored.DueDate, week)
	}
	if stored.DeferredUntil != nil {
		t.Fatalf("the due-only picker wrote a deferral: %s", stored.DeferredUntil)
	}
}

// Undo takes a shortcut back exactly as it takes an Enter back: one entry, the
// same transaction, no separate ledger for keyboard-driven commits.
func TestADatePresetShortcutLeavesOneUndoEntry(t *testing.T) {
	env := newDatePickerShortcutEnv(t, operation.ValueProviderDate)
	env.press(t, "n")
	if env.stored(t).DueDate == nil {
		t.Fatal("`n` stored no due date")
	}
	env.env.undo(t)
	if stored := env.stored(t); stored.DueDate != nil {
		t.Fatalf("undo left the due date %s standing", stored.DueDate)
	}
}
