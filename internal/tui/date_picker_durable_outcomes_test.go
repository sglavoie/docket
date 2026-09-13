package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
)

// What a date keystroke means is what survives the transaction it commits, and
// a picker status line cannot answer that: Today membership lives in
// planning.json, a deferral's Backlog claim lives in tasks.md, and both have to
// be read back from disk. These tests drive the real picker, the real executor,
// and a service carrying the planning builder app_init wires, then reload the
// workspace and ask it.

// durableDatePickerEnv extends the shortcut harness with the whole selection
// and the projects it spans, so a test can name any target and read either
// durable domain back.
type durableDatePickerEnv struct {
	*datePickerShortcutEnv
	tasks []*models.Task
	// owners maps each task to the index of the project holding it, which is
	// what makes a cross-project selection addressable in reload order.
	owners []int
}

// newPlanningWiredModel builds the workspace and the shell the date operations
// run through. The planning builder is the same one app_init installs: without
// it every today-dated commit is refused, because planning is not an optional
// consequence of a deadline.
func newPlanningWiredModel(t *testing.T, groups ...[]*models.Task) (*appBulkPickerEnv, *AppModel) {
	t.Helper()
	env := newAppBulkPickerEnv(t, groups...)
	env.svc.WithPlanningArtifacts(coordinator.ManagerPlanningArtifacts(env.mgr))
	model := env.appModel()
	model.operations.Registry = operation.NewOperationRegistry()
	model.operations.Executor = operation.NewOperationExecutor(model.operations.Registry)
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	return env, model
}

// flattenTaskGroups pairs every task with the project index it was seeded into.
func flattenTaskGroups(groups [][]*models.Task) ([]*models.Task, []int) {
	tasks := make([]*models.Task, 0, len(groups))
	owners := make([]int, 0, len(groups))
	for index, group := range groups {
		for _, task := range group {
			tasks = append(tasks, task)
			owners = append(owners, index)
		}
	}
	return tasks, owners
}

// newDurableDatePickerEnv opens one date picker over every seeded task. One
// group per project, so a two-group call is the cross-project selection.
func newDurableDatePickerEnv(
	t *testing.T,
	provider operation.ValueProviderID,
	groups ...[]*models.Task,
) *durableDatePickerEnv {
	t.Helper()
	env, model := newPlanningWiredModel(t, groups...)
	tasks, owners := flattenTaskGroups(groups)
	if len(tasks) == 0 {
		t.Fatal("a date picker needs at least one target")
	}

	context := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	context.Selection.Visual = len(tasks) > 1
	for index, task := range tasks {
		project := env.projects[owners[index]]
		context.Selection.Tasks = append(context.Selection.Tasks, taskref.For(task, project))
		context.Selection.Entities = append(
			context.Selection.Entities, outliner.TaskOperationEntity(task, project),
		)
	}
	context.Entity = context.Selection.Entities[0]

	picker, err := model.newOperationValuePicker(operation.OperationExecutionRequest{
		ID:      operation.OperationTaskSetDueDate,
		Context: context,
	}, provider)
	if err != nil {
		t.Fatalf("newOperationValuePicker: %v", err)
	}
	return &durableDatePickerEnv{
		datePickerShortcutEnv: &datePickerShortcutEnv{
			env: env, model: model, task: tasks[0], picker: picker,
		},
		tasks:  tasks,
		owners: owners,
	}
}

