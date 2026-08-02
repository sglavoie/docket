package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// taskDeleteEnv wires the canonical shell-owned deletion transaction over the
// real transactional stack (two projects on disk, a workspace ledger, and a
// temp-dir config manager) so a test reads as: arm request -> confirm -> assert
// disk/planning/selection/undo.
type taskDeleteEnv struct {
	dataDir string
	ledger  *storage.WorkspaceLedger
	svc     *app.Service
	mgr     *config.Manager
	cfg     *config.RuntimeState
	alpha   *models.Project
	beta    *models.Project
	// alpha task roots.
	parent  *models.Task // #1, root of a subtree
	child   *models.Task // #2, descendant of parent
	sibling *models.Task // #3, standalone survivor
	lone    *models.Task // beta #10
}

func newTaskDeleteEnv(t *testing.T) *taskDeleteEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	newProject := func(slug, name string, tasks []*models.Task) *models.Project {
		p, err := models.NewProject(uuid.New().String(), slug, "")
		if err != nil {
			t.Fatalf("NewProject: %v", err)
		}
		p.Slug = slug
		p.Name = name
		p.Path = filepath.Join(dataDir, "projects", slug)
		p.Tasks = tasks
		next := 1
		for _, tk := range tasks {
			if tk.ShortID >= next {
				next = tk.ShortID + 1
			}
		}
		p.NextShortID = next
		p.BuildTaskTree()
		return p
	}

	// The parent is an epic so a task may legally nest beneath it (a task cannot
	// be the child of another task per the type hierarchy).
	parent, err := models.NewGenericTask(uuid.New().String(), 1, "Parent")
	if err != nil {
		t.Fatalf("NewTask epic: %v", err)
	}
	parent.ApplyWorkflowState(models.StateBacklog)
	child := seedTask(t, 2, "Child", models.StateBacklog)
	child.ParentID = &parent.ID
	sibling := seedTask(t, 3, "Sibling", models.StateBacklog)
	lone := seedTask(t, 10, "Lone", models.StateBacklog)

	alpha := newProject("alpha", "Alpha", []*models.Task{parent, child, sibling})
	beta := newProject("beta", "Beta", []*models.Task{lone})

	repo := newScenarioRepo(commitSvc, alpha, beta)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	mgr := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{alpha, beta})
	// A durable planning ref inside the deleted subtree must be repaired on delete.
	cfg.Planning().SetNext([]config.ProjectTaskLocator{{ProjectID: alpha.ID, TaskID: child.ID}})
	if err := saveTestRuntime(mgr, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	return &taskDeleteEnv{
		dataDir: dataDir, ledger: ledger, svc: svc, mgr: mgr, cfg: cfg,
		alpha: alpha, beta: beta,
		parent: parent, child: child, sibling: sibling, lone: lone,
	}
}

func (e *taskDeleteEnv) appModel() *AppModel {
	return &AppModel{
		configManager:        e.mgr,
		runtime:              testRuntimeAt(e.cfg.DataDir(), e.cfg),
		projects:             map[string]*models.Project{e.alpha.ID: e.alpha, e.beta.ID: e.beta},
		persister:            NewProjectService(),
		appService:           e.svc,
		secondaryFeatures:    newSecondaryCoordinators(e.svc),
		views:                NewViewRegistry(),
		width:                80,
		height:               24,
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
	}
}

func (e *taskDeleteEnv) reload(t *testing.T, p *models.Project) *models.Project {
	t.Helper()
	fresh := &models.Project{ID: p.ID, Name: p.Name, Slug: p.Slug, Path: p.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload %s: %v", p.Slug, err)
	}
	return fresh
}

func keyPress(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		return tea.KeyPressMsg{Code: rune(s[0])}
	}
}

// --- canonicalization + prompt grammar -------------------------------------

func TestCanonicalizeTaskDeleteRootsDedupesAndDropsInvalid(t *testing.T) {
	p := &models.Project{ID: "p"}
	a := &models.Task{ID: "a", ShortID: 1}
	b := &models.Task{ID: "b", ShortID: 2}

	refs := []TaskProjectRef{
		{Task: a, Project: p},
		{Task: nil, Project: p},            // dropped: nil task
		{Task: b, Project: nil},            // dropped: nil project
		{Task: &models.Task{}, Project: p}, // dropped: blank ID
		{Task: a, Project: p},              // dropped: duplicate root
		{Task: b, Project: p},
	}
	roots := canonicalizeTaskDeleteRoots(refs)
	if len(roots) != 2 {
		t.Fatalf("roots = %d, want 2 (a,b)", len(roots))
	}
	if roots[0].Task.ID != "a" || roots[1].Task.ID != "b" {
		t.Errorf("first-seen order not preserved: %q,%q", roots[0].Task.ID, roots[1].Task.ID)
	}
}

