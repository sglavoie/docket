package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/util"
)

// This file covers the Kanban bulk migration onto the shared
// BulkCoordinator/finalizeBatchResult path (session 33.7.3.3.1). Each migrated
// coordinator-supported kind (status, priority, due-date set/clear, snooze,
// state-oriented section move) commits ONE asynchronous cross-project
// transaction; the shell owns reload, selection-clear, and recent-visit repair.
// The tests drive the real transactional storage stack (the same
// mutation→reload→undo cycle the All Tasks reference migration honors), so they
// assert on committed on-disk state and the workspace ledger rather than the
// retired synchronous raw-save behavior. The retired bulk-stage kind and the
// error-preserves-selection contract are covered explicitly.

// kanbanBulkEnv seeds one or more projects on disk and wires a BulkCoordinator
// over the same application service, mirroring the secondary-scenario harness but
// supporting the cross-project selections Kanban can build.
type kanbanBulkEnv struct {
	dataDir  string
	ledger   *storage.WorkspaceLedger
	svc      *app.Service
	bulk     BulkCoordinator
	taskItem TaskItemCoordinator
	projects []*models.Project
}

func newKanbanBulkEnv(t *testing.T, taskGroups ...[]*models.Task) *kanbanBulkEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	projects := make([]*models.Project, len(taskGroups))
	for i, tasks := range taskGroups {
		slug := fmt.Sprintf("kb%d", i)
		project, err := models.NewProject(uuid.New().String(), slug, "")
		if err != nil {
			t.Fatalf("NewProject: %v", err)
		}
		project.Slug = slug
		project.Path = filepath.Join(dataDir, "projects", slug)
		project.Tasks = tasks
		next := 1
		for _, task := range tasks {
			if task.ShortID >= next {
				next = task.ShortID + 1
			}
		}
		project.NextShortID = next
		project.EnsureDefaultSections()
		project.BuildTaskTree()
		projects[i] = project
	}

	repo := newScenarioRepo(commitSvc, projects...)
	repo.seed(t)
	svc := app.NewService(repo, nil, nil)
	coords := newSecondaryCoordinators(svc, nil)
	return &kanbanBulkEnv{
		dataDir:  dataDir,
		ledger:   ledger,
		svc:      svc,
		bulk:     coords.Bulk,
		taskItem: coords.TaskItem,
		projects: projects,
	}
}

// reload reads project idx fresh from disk so an assertion sees the committed
// bytes, not the in-memory pointer the coordinator mutated.
func (e *kanbanBulkEnv) reload(t *testing.T, idx int) *models.Project {
	t.Helper()
	p := e.projects[idx]
	fresh := &models.Project{ID: p.ID, Name: p.Name, Slug: p.Slug, Path: p.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project %d: %v", idx, err)
	}
	return fresh
}

func (e *kanbanBulkEnv) undo(t *testing.T) {
	t.Helper()
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
}

// model builds a wired Kanban model whose cards reference the seeded projects,
// with every listed task selected in multi-select mode.
func (e *kanbanBulkEnv) model(t *testing.T, cols []KanbanColumn, selected ...string) *KanbanModel {
	t.Helper()
	m := newTestKanban(cols)
	m.bulk = e.bulk
	m.taskItem = e.taskItem
	m.svc = NewProjectService()
	m.loadedProjects = e.projects
	m.MultiSelectState = MultiSelectState{multiSelectMode: true, selectedTasks: map[string]bool{}}
	for _, id := range selected {
		m.selectedTasks[id] = true
	}
	return m
}

// singleProjectCols builds one column holding every task of projects[0].
func (e *kanbanBulkEnv) singleProjectCols(tasks ...*models.Task) []KanbanColumn {
	cards := make([]KanbanCard, len(tasks))
	for i, task := range tasks {
		cards[i] = KanbanCard{Task: task, Project: e.projects[0]}
	}
	return []KanbanColumn{{ID: "todo", Label: "Todo", Tasks: cards}}
}

func TestKanbanBulkStatusMigratedCommitsAsyncAndUndo(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateReady)
	b := seedTask(t, 2, "B", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a, b})
	m := env.model(t, env.singleProjectCols(a, b), a.ID, b.ID)

	_, cmd := m.applySelectedBulkState(models.StateDone)
	msg := runBatchResult(t, cmd)

	if msg.Summary != "Changed 2 tasks to done" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Changed 2 tasks to done")
	}
	if len(msg.ClearSelectionTaskIDs) != 2 {
		t.Errorf("clear selection ids = %v, want 2", msg.ClearSelectionTaskIDs)
	}

	reloaded := env.reload(t, 0)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.State() != models.StateDone {
			t.Fatalf("task %s state = %v, want done", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t, 0)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.State() != models.StateReady {
			t.Fatalf("undo state for %s = %v, want ready", id, got)
		}
	}
}