// newFrozenSelectionDateEnv is newDurableDatePickerEnv over a real Outliner,
// selecting the first `selected` tasks and leaving the rest addressable by the
// cursor alone. It is what makes "the picker commits the selection it froze" a
// claim a test can refute: the cursor can be moved onto a task nobody selected
// while the picker is open.
func newFrozenSelectionDateEnv(
	t *testing.T,
	provider operation.ValueProviderID,
	selected int,
	groups ...[]*models.Task,
) (*durableDatePickerEnv, *outliner.OutlinerModel) {
	t.Helper()
	env, model := newPlanningWiredModel(t, groups...)
	tasks, owners := flattenTaskGroups(groups)
	if selected < 1 || selected > len(tasks) {
		t.Fatalf("selected = %d, want between 1 and %d", selected, len(tasks))
	}

	view := outliner.NewOutlinerModel(nil, model.domain.manager, model.domain.runtime)
	view.Population.Projects = env.projects
	view.MultiSelectMode = true
	for index, task := range tasks {
		project := env.projects[owners[index]]
		view.Rows = append(view.Rows, outliner.OutlinerRow{Task: task, ProjectID: project.ID})
		if index < selected {
			view.SelectTaskIdentity(taskref.For(task, project))
		}
	}
	model.crossOutliner = view
	model.viewNav.views.Register(presentation.ViewCrossProjectOutliner, view)

	picker, err := model.newOperationValuePicker(operation.OperationExecutionRequest{
		ID:      operation.OperationTaskSetDueDate,
		Context: model.operationCommandContext(),
	}, provider)
	if err != nil {
		t.Fatalf("newOperationValuePicker: %v", err)
	}
	return &durableDatePickerEnv{
		datePickerShortcutEnv: &datePickerShortcutEnv{
			env: env, model: model, task: tasks[0], picker: picker,
		},
		tasks:  tasks,
		owners: owners,
	}, view
}

// reopen replaces the spent picker with one built over the tasks as they now
// stand, so a second keystroke in one test reads the durable state the first
// one left.
func (e *durableDatePickerEnv) reopen(t *testing.T) {
	t.Helper()
	e.picker = reopenDatePicker(t, e.datePickerShortcutEnv)
}

// storedTask reads one target back out of its own project's tasks.md.
func (e *durableDatePickerEnv) storedTask(t *testing.T, index int) *models.Task {
	t.Helper()
	stored := e.env.reload(t, e.owners[index]).GetTaskByID(e.tasks[index].ID)
	if stored == nil {
		t.Fatalf("task %q is gone from its project", e.tasks[index].Title)
	}
	return stored
}

// plannedToday reports whether the durable planning domain still names the
// first target in Today, project-qualified the way the collection stores it.
// Today membership is what a single keystroke writes or gives up, so the
// question is always asked about the one task that keystroke was aimed at.
func (e *durableDatePickerEnv) plannedToday(t *testing.T) bool {
	t.Helper()
	planning, err := e.env.mgr.DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	project := e.env.projects[e.owners[0]]
	for _, item := range planning.GetToday() {
		if item.TaskID == e.tasks[0].ID && item.ProjectID == project.ID {
			return true
		}
	}
	return false
}

// removeFromWorkspace deletes one target from the workspace while the picker
// holds it, which is the race the frozen request has to survive: another writer
// removed the task between opening the picker and pressing a key.
func (e *durableDatePickerEnv) removeFromWorkspace(t *testing.T, index int) {
	t.Helper()
	project := e.env.projects[e.owners[index]]
	remaining := make([]*models.Task, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		if task.ID != e.tasks[index].ID {
			remaining = append(remaining, task)
		}
	}
	project.Tasks = remaining
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("removing the task from its project: %v", err)
	}
}

// completeInWorkspace finishes one target while the picker holds it, which is
// the other half of the race: the task still resolves, but it is no longer the
// task the picker's rows were built against.
func (e *durableDatePickerEnv) completeInWorkspace(t *testing.T, index int) {
	t.Helper()
	e.tasks[index].Complete()
	project := e.env.projects[e.owners[index]]
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("completing the task in its project: %v", err)
	}
}

// A due date of today carries planning intent, so `t` writes tasks.md and
// planning.json in one transaction — and one ledger entry moves both together
// in either direction.
func TestDueTodayPresetKeyWritesTodayPlanningThatUndoAndRedoMoveWithIt(t *testing.T) {
	today := util.StartOfDay(time.Now())
	env := newDurableDatePickerEnv(
		t, operation.ValueProviderDate, []*models.Task{newBulkPickerTestTask(t, 1, "plan me")},
	)
	env.press(t, "t")

	stored := env.storedTask(t, 0)
	if stored.DueDate == nil || !stored.DueDate.Equal(today) {
		t.Fatalf("`t` stored due date %v, want today %s", stored.DueDate, today)
	}
	if !env.plannedToday(t) {
		t.Fatal("`t` set a deadline for today without planning the task for today")
	}

	env.env.undo(t)
	if stored := env.storedTask(t, 0); stored.DueDate != nil {
		t.Fatalf("undo left the due date %s standing", stored.DueDate)
	}
	if env.plannedToday(t) {
		t.Fatal("undo took the due date back and left the Today membership behind")
	}

	env.env.redo(t)
	if stored := env.storedTask(t, 0); stored.DueDate == nil || !stored.DueDate.Equal(today) {
		t.Fatalf("redo restored due date %v, want today %s", stored.DueDate, today)
	}
	if !env.plannedToday(t) {
		t.Fatal("redo restored the due date without the Today membership it was committed with")
	}
}