func TestTaskDeletePromptExactWording(t *testing.T) {
	p := &models.Project{ID: "p"}
	one := []TaskProjectRef{{Task: &models.Task{ID: "a", ShortID: 7}, Project: p}}
	if got := taskDeletePrompt(one); got != "Delete task #7?" {
		t.Errorf("single-root prompt = %q, want %q", got, "Delete task #7?")
	}
	many := []TaskProjectRef{
		{Task: &models.Task{ID: "a", ShortID: 7}, Project: p},
		{Task: &models.Task{ID: "b", ShortID: 8}, Project: p},
	}
	if got := taskDeletePrompt(many); got != "Delete 2 tasks?" {
		t.Errorf("multi-root prompt = %q, want %q", got, "Delete 2 tasks?")
	}
}

func TestHandleTaskDeleteRequestedArmsConfirmWithoutMutating(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()

	cmd := m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs:   []TaskProjectRef{{Task: e.parent, Project: e.alpha}},
		Origin: ViewCrossProjectOutliner,
	})
	if cmd != nil {
		t.Fatal("arming the confirm must not dispatch a command")
	}
	if !m.taskDeleteConfirm.IsActive() {
		t.Fatal("confirm dialog should be active after request")
	}
	if m.pendingTaskDelete.origin != ViewCrossProjectOutliner {
		t.Errorf("origin = %v, want Outliner", m.pendingTaskDelete.origin)
	}
	if got := m.taskDeleteConfirm.Prompt(); got != "Delete task #1?" {
		t.Errorf("prompt = %q, want %q", got, "Delete task #1?")
	}
	if m.renderConfirmDialogOverlay() == "" {
		t.Error("armed confirm should render an overlay")
	}
	// No mutation before y: the subtree is still on disk.
	if reloaded := e.reload(t, e.alpha); reloaded.GetTaskByID(e.parent.ID) == nil {
		t.Error("no task should be deleted before confirmation")
	}

	// An empty/invalid request is a no-op that leaves nothing armed.
	m2 := e.appModel()
	if cmd := m2.handleTaskDeleteRequested(TaskDeleteRequestedMsg{Refs: []TaskProjectRef{{Task: nil, Project: nil}}}); cmd != nil {
		t.Error("empty request should return nil")
	}
	if m2.taskDeleteConfirm.IsActive() {
		t.Error("empty request must not arm the confirm")
	}
}

// --- cancellation preserves everything -------------------------------------

func TestTaskDeleteCancelViaNAndEscKeepsData(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		e := newTaskDeleteEnv(t)
		m := e.appModel()
		m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
			Refs: []TaskProjectRef{{Task: e.parent, Project: e.alpha}},
		})

		handled, cmd := m.handleGlobalKeyPress(keyPress(key))
		if !handled {
			t.Fatalf("%q should be handled by the modal confirm", key)
		}
		if cmd != nil {
			t.Errorf("%q cancel should not dispatch a delete command", key)
		}
		if m.taskDeleteConfirm.IsActive() || len(m.pendingTaskDelete.roots) != 0 {
			t.Errorf("%q should clear the pending deletion", key)
		}
		if reloaded := e.reload(t, e.alpha); reloaded.GetTaskByID(e.parent.ID) == nil {
			t.Errorf("%q cancel must preserve the task on disk", key)
		}
	}
}

func TestTaskDeleteConfirmSwallowsOtherKeys(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()
	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{{Task: e.parent, Project: e.alpha}},
	})
	// An unrelated key is consumed (modal priority) and leaves the prompt up.
	handled, _ := m.handleGlobalKeyPress(keyPress("j"))
	if !handled {
		t.Fatal("modal confirm must swallow unrelated keys")
	}
	if !m.taskDeleteConfirm.IsActive() {
		t.Fatal("prompt should remain active after an unrelated key")
	}
	if reloaded := e.reload(t, e.alpha); reloaded.GetTaskByID(e.parent.ID) == nil {
		t.Error("swallowed key must not mutate task data")
	}
}