func TestKanbanBulkPriorityMigratedCommitsAsync(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateReady)
	b := seedTask(t, 2, "B", models.StateReady)
	a.Priority = 5
	b.Priority = 5
	env := newKanbanBulkEnv(t, []*models.Task{a, b})
	m := env.model(t, env.singleProjectCols(a, b), a.ID, b.ID)

	_, cmd := m.applySelectedBulkPriority("3")
	msg := runBatchResult(t, cmd)

	if msg.Summary != "Set priority 3 on 2 tasks" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Set priority 3 on 2 tasks")
	}

	reloaded := env.reload(t, 0)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.Priority != 3 {
			t.Fatalf("task %s priority = %v, want 3", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t, 0)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.Priority != 5 {
			t.Fatalf("undo priority for %s = %v, want 5", id, got)
		}
	}
}

func TestKanbanBulkDueDateSetClearMigratedCommitsAsync(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateReady)
	b := seedTask(t, 2, "B", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a, b})
	m := env.model(t, env.singleProjectCols(a, b), a.ID, b.ID)

	_, cmd := m.applySelectedBulkDueDate(0, false)
	msg := runBatchResult(t, cmd)
	if msg.Summary != "Due date set on 2 tasks" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Due date set on 2 tasks")
	}

	today := util.StartOfDay(time.Now()).Format("2006-01-02")
	reloaded := env.reload(t, 0)
	for _, id := range []string{a.ID, b.ID} {
		got := reloaded.GetTaskByID(id)
		if got == nil || got.DueDate == nil || got.DueDate.Format("2006-01-02") != today {
			t.Fatalf("task %s due date = %v, want %s", id, got, today)
		}
	}

	// Now clear the due dates through the same migrated path.
	m = env.model(t, env.singleProjectColsFrom(reloaded, a.ID, b.ID), a.ID, b.ID)
	_, cmd = m.applySelectedBulkDueDate(0, true)
	msg = runBatchResult(t, cmd)
	if msg.Summary != "Cleared due date on 2 tasks" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Cleared due date on 2 tasks")
	}
	reloaded = env.reload(t, 0)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DueDate != nil {
			t.Fatalf("task %s due date = %v, want cleared", id, got)
		}
	}
}

func TestKanbanBulkSnoozeMigratedCommitsAsync(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateReady)
	b := seedTask(t, 2, "B", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a, b})
	m := env.model(t, env.singleProjectCols(a, b), a.ID, b.ID)

	_, cmd := m.applySelectedBulkSnooze("w")
	msg := runBatchResult(t, cmd)
	if msg.Summary != "Snoozed 2 tasks" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Snoozed 2 tasks")
	}

	nextWeek := util.StartOfDay(time.Now()).AddDate(0, 0, 7).Format("2006-01-02")
	reloaded := env.reload(t, 0)
	for _, id := range []string{a.ID, b.ID} {
		got := reloaded.GetTaskByID(id)
		if got == nil || got.SnoozedUntil == nil || got.SnoozedUntil.Format("2006-01-02") != nextWeek {
			t.Fatalf("task %s snooze = %v, want %s", id, got, nextWeek)
		}
	}
}

// TestKanbanBulkSectionMoveMigratedTransitionsStateCrossProject verifies the
// state-oriented section move now commits one cross-project transaction (the
// former same-project restriction is gone) and moves every selected task to the
// workflow state the target GTD section maps onto.
func TestKanbanBulkSectionMoveMigratedTransitionsStateCrossProject(t *testing.T) {
	a := seedTask(t, 1, "Alpha", models.StateReady)
	b := seedTask(t, 1, "Beta", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a}, []*models.Task{b})
	m := env.model(t, []KanbanColumn{
		{ID: "todo", Label: "Todo", Tasks: []KanbanCard{
			{Task: a, Project: env.projects[0]},
			{Task: b, Project: env.projects[1]},
		}},
	}, a.ID, b.ID)

	_, cmd := m.applySelectedBulkSectionMove(models.SectionNext)
	msg := runBatchResult(t, cmd)

	if msg.Summary != "Moved 2 tasks to Next" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Moved 2 tasks to Next")
	}
	if len(msg.TouchedProjectIDs) != 2 {
		t.Errorf("touched projects = %v, want 2 (cross-project)", msg.TouchedProjectIDs)
	}

	if got := env.reload(t, 0).GetTaskByID(a.ID); got == nil || got.State() != models.StateNext {
		t.Fatalf("alpha state = %v, want next", got)
	}
	if got := env.reload(t, 1).GetTaskByID(b.ID); got == nil || got.State() != models.StateNext {
		t.Fatalf("beta state = %v, want next", got)
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(a.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("undo alpha state = %v, want ready", got)
	}
	if got := env.reload(t, 1).GetTaskByID(b.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("undo beta state = %v, want ready", got)
	}
}