func TestCheckInSectionOpensItsEditorAndCheckInTodayDoesNotPlanToday(t *testing.T) {
	env := newDurableDatePickerEnv(
		t, operation.ValueProviderDate, []*models.Task{newBulkPickerTestTask(t, 1, "review without planning")},
	)
	env.model.operations.ValuePicker = env.picker
	for range 2 { // Due -> Defer -> Check-in.
		if _, dismissed, result := env.picker.HandleKey(tea.KeyPressMsg{Code: tea.KeyTab}); dismissed || result != nil {
			t.Fatalf("Tab to Check-in dismissed=%v result=%#v", dismissed, result)
		}
	}
	_, dismissed, result := env.picker.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	request, ok := result.(operation.OperationValueRequestMsg)
	if !dismissed || !ok {
		t.Fatalf("Check-in row dismissed=%v result=%T", dismissed, result)
	}
	handled, init := env.model.handleOperationOverlayMessage(request)
	if !handled || init == nil || env.model.operations.ValuePicker != nil || env.model.operations.CheckInEditor == nil {
		t.Fatalf("editor handoff handled=%v init=%v picker=%v editor=%v", handled, init != nil, env.model.operations.ValuePicker, env.model.operations.CheckInEditor)
	}

	editor := env.model.operations.CheckInEditor
	editor.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter}) // open date presets
	editor.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter}) // choose today
	_, dismissed, result = editor.HandleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	selected, ok := result.(operation.OperationValueSelectedMsg)
	if !dismissed || !ok {
		t.Fatalf("editor save dismissed=%v result=%T", dismissed, result)
	}
	handled, cmd := env.model.handleOperationOverlayMessage(selected)
	if !handled || cmd == nil {
		t.Fatal("editor save did not resume the check-in operation")
	}
	runBatchResult(t, cmd)

	stored := env.storedTask(t, 0)
	if stored.NextCheckIn == nil || stored.NextCheckIn.Format(time.DateOnly) != time.Now().Format(time.DateOnly) {
		t.Fatalf("stored check-in = %v, want today", stored.NextCheckIn)
	}
	if stored.DueDate != nil || stored.DeferredUntil != nil {
		t.Fatalf("check-in action changed due/defer = %v/%v", stored.DueDate, stored.DeferredUntil)
	}
	if env.plannedToday(t) {
		t.Fatal("check-in today added the task to Today")
	}
}

// Deferring is a scheduling act, not an attribute edit: it claims Backlog and
// gives up Today. The due date it does not name is left exactly as it was.
func TestADeferPresetKeyTakesTheTaskOutOfTodayAndClaimsBacklog(t *testing.T) {
	today := util.StartOfDay(time.Now())
	tomorrow := today.AddDate(0, 0, 1)
	task := newBulkPickerTestTask(t, 1, "deferred out of today")
	task.Start()
	env := newDurableDatePickerEnv(t, operation.ValueProviderDate, []*models.Task{task})

	env.press(t, "t")
	if !env.plannedToday(t) {
		t.Fatal("setup did not plan the task for today")
	}

	env.reopen(t)
	env.crossField(t, false)
	env.press(t, "m")

	stored := env.storedTask(t, 0)
	if stored.DeferredUntil == nil || !stored.DeferredUntil.Equal(tomorrow) {
		t.Fatalf("`m` in the defer field stored %v, want tomorrow %s", stored.DeferredUntil, tomorrow)
	}
	if stored.DueDate == nil || !stored.DueDate.Equal(today) {
		t.Fatalf("deferring moved the due date to %v, want today %s", stored.DueDate, today)
	}
	if got := stored.State(); got != models.StateBacklog {
		t.Fatalf("deferred task is %s, want the deferral's Backlog", got)
	}
	if env.plannedToday(t) {
		t.Fatal("a task hidden until tomorrow is still planned for today")
	}

	// The planning consequence rode the same transaction, so one undo owes both.
	env.env.undo(t)
	stored = env.storedTask(t, 0)
	if stored.DeferredUntil != nil {
		t.Fatalf("undo left the deferral %s standing", stored.DeferredUntil)
	}
	if !env.plannedToday(t) {
		t.Fatal("undo took the deferral back without restoring the Today membership it removed")
	}
}

