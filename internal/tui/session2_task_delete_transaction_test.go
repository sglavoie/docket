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
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/route"
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
	domain := newDomainController(testRuntimeAt(e.cfg.DataDir(), e.cfg), e.mgr)
	domain.service = e.svc
	domain.projects = map[string]*models.Project{e.alpha.ID: e.alpha, e.beta.ID: e.beta}

	return &AppModel{
		domain:            domain,
		secondaryFeatures: coordinator.NewSecondaryCoordinators(e.svc),
		viewNav:           viewNavigationController{views: route.NewViewRegistry(), width: 80, height: 24},
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

	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs: []TaskProjectRef{{Task: e.parent, Project: e.alpha}},
	})
	if !m.taskMutation.deleteConfirm.IsActive() {
		t.Fatal("confirm dialog should be active after request")
	}
	if got := m.taskMutation.deleteConfirm.Prompt(); got != "Delete task #1?" {
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
	m2.handleTaskDeleteRequested(TaskDeleteRequestedMsg{Refs: []TaskProjectRef{{Task: nil, Project: nil}}})
	if m2.taskMutation.deleteConfirm.IsActive() {
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
		if m.taskMutation.deleteConfirm.IsActive() || len(m.taskMutation.pendingDelete.roots) != 0 {
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
	if !m.taskMutation.deleteConfirm.IsActive() {
		t.Fatal("prompt should remain active after an unrelated key")
	}
	if reloaded := e.reload(t, e.alpha); reloaded.GetTaskByID(e.parent.ID) == nil {
		t.Error("swallowed key must not mutate task data")
	}
}
