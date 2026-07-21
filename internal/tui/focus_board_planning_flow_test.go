package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestPlannerAddUsesActiveTabPresetAndOneUndoableTransaction(t *testing.T) {
	for _, tc := range []struct {
		name      string
		tab       int
		wantState models.WorkflowState
		wantToday bool
		wantNext  bool
	}{
		{name: "Today", tab: boardTabToday, wantState: models.StateReady, wantToday: true},
		{name: "Next", tab: boardTabNext, wantState: models.StateNext, wantNext: true},
		{name: "Backlog", tab: boardTabBacklog, wantState: models.StateReady},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newPlanningReconcileEnv(t, models.StateReady)
			baselineUndo, err := env.ledger.UndoCount(storage.GlobalScope())
			if err != nil {
				t.Fatalf("baseline undo count: %v", err)
			}
			m := newTestFocusBoard()
			m.runtime = env.runtime
			m.configManager = env.manager
			m.filterMachine = env.runtime.Machine()
			m.capture = CaptureCoordinator{newFeatureCoordinator("capture", env.service, env.service)}
			m.acProjectCache = []*models.Project{env.project}
			m.tabs.Active = tc.tab

			_, cmd := m.createItemFromInput("Created from Planner %planning-reconcile")
			if cmd == nil {
				t.Fatal("Planner add returned no command")
			}
			raw := cmd()
			msg, ok := raw.(TransactionResultMsg)
			if !ok {
				t.Fatalf("Planner add emitted %T (%#v), want TransactionResultMsg", raw, raw)
			}
			if got := msg.PlanningChanged; got != (tc.wantToday || tc.wantNext) {
				t.Fatalf("PlanningChanged = %v, want %v", got, tc.wantToday || tc.wantNext)
			}

			assertPlannerCreatedState(t, env, msg.Result.TaskID, tc.wantState, tc.wantToday, tc.wantNext)
			if count, err := env.ledger.UndoCount(storage.GlobalScope()); err != nil || count != baselineUndo+1 {
				t.Fatalf("undo entries after Planner add = %d, err=%v, want baseline+1 (%d)", count, err, baselineUndo+1)
			}

			if _, err := env.ledger.Undo(storage.GlobalScope()); err != nil {
				t.Fatalf("undo Planner add: %v", err)
			}
			if err := storage.LoadProject(env.project); err != nil {
				t.Fatalf("reload project after undo: %v", err)
			}
			if task := env.project.GetTaskByID(msg.Result.TaskID); task != nil {
				t.Fatalf("created task survived undo: %+v", task)
			}
			planning := env.reloadPlanning(t)
			if dailyPlanContains(planning, msg.Result.TaskID) || focusQueueContains(planning, msg.Result.TaskID) {
				t.Fatal("created task's planning membership survived undo")
			}

			if _, err := env.ledger.Redo(storage.GlobalScope()); err != nil {
				t.Fatalf("redo Planner add: %v", err)
			}
			assertPlannerCreatedState(t, env, msg.Result.TaskID, tc.wantState, tc.wantToday, tc.wantNext)
		})
	}
}

func assertPlannerCreatedState(
	t *testing.T,
	env *planningReconcileEnv,
	taskID string,
	wantState models.WorkflowState,
	wantToday bool,
	wantNext bool,
) {
	t.Helper()
	if err := storage.LoadProject(env.project); err != nil {
		t.Fatalf("reload created project: %v", err)
	}
	task := env.project.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("created task %q missing", taskID)
	}
	if got := task.State(); got != wantState {
		t.Fatalf("created task state = %s, want %s", got, wantState)
	}
	planning := env.reloadPlanning(t)
	if got := dailyPlanContains(planning, taskID); got != wantToday {
		t.Fatalf("Today membership = %v, want %v", got, wantToday)
	}
	if got := focusQueueContains(planning, taskID); got != wantNext {
		t.Fatalf("Next membership = %v, want %v", got, wantNext)
	}
	if wantToday && wantNext {
		t.Fatal("test requested invalid simultaneous Today and Next membership")
	}
}
