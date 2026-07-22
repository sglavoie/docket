package tui

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/workflow"
)

// TestFocusBoardCompletionRunsWorkflowCascade proves the migrated focus-board
// completion preserves the legacy behavior of running the stateless workflow-rule
// cascade after a single-task change: a "status == done -> add tag" rule fires and
// the tag lands on disk in the same committed transaction.
func TestFocusBoardCompletionRunsWorkflowCascade(t *testing.T) {
	m, project, tasks := newFlowLoopFocusBoard(t, models.StatusTodo)
	m.runtime.Automation().WorkflowRules = []workflow.WorkflowRule{{
		ID:      "r-done-tag",
		Name:    "Tag done",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "shipped"}},
	}}
	m.cursor = 0

	_, cmd := m.markDone()
	finishFocusBoardMutation(t, m, cmd)

	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	done := fresh.GetTaskByID(tasks[0].ID)
	if done == nil {
		t.Fatal("completed task missing after reload")
	}
	if done.State() != models.StateDone {
		t.Errorf("task state = %s, want done", done.State())
	}
	if !done.HasTag("shipped") {
		t.Errorf("workflow rule did not apply tag through the migrated coordinator; tags = %v", done.Tags)
	}
}

func newFlowLoopFocusBoard(t *testing.T, statuses ...models.TaskStatus) (*FocusBoardModel, *models.Project, []*models.Task) {
	t.Helper()

	cfg := newTestConfigAt(t.TempDir())
	project := testutil.NewTestProject("Flow Loop")
	project.Path = cfg.ProjectPath(project.Slug)
	setTestProjects(cfg, []*models.Project{project})
	cfg.Machine().CurrentProject = project.ID

	tasks := make([]*models.Task, 0, len(statuses))
	for i, status := range statuses {
		task := cockpitTask(t, project, fmt.Sprintf("Flow task %d", i+1), status, models.SectionNext)
		tasks = append(tasks, task)
	}

	today := time.Now().Format("2006-01-02")
	dailyPlan := make([]FocusTask, 0, len(tasks))
	dailyPlanIDs := make(map[TaskIdentity]bool, len(tasks))
	planItems := make([]config.DailyPlanItem, 0, len(tasks))
	for _, task := range tasks {
		dailyPlan = append(dailyPlan, FocusTask{Task: task, Project: project})
		dailyPlanIDs[taskIdentityFor(task, project)] = true
		planItems = append(planItems, config.DailyPlanItem{
			TaskID:      task.ID,
			ProjectID:   project.ID,
			PlannedDate: today,
		})
	}
	cfg.Planning().SetDailyPlan(planItems)

	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	focusQueue := []FocusTask{}
	m := NewFocusBoardModel(testRuntimeAt(cfg.DataDir(), cfg), nil, &dailyPlan, &focusQueue, map[TaskIdentity]bool{}, dailyPlanIDs, map[TaskIdentity]bool{})
	m.tabs.Active = boardTabToday

	// Wire the single-task coordinator over the real transactional stack rooted at
	// the fixture's data dir, so the migrated focus-board mutations commit through
	// the application layer / workspace ledger. Mutations write to disk; the board
	// reloads from disk via refreshData(), simulating the shell's post-commit
	// refresh (finishFocusBoardMutation drives that round-trip in the tests).
	ledger := storage.NewWorkspaceLedger(filepath.Join(cfg.DataDir(), ".tt", "undo"), testWorkspaceLedgerDepth, []string{cfg.DataDir()})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(cfg.DataDir(), ".tt", "transactions"), []string{cfg.DataDir()}, ledger)
	repo := newScenarioRepo(commitSvc, project)
	svc := app.NewService(repo, nil, nil)
	m.SetTaskItemCoordinator(newSecondaryCoordinators(svc, nil).TaskItem)

	m.applyFilter()
	return m, project, tasks
}

// finishFocusBoardMutation runs a migrated focus-board single-task command to
// commit it, then reloads the board from disk exactly as the shell does after a
// BatchTransactionResultMsg, so a test can assert the post-commit board state. It
// fails the test if the command rejected the mutation.
func finishFocusBoardMutation(t *testing.T, m *FocusBoardModel, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("focus-board mutation returned nil command, want a commit command")
	}
	for _, msg := range drainFocusBoardCmd(cmd) {
		if e, ok := msg.(TransactionErrorMsg); ok {
			t.Fatalf("focus-board mutation rejected: %v", e.Err)
		}
	}
	m.refreshData()
}

// drainFocusBoardCmd runs a command and any commands a tea.Batch fans out, so the
// coordinator's commit side effect actually executes (markDone/dismiss/unblock
// wrap the commit command with a flash timer via tea.Batch).
func drainFocusBoardCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			msgs = append(msgs, drainFocusBoardCmd(c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func TestFocusBoardFlowLoopCompletionAdvancesToNextActionable(t *testing.T) {
	m, _, tasks := newFlowLoopFocusBoard(t, models.StatusTodo, models.StatusTodo)
	m.cursor = 0

	_, cmd := m.markDone()
	finishFocusBoardMutation(t, m, cmd)

	if got := m.filtered[m.cursor].Task.ID; got != tasks[1].ID {
		t.Fatalf("cursor selected %q, want next actionable task %q", got, tasks[1].ID)
	}
	rec := m.computeNextActionRecommendation()
	if rec.Kind != RecommendationKindWorkOnTask || rec.Task == nil || rec.Task.ID != tasks[1].ID {
		t.Fatalf("recommendation after completion = %s/%v, want WorkOnTask for next task", rec.Kind, rec.Task)
	}
}

func TestFocusBoardFlowLoopDismissAdvancesToNextActionable(t *testing.T) {
	m, _, tasks := newFlowLoopFocusBoard(t, models.StatusTodo, models.StatusTodo)
	m.cursor = 0

	_, cmd := m.dismiss()
	finishFocusBoardMutation(t, m, cmd)

	if got := m.filtered[m.cursor].Task.ID; got != tasks[1].ID {
		t.Fatalf("cursor selected %q, want next actionable task %q", got, tasks[1].ID)
	}
	rec := m.computeNextActionRecommendation()
	if rec.Kind != RecommendationKindWorkOnTask || rec.Task == nil || rec.Task.ID != tasks[1].ID {
		t.Fatalf("recommendation after dismiss = %s/%v, want WorkOnTask for next task", rec.Kind, rec.Task)
	}
}

func TestFocusBoardRefreshPreservesCursorByTaskIDAfterReorder(t *testing.T) {
	m, project, tasks := newFlowLoopFocusBoard(t, models.StatusTodo, models.StatusTodo)
	m.cursor = 1

	project.GetTaskByID(tasks[0].ID).ApplyTransition(models.StatusDone)
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	m.Update(FocusBoardRefreshMsg{})

	if got := m.filtered[m.cursor].Task.ID; got != tasks[1].ID {
		t.Fatalf("refresh selected %q, want preserved task %q", got, tasks[1].ID)
	}
}