// Clearing a deadline is an attribute edit and nothing more. Today is an
// explicit membership rather than a shadow of the date, so it stands.
func TestClearingADueDateLeavesTheTodayMembershipStanding(t *testing.T) {
	env := newDurableDatePickerEnv(
		t, operation.ValueProviderDate, []*models.Task{newBulkPickerTestTask(t, 1, "planned and undated")},
	)
	env.press(t, "t")
	if !env.plannedToday(t) {
		t.Fatal("setup did not plan the task for today")
	}
	before := env.storedTask(t, 0).State()

	env.reopen(t)
	env.press(t, "0")

	stored := env.storedTask(t, 0)
	if stored.DueDate != nil {
		t.Fatalf("`0` in the due field left due date %s standing", stored.DueDate)
	}
	if !env.plannedToday(t) {
		t.Fatal("clearing the deadline also unplanned the task, which is a second axis")
	}
	if got := stored.State(); got != before {
		t.Fatalf("clearing a due date moved the task to %s, want %s", got, before)
	}
}

// Clearing a deferral preserves the lifecycle the deferral left behind: it
// un-hides the task, it does not resurrect Doing, and it invents no planning
// membership on the way back.
func TestClearingADeferralPreservesTheLifecycleTheDeferralLeft(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "hidden then revealed")
	task.Start()
	env := newDurableDatePickerEnv(t, operation.ValueProviderDate, []*models.Task{task})

	env.crossField(t, false)
	env.press(t, "1")
	if stored := env.storedTask(t, 0); stored.DeferredUntil == nil || stored.State() != models.StateBacklog {
		t.Fatalf("setup deferral = %v in %s, want a deferral in Backlog", stored.DeferredUntil, stored.State())
	}

	env.reopen(t)
	env.crossField(t, false)
	env.press(t, "0")

	stored := env.storedTask(t, 0)
	if stored.DeferredUntil != nil {
		t.Fatalf("`0` in the defer field left deferral %s standing", stored.DeferredUntil)
	}
	if got := stored.State(); got != models.StateBacklog {
		t.Fatalf("clearing the deferral moved the task to %s, want the Backlog it was left in", got)
	}
	if env.plannedToday(t) {
		t.Fatal("clearing a deferral planned the task for today")
	}

	// A revealed task is plannable again, which is the point of clearing.
	env.reopen(t)
	env.press(t, "t")
	if !env.plannedToday(t) {
		t.Fatal("a task whose deferral was cleared cannot be planned for today")
	}
}

// A selection mixing a terminal task with an open one cannot be deferred, and
// the whole defer field says so rather than deferring the half that qualifies.
// Due stays reachable on that same selection, which is the point of refusing
// per field instead of per picker.
func TestAMixedTerminalSelectionRefusesTheDeferralWithoutAPartialWrite(t *testing.T) {
	week := util.StartOfDay(time.Now()).AddDate(0, 0, 7)
	open := newBulkPickerTestTask(t, 1, "still open")
	done := newBulkPickerTestTask(t, 2, "already done")
	done.Complete()
	env := newDurableDatePickerEnv(t, operation.ValueProviderDate, []*models.Task{open, done})

	env.crossField(t, false)
	if reason := env.pressRefusedRow(t, "m"); !strings.Contains(reason, "cannot defer a done task") {
		t.Fatalf("the defer field refused with %q, want the done task named", reason)
	}
	for index := range env.tasks {
		if stored := env.storedTask(t, index); stored.DeferredUntil != nil {
			t.Fatalf("the refused deferral still wrote %s to %q", stored.DeferredUntil, stored.Title)
		}
	}

	env.crossField(t, true)
	env.press(t, "w")
	for index := range env.tasks {
		stored := env.storedTask(t, index)
		if stored.DueDate == nil || !stored.DueDate.Equal(week) {
			t.Fatalf("%q stored due date %v, want one week out %s", stored.Title, stored.DueDate, week)
		}
	}
	if got := env.storedTask(t, 1).State(); got != models.StateDone {
		t.Fatalf("a deadline moved the terminal task to %s, want %s", got, models.StateDone)
	}
}