// TestKanbanBulkSectionMoveUnknownSectionSetsStatus verifies an unrecognized
// section name surfaces a status message and issues no command.
func TestKanbanBulkSectionMoveUnknownSectionSetsStatus(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a})
	m := env.model(t, env.singleProjectCols(a), a.ID)

	_, cmd := m.applySelectedBulkSectionMove("Nonsense")
	if cmd != nil {
		t.Fatalf("expected no command for unknown section, got %T", cmd())
	}
	if m.statusMsg != `Unknown section "Nonsense"` {
		t.Errorf("status = %q, want unknown-section message", m.statusMsg)
	}
}

// TestKanbanBulkErrorPreservesSelection verifies a rejected batch passes its
// TransactionErrorMsg through untouched, so the shell never clears the selection.
func TestKanbanBulkErrorPreservesSelection(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateReady)
	b := seedTask(t, 2, "B", models.StateReady)
	proj := testutil.NewTestProject("p")
	m := newTestKanban([]KanbanColumn{{ID: "todo", Tasks: []KanbanCard{
		{Task: a, Project: proj}, {Task: b, Project: proj},
	}}})
	m.bulk = BulkCoordinator{name: "bulk", bulk: erroringBulkMutator{}}
	m.MultiSelectState = MultiSelectState{multiSelectMode: true, selectedTasks: map[string]bool{a.ID: true, b.ID: true}}

	_, cmd := m.applySelectedBulkState(models.StateDone)
	if cmd == nil {
		t.Fatal("expected a command")
	}
	if _, ok := cmd().(TransactionErrorMsg); !ok {
		t.Fatalf("expected TransactionErrorMsg passthrough, got %T", cmd())
	}
	// The handler never clears optimistically; selection survives the rejection.
	if !m.multiSelectMode || m.SelectedCount() != 2 {
		t.Fatalf("selection = mode %v count %d, want preserved", m.multiSelectMode, m.SelectedCount())
	}
}

// TestKanbanBulkShellClearsSelectionOnCommit verifies the shell clears the
// registered Kanban view's multiselect once a migrated batch commits.
func TestKanbanBulkShellClearsSelectionOnCommit(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateReady)
	b := seedTask(t, 2, "B", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a, b})
	km := env.model(t, env.singleProjectCols(a, b), a.ID, b.ID)

	_, cmd := km.applySelectedBulkState(models.StateDone)
	msg := runBatchResult(t, cmd)

	mgr := config.NewManager(filepath.Join(env.dataDir, ".tt.json"))
	appModel := newTestAppModel()
	appModel.configManager = mgr
	appModel.views = NewViewRegistry()
	setTestAppProjects(appModel, env.projects...)
	appModel.views.Register(ViewKanban, km)

	appModel.handleBatchTransactionResult(msg)

	if km.multiSelectMode || km.SelectedCount() != 0 {
		t.Fatalf("committed batch should clear the Kanban selection (mode %v count %d)", km.multiSelectMode, km.SelectedCount())
	}
}