// --- single-root subtree delete: full contract -----------------------------

func TestTaskDeleteSingleRootCommitsSubtreePlanningAndUndo(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()
	runtime := m.runtime

	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs:   []TaskProjectRef{{Task: e.parent, Project: e.alpha}},
		Origin: ViewCrossProjectOutliner,
	})

	handled, cmd := m.handleGlobalKeyPress(keyPress("y"))
	if !handled || cmd == nil {
		t.Fatal("y should confirm and dispatch the delete transaction")
	}
	if m.taskDeleteConfirm.IsActive() {
		t.Error("confirm should be dismissed once the transaction is dispatched")
	}

	// Building the checked artifact must not mutate the cached snapshot before commit.
	if len(runtime.Planning().GetNextOrder()) != 1 {
		t.Errorf("cached focus queue changed before commit: %v", runtime.Planning().GetNextOrder())
	}

	msg := runBatchResult(t, cmd)
	// Root + descendant removed; the requested root is the selection to clear.
	if len(msg.RemovedTaskIDs) != 2 {
		t.Errorf("removed ids = %v, want parent+child", msg.RemovedTaskIDs)
	}
	if len(msg.ClearSelectionTaskIDs) != 1 || msg.ClearSelectionTaskIDs[0] != e.parent.ID {
		t.Errorf("clear-selection ids = %v, want [parent]", msg.ClearSelectionTaskIDs)
	}

	m.handleBatchTransactionResult(msg)

	// Subtree gone, sibling survives on disk.
	reloaded := e.reload(t, e.alpha)
	if reloaded.GetTaskByID(e.parent.ID) != nil || reloaded.GetTaskByID(e.child.ID) != nil {
		t.Error("parent and child should be gone from disk")
	}
	if reloaded.GetTaskByID(e.sibling.ID) == nil {
		t.Error("sibling should survive the subtree delete")
	}

	// Durable planning cleanup committed with the delete.
	if len(runtime.Planning().GetNextOrder()) != 0 {
		t.Errorf("runtime focus queue should be cleaned, got %v", runtime.Planning().GetNextOrder())
	}
	reloadedCfg, err := config.NewManager(filepath.Join(e.dataDir, ".tt.json")).LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloadedCfg.Planning().NextOrder) != 0 {
		t.Errorf("durable focus queue should be cleaned on disk, got %v", reloadedCfg.Planning().NextOrder)
	}

	// Undo restores the deleted subtree from the workspace ledger.
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	restored := e.reload(t, e.alpha)
	if restored.GetTaskByID(e.parent.ID) == nil || restored.GetTaskByID(e.child.ID) == nil {
		t.Error("undo should restore the deleted subtree")
	}
}

// --- nested roots: duplicate subtree collapses, sibling survives -----------

func TestTaskDeleteNestedRootsPromptAndRemoval(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()

	// Select both the parent and its own child as roots.
	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{
			{Task: e.parent, Project: e.alpha},
			{Task: e.child, Project: e.alpha},
		},
	})
	if got := m.taskDeleteConfirm.Prompt(); got != "Delete 2 tasks?" {
		t.Errorf("nested-root prompt = %q, want %q", got, "Delete 2 tasks?")
	}

	_, cmd := m.handleGlobalKeyPress(keyPress("y"))
	msg := runBatchResult(t, cmd)
	// The application service collapses the nested root; the subtree removes once.
	if len(msg.RemovedTaskIDs) != 2 {
		t.Errorf("removed ids = %v, want parent+child once", msg.RemovedTaskIDs)
	}
	m.handleBatchTransactionResult(msg)

	reloaded := e.reload(t, e.alpha)
	if reloaded.GetTaskByID(e.parent.ID) != nil || reloaded.GetTaskByID(e.child.ID) != nil {
		t.Error("nested roots should delete the whole subtree")
	}
	if reloaded.GetTaskByID(e.sibling.ID) == nil {
		t.Error("sibling should survive nested-root delete")
	}
}

// --- cross-project selection commits atomically ----------------------------