// The request froze its targets when the picker opened. A target that no longer
// resolves is the one authoritative refusal, and the task the cursor happens to
// name is not a substitute for it.
func TestATargetRemovedWhileThePickerIsOpenRefusesInsteadOfFallingBackToTheCursor(t *testing.T) {
	doomed := newBulkPickerTestTask(t, 1, "removed underneath")
	bystander := newBulkPickerTestTask(t, 2, "never selected")
	env, view := newFrozenSelectionDateEnv(
		t, operation.ValueProviderDate, 1, []*models.Task{doomed, bystander},
	)

	// The cursor walks onto the unselected row while the picker is open, which is
	// exactly the fallback this refusal must not take.
	view.Cursor = 1
	env.removeFromWorkspace(t, 0)

	err := env.pressRefusal(t, "w")
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("refusal = %v, want %v", err, app.ErrTaskNotFound)
	}
	if stored := env.storedTask(t, 1); stored.DueDate != nil {
		t.Fatalf("the refused commit dated the cursor's task instead: %s", stored.DueDate)
	}
}

// A row the picker drew as available is not a promise. A target that finished
// while the picker was open is re-judged against a fresh snapshot at commit
// time, and one ineligible member refuses the whole batch: the open task beside
// it keeps its undeferred state rather than taking half the keystroke.
func TestATargetCompletedWhileThePickerIsOpenRefusesTheWholeDeferral(t *testing.T) {
	stillOpen := newBulkPickerTestTask(t, 1, "still open at commit time")
	finished := newBulkPickerTestTask(t, 2, "finished under the picker")
	env := newDurableDatePickerEnv(t, operation.ValueProviderDate, []*models.Task{stillOpen, finished})

	// The defer rows were built while both tasks qualified, so the refusal cannot
	// come from the picker: only the application layer sees the completion.
	env.crossField(t, false)
	env.completeInWorkspace(t, 1)

	err := env.pressRefusal(t, "m")
	if !strings.Contains(err.Error(), "cannot defer a done task") {
		t.Fatalf("refusal = %v, want the done task named", err)
	}
	for index := range env.tasks {
		if stored := env.storedTask(t, index); stored.DeferredUntil != nil {
			t.Fatalf("the refused batch still deferred %q until %s", stored.Title, stored.DeferredUntil)
		}
	}
	if got := env.storedTask(t, 0).State(); got != models.StateBacklog {
		t.Fatalf("the open member is in %s, want the Backlog a refused deferral leaves it in", got)
	}
}

// One keystroke over a selection spanning two projects is one transaction: both
// projects are written, the cursor's unselected task is not, and one ledger
// entry takes the whole thing back.
func TestACrossProjectPresetKeyCommitsTheFrozenSelectionAfterTheCursorMoves(t *testing.T) {
	week := util.StartOfDay(time.Now()).AddDate(0, 0, 7)
	first := newBulkPickerTestTask(t, 1, "first project")
	second := newBulkPickerTestTask(t, 1, "second project")
	bystander := newBulkPickerTestTask(t, 2, "unselected neighbor")
	// The selection is the first two seeded tasks, one per project; the bystander
	// shares the second project and is only ever under the cursor.
	env, view := newFrozenSelectionDateEnv(
		t, operation.ValueProviderDate, 2,
		[]*models.Task{first}, []*models.Task{second, bystander},
	)

	view.Cursor = 2
	batch := env.pressBatch(t, "w")
	if len(batch.TouchedProjectIDs) != 2 {
		t.Fatalf("touched projects = %v, want both selected projects", batch.TouchedProjectIDs)
	}
	for _, index := range []int{0, 1} {
		stored := env.storedTask(t, index)
		if stored.DueDate == nil || !stored.DueDate.Equal(week) {
			t.Fatalf("%q stored due date %v, want one week out %s", stored.Title, stored.DueDate, week)
		}
	}
	if stored := env.storedTask(t, 2); stored.DueDate != nil {
		t.Fatalf("the unselected task under the cursor was dated %s", stored.DueDate)
	}

	env.env.undo(t)
	for _, index := range []int{0, 1} {
		if stored := env.storedTask(t, index); stored.DueDate != nil {
			t.Fatalf("one undo left %q dated %s, so the two projects were not one entry", stored.Title, stored.DueDate)
		}
	}
}