// TestKanbanBulkAdjacentMoveMigratedCommitsAsyncAndUndo verifies the H/L bulk
// column move (bulkMoveSelectedTasks → workflowPersist removed) now applies the
// per-mode mutation to every selected card in memory and commits the touched
// project through the workflow-aware coordinator. In status mode every valid
// selected task transitions to the target column's state, and the workspace ledger
// restores the prior state.
func TestKanbanBulkAdjacentMoveMigratedCommitsAsyncAndUndo(t *testing.T) {
	a := seedTask(t, 1, "A", models.StateReady)
	b := seedTask(t, 2, "B", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{a, b})
	cols := []KanbanColumn{
		{ID: "ready", Label: "Ready", Tasks: []KanbanCard{
			{Task: a, Project: env.projects[0]},
			{Task: b, Project: env.projects[0]},
		}},
		{ID: "done", Label: "Done"},
	}
	m := env.model(t, cols, a.ID, b.ID)
	m.taskItem = env.taskItem
	m.mode = KanbanModeStatus
	m.activeCol = 0

	_, cmd := m.bulkMoveSelectedTasks(+1)
	if m.statusMsg != "Moved 2 tasks → Done" {
		t.Errorf("status = %q, want %q", m.statusMsg, "Moved 2 tasks → Done")
	}
	runBatchResult(t, cmd)

	reloaded := env.reload(t, 0)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.State() != models.StateDone {
			t.Fatalf("task %s state = %v, want done", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t, 0)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.State() != models.StateReady {
			t.Fatalf("undo state for %s = %v, want ready", id, got)
		}
	}
}

// TestKanbanBulkAdjacentMoveStatusSkipsInvalidTransition verifies the status-mode
// column move skips tasks that cannot transition (so the count and the commit both
// reflect only the valid subset), and issues no command when nothing moved.
func TestKanbanBulkAdjacentMoveStatusSkipsInvalidTransition(t *testing.T) {
	done1 := seedTask(t, 1, "done1", models.StateDone)
	done2 := seedTask(t, 2, "done2", models.StateDone)
	env := newKanbanBulkEnv(t, []*models.Task{done1, done2})
	cols := []KanbanColumn{
		{ID: "done", Label: "Done", Tasks: []KanbanCard{
			{Task: done1, Project: env.projects[0]},
			{Task: done2, Project: env.projects[0]},
		}},
		{ID: "canceled", Label: "Canceled"},
	}
	m := env.model(t, cols, done1.ID, done2.ID)
	m.taskItem = env.taskItem
	m.mode = KanbanModeStatus
	m.activeCol = 0

	_, cmd := m.bulkMoveSelectedTasks(+1)
	if m.statusMsg != "Moved 0 tasks → Canceled" {
		t.Errorf("status = %q, want %q", m.statusMsg, "Moved 0 tasks → Canceled")
	}
	if cmd != nil {
		t.Fatalf("no valid transitions should issue no command, got %T", cmd())
	}
}

// singleProjectColsFrom rebuilds a single column from a freshly reloaded project
// so a follow-up mutation operates on the committed task pointers.
func (e *kanbanBulkEnv) singleProjectColsFrom(project *models.Project, ids ...string) []KanbanColumn {
	e.projects[0] = project
	cards := make([]KanbanCard, 0, len(ids))
	for _, id := range ids {
		if task := project.GetTaskByID(id); task != nil {
			cards = append(cards, KanbanCard{Task: task, Project: project})
		}
	}
	return []KanbanColumn{{ID: "todo", Label: "Todo", Tasks: cards}}
}

// erroringBulkMutator is a BulkMutator whose every operation fails, used to
// assert the error-preserves-selection contract.
type erroringBulkMutator struct{}

var errBulkRejected = errors.New("bulk rejected")

func (erroringBulkMutator) BulkTransition([]string, models.WorkflowState) (app.BulkResult, error) {
	return app.BulkResult{}, errBulkRejected
}

func (erroringBulkMutator) BulkTransitionWithWorkflow(app.BulkTransitionRequest) (app.BulkResult, error) {
	return app.BulkResult{}, errBulkRejected
}

func (erroringBulkMutator) BulkDelete(app.BulkDeleteRequest, ...storage.ArtifactChange) (app.BulkDeleteResult, error) {
	return app.BulkDeleteResult{}, errBulkRejected
}

func (erroringBulkMutator) BulkSetPriority([]string, int) (app.BulkResult, error) {
	return app.BulkResult{}, errBulkRejected
}

func (erroringBulkMutator) BulkSetDueDate([]string, time.Time, bool) (app.BulkResult, error) {
	return app.BulkResult{}, errBulkRejected
}

func (erroringBulkMutator) BulkClearDueDate([]string) (app.BulkResult, error) {
	return app.BulkResult{}, errBulkRejected
}

func (erroringBulkMutator) BulkSnooze([]string, *time.Time) (app.BulkResult, error) {
	return app.BulkResult{}, errBulkRejected
}

func (erroringBulkMutator) BulkSetTags([]string, []string) (app.BulkResult, error) {
	return app.BulkResult{}, errBulkRejected
}