func TestTaskDeleteCrossProjectSelection(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()

	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{
			{Task: e.sibling, Project: e.alpha},
			{Task: e.lone, Project: e.beta},
		},
	})
	if got := m.taskDeleteConfirm.Prompt(); got != "Delete 2 tasks?" {
		t.Errorf("prompt = %q, want %q", got, "Delete 2 tasks?")
	}

	_, cmd := m.handleGlobalKeyPress(keyPress("y"))
	msg := runBatchResult(t, cmd)
	if len(msg.TouchedProjectIDs) != 2 {
		t.Errorf("touched projects = %v, want alpha+beta", msg.TouchedProjectIDs)
	}
	if len(msg.RemovedTaskIDs) != 2 {
		t.Errorf("removed ids = %v, want sibling+lone", msg.RemovedTaskIDs)
	}
	m.handleBatchTransactionResult(msg)

	if e.reload(t, e.alpha).GetTaskByID(e.sibling.ID) != nil {
		t.Error("alpha sibling should be gone")
	}
	if e.reload(t, e.beta).GetTaskByID(e.lone.ID) != nil {
		t.Error("beta lone should be gone")
	}
	// The parent subtree in alpha is untouched by this selection.
	if e.reload(t, e.alpha).GetTaskByID(e.parent.ID) == nil {
		t.Error("unselected parent should survive")
	}
}

func TestTUITaskDeletePreservesDuplicateIDReferencesInOtherProject(t *testing.T) {
	e := newTaskDeleteEnv(t)
	e.lone.ID = e.parent.ID
	if err := storage.SaveProject(e.beta); err != nil {
		t.Fatalf("save beta duplicate: %v", err)
	}
	e.cfg.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: e.alpha.ID, TaskID: e.parent.ID},
		{ProjectID: e.beta.ID, TaskID: e.lone.ID},
	})
	e.cfg.Planning().Today = []config.ProjectTaskLocator{
		{ProjectID: e.alpha.ID, TaskID: e.parent.ID},
		{ProjectID: e.beta.ID, TaskID: e.lone.ID},
	}
	if err := saveTestRuntime(e.mgr, e.cfg); err != nil {
		t.Fatalf("save duplicate refs: %v", err)
	}

	m := e.appModel()
	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{{Task: e.lone, Project: e.beta}},
	})
	_, cmd := m.handleGlobalKeyPress(keyPress("y"))
	msg := runBatchResult(t, cmd)
	m.handleBatchTransactionResult(msg)

	if e.reload(t, e.alpha).GetTaskByID(e.parent.ID) == nil {
		t.Fatal("alpha/shared was deleted")
	}
	if e.reload(t, e.beta).GetTaskByID(e.lone.ID) != nil {
		t.Fatal("beta/shared survived delete")
	}
	if got := m.runtime.Planning().NextOrder; len(got) != 1 || got[0].ProjectID != e.alpha.ID {
		t.Fatalf("delete cleaned wrong focus refs: %+v", got)
	}
	if got := m.runtime.Planning().Today; len(got) != 1 || got[0].ProjectID != e.alpha.ID {
		t.Fatalf("delete cleaned wrong daily refs: %+v", got)
	}
}

// --- failed delete preserves state and emits no success --------------------

func TestTaskDeleteFailurePreservesStateNoSuccess(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()

	// A root whose task is not on disk: the application service rejects the batch
	// with ErrTaskNotFound, so the transaction never commits.
	ghost := seedTask(t, 99, "Ghost", models.StateBacklog)
	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{{Task: ghost, Project: e.alpha}},
	})

	_, cmd := m.handleGlobalKeyPress(keyPress("y"))
	if cmd == nil {
		t.Fatal("confirm should dispatch a command even for a doomed delete")
	}
	out := cmd()
	if _, ok := out.(TransactionErrorMsg); !ok {
		t.Fatalf("failed delete should yield TransactionErrorMsg, got %T", out)
	}
	// A rejected batch never enriches ClearSelectionTaskIDs, so the shell keeps the
	// selection intact for a retry — and the real tasks are untouched on disk.
	if reloaded := e.reload(t, e.alpha); reloaded.GetTaskByID(e.parent.ID) == nil || reloaded.GetTaskByID(e.sibling.ID) == nil {
		t.Error("a failed delete must leave existing tasks on disk")
	}

	// Routed through Update, the error sets the error banner, not a success status.
	m.err = nil
	if handled, _ := m.handleGlobalSaveMessage(out); !handled {
		t.Fatal("TransactionErrorMsg should be handled by the save-message path")
	}
	if m.err == nil {
		t.Error("failure should surface an error, not a success state")
	}
}
