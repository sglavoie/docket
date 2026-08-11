package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// planningOrderProject seeds one on-disk project with the given StateBacklog tasks
// (in slice order) so a cross-project reorder test can interleave several
// projects' Next horizons.
func planningOrderProject(t *testing.T, dataDir, id, slug string, titles ...string) (*models.Project, []*models.Task) {
	t.Helper()
	project, err := models.NewProject(id, slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = slug
	project.Path = filepath.Join(dataDir, "projects", slug)
	tasks := make([]*models.Task, len(titles))
	for i, title := range titles {
		tasks[i] = seedTask(t, i+1, title, models.StateBacklog)
	}
	project.Tasks = tasks
	project.NextShortID = len(titles) + 1
	project.BuildTaskTree()
	return project, tasks
}

// newPlanningOrderApp builds a ledger-backed AppModel hosting the given projects
// with the cross-project Outliner installed as crossOutliner, so a J/K reorder can
// be driven through the real keymap and its durable planning commit inspected on
// disk. It returns the model, the cross Outliner, and the data dir.
func newPlanningOrderApp(t *testing.T, projects ...*models.Project) (*AppModel, *OutlinerModel, string) {
	t.Helper()
	dataDir := t.TempDir()
	for _, p := range projects {
		p.Path = filepath.Join(dataDir, "projects", p.Slug)
	}

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := &failableScenarioRepo{scenarioRepo: newScenarioRepo(commitSvc, projects...)}
	repo.seed(t)

	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, projects)
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil).
		WithPlanningArtifacts(planningReconcileArtifactBuilder(manager))

	projectMap := make(map[string]*models.Project, len(projects))
	for _, p := range projects {
		projectMap[p.ID] = p
	}

	m := &AppModel{
		configManager:     manager,
		runtime:           runtime,
		deps:              newShellDeps(ledger),
		projects:          projectMap,
		persister:         NewProjectService(),
		appService:        svc,
		secondaryFeatures: newSecondaryCoordinators(svc),
		width:             120,
		height:            40,
		views:             NewViewRegistry(),
		currentProject:    projects[0],
		nextOrder:         []PlanningTask{},
		nextOrderIDs:      make(map[TaskIdentity]bool),
		today:             []PlanningTask{},
		todayIDs:          make(map[TaskIdentity]bool),
		operationRegistry: NewOperationRegistry(),
		navigation:        NewNavigationState(),
	}
	m.operationExecutor = NewOperationExecutor(m.operationRegistry)
	m.rebuildPlanningFromRuntime()

	cross := NewOutlinerModel(nil, manager, runtime)
	cross.loadedProjects = projects
	if len(projects) == 1 {
		cross.scopedProjectID = projects[0].ID
	}
	cross.SetSize(120, 40)
	m.crossOutliner = cross
	m.views.Register(ViewCrossProjectOutliner, cross)
	if err := m.registerCoreInputOperations(); err != nil {
		t.Fatalf("register core operations: %v", err)
	}
	if err := m.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task action operations: %v", err)
	}
	if err := m.registerFilterOperations(); err != nil {
		t.Fatalf("register filter operations: %v", err)
	}
	if err := m.registerSortOperations(); err != nil {
		t.Fatalf("register sort operations: %v", err)
	}
	if err := m.registerProjectOperations(); err != nil {
		t.Fatalf("register project operations: %v", err)
	}
	if err := m.registerChecklistRowOperations(); err != nil {
		t.Fatalf("register checklist row operations: %v", err)
	}
	if err := m.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified input: %v", err)
	}
	m.wireOutlinerOperationExecutor(cross)
	return m, cross, dataDir
}

// enterCrossMode selects a mode on the cross Outliner and rebuilds its
// rows so the mode's live rows and flags are exactly what production renders.
func enterCrossMode(cross *OutlinerModel, id OutlinerModeID) {
	cross.selectOutlinerMode(id)
	cross.crossForceRebuild = true
	cross.crossProjectRebuildRows()
}

// seedNextOrderOrder commits a workspace focus-queue order to disk so the global
// Next sequence is deterministic (rank order) rather than input order.
func seedNextOrderOrder(t *testing.T, m *AppModel, locators ...config.ProjectTaskLocator) {
	t.Helper()
	items := make([]config.ProjectTaskLocator, len(locators))
	for i, loc := range locators {
		items[i] = config.ProjectTaskLocator{TaskID: loc.TaskID, ProjectID: loc.ProjectID}
	}
	if err := commitPlanningToRuntime(m.runtime, func(p *config.PlanningState) {
		p.SetNext(items)
	}); err != nil {
		t.Fatalf("seed focus queue order: %v", err)
	}
	m.rebuildPlanningFromRuntime()
}

// seedTodayOrder commits a workspace daily-plan order to disk.
func seedTodayOrder(t *testing.T, m *AppModel, locators ...config.ProjectTaskLocator) {
	t.Helper()
	items := make([]config.ProjectTaskLocator, len(locators))
	for i, loc := range locators {
		items[i] = config.ProjectTaskLocator{TaskID: loc.TaskID, ProjectID: loc.ProjectID}
	}
	if err := commitPlanningToRuntime(m.runtime, func(p *config.PlanningState) {
		p.SetToday(items)
	}); err != nil {
		t.Fatalf("seed daily plan order: %v", err)
	}
	m.rebuildPlanningFromRuntime()
}

// diskNextOrderIDs / diskTodayIDs read the committed planning order fresh
// from disk so assertions observe exactly what a restart would load.
func diskNextOrderIDs(t *testing.T, dataDir string) []string {
	t.Helper()
	planning, err := config.NewManager(filepath.Join(dataDir, ".tt.json")).DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	ids := make([]string, 0, len(planning.GetNext()))
	for _, item := range planning.GetNext() {
		ids = append(ids, item.TaskID)
	}
	return ids
}

func diskTodayIDs(t *testing.T, dataDir string) []string {
	t.Helper()
	planning, err := config.NewManager(filepath.Join(dataDir, ".tt.json")).DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	ids := make([]string, 0, len(planning.GetToday()))
	for _, item := range planning.GetToday() {
		ids = append(ids, item.TaskID)
	}
	return ids
}

func assertOrder(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s order = %v, want %v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s order = %v, want %v", label, got, want)
		}
	}
}

// pressReorderKey drives one J/K key through the OperationRegistry grammar and
// routes the emitted planning-reorder command back through the shell, mirroring
// production dispatch end to end. It returns whether a reorder command fired.
func pressReorderKey(t *testing.T, m *AppModel, key rune) bool {
	t.Helper()
	handled, cmd := m.handleGlobalKeyPress(syntheticKey(key))
	if !handled || cmd == nil {
		return false
	}
	msg := cmd()
	if msg == nil {
		return false
	}
	m.Update(msg)
	return true
}

// TestOutlinerPlanningOrderJK verifies that dedicated planning modes and their
// All-mode synthetic groups reorder only the durable planning lists.
func TestOutlinerPlanningOrderJK(t *testing.T) {
	t.Run("Next J reorders the durable focus queue and persists after reload", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-next", "next", "Alpha", "Bravo", "Charlie")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]), locatorOf(project, tasks[2]),
		)
		enterCrossMode(cross, ModeNext)
		if cross.primaryMode != ModeNext {
			t.Fatal("ModeNext did not become the global Outliner mode")
		}
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J in Next produced no reorder command")
		}

		// Alpha moved down past Bravo; the durable focus queue reflects it, and the
		// task tree is untouched (still three roots in their original model order).
		assertOrder(t, "focus queue after J", diskNextOrderIDs(t, dataDir),
			[]string{tasks[1].ID, tasks[0].ID, tasks[2].ID})
		assertTreeUnchanged(t, project, []string{tasks[0].ID, tasks[1].ID, tasks[2].ID})

		// A rebuild from the committed runtime keeps the reordered sequence.
		enterCrossMode(cross, ModeNext)
		assertOrder(t, "focus queue after reload", diskNextOrderIDs(t, dataDir),
			[]string{tasks[1].ID, tasks[0].ID, tasks[2].ID})
	})

	t.Run("Next K moves the cursor task up in the focus queue", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-next2", "next2", "Alpha", "Bravo", "Charlie")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]), locatorOf(project, tasks[2]),
		)
		enterCrossMode(cross, ModeNext)
		if !cross.moveCursorToProjectTask(tasks[2].ID, project.ID) {
			t.Fatal("cursor did not land on Charlie")
		}

		if !pressReorderKey(t, m, 'K') {
			t.Fatal("K in Next produced no reorder command")
		}

		assertOrder(t, "focus queue after K", diskNextOrderIDs(t, dataDir),
			[]string{tasks[0].ID, tasks[2].ID, tasks[1].ID})
	})

	t.Run("all-project grouped Next order follows visible project groups", func(t *testing.T) {
		dir := t.TempDir()
		p1, t1 := planningOrderProject(t, dir, "proj-1", "one", "A", "C")
		p2, t2 := planningOrderProject(t, dir, "proj-2", "two", "B", "D")
		m, cross, dataDir := newPlanningOrderApp(t, p1, p2)
		// Interleave the two projects' Next tasks: A(p1), B(p2), C(p1), D(p2).
		seedNextOrderOrder(t, m,
			locatorOf(p1, t1[0]), locatorOf(p2, t2[0]), locatorOf(p1, t1[1]), locatorOf(p2, t2[1]),
		)
		enterCrossMode(cross, ModeNext)
		if !cross.moveCursorToProjectTask(t2[0].ID, p2.ID) {
			t.Fatal("cursor did not land on B")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J at the all-project root produced no workspace reorder")
		}

		// The visible order is A, C under project one, then B, D under project two.
		// Moving B down therefore swaps it with D in the durable interleaved list.
		want := []string{t1[0].ID, t2[1].ID, t1[1].ID, t2[0].ID}
		assertOrder(t, "grouped focus queue", diskNextOrderIDs(t, dataDir), want)
	})

	t.Run("Today J reorders the durable daily plan without changing hierarchy", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-today", "today", "Alpha", "Bravo", "Charlie")
		for _, task := range tasks {
			task.ApplyWorkflowState(models.StateBacklog)
		}
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedTodayOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]), locatorOf(project, tasks[2]),
		)
		enterCrossMode(cross, ModeToday)
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J in Today produced no reorder command")
		}

		assertOrder(t, "daily plan after J", diskTodayIDs(t, dataDir),
			[]string{tasks[1].ID, tasks[0].ID, tasks[2].ID})
		assertTreeUnchanged(t, project, []string{tasks[0].ID, tasks[1].ID, tasks[2].ID})
	})

	t.Run("All-mode Next group reorders the durable focus queue", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-all-next", "all-next", "Alpha", "Bravo")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m, locatorOf(project, tasks[0]), locatorOf(project, tasks[1]))
		enterCrossMode(cross, ModeAll)
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}
		if !cross.rows[cross.cursor].IsNextSection {
			t.Fatal("All-mode Next row lost its planning-section identity")
		}
		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J in All-mode Next produced no reorder command")
		}
		assertOrder(t, "All-mode Next after J", diskNextOrderIDs(t, dataDir),
			[]string{tasks[1].ID, tasks[0].ID})
	})
}

func TestTodayPlanningReorderCrossesLifecycleStatesInDurableOrder(t *testing.T) {
	project, tasks := planningOrderProject(t, t.TempDir(), "p-state-order", "state-order", "Waiting", "Backlog")
	tasks[0].ApplyWorkflowState(models.StateWaiting)
	tasks[1].ApplyWorkflowState(models.StateBacklog)
	m, cross, dataDir := newPlanningOrderApp(t, project)
	seedTodayOrder(t, m, locatorOf(project, tasks[0]), locatorOf(project, tasks[1]))
	enterCrossMode(cross, ModeToday)
	assertOrder(t, "Today before cross-state reorder", visibleTaskIDs(cross),
		[]string{tasks[0].ID, tasks[1].ID})
	if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
		t.Fatal("cursor did not land on Waiting")
	}
	if !pressReorderKey(t, m, 'J') {
		t.Fatal("J across lifecycle states produced no planning reorder")
	}
	want := []string{tasks[1].ID, tasks[0].ID}
	assertOrder(t, "Today after cross-state reorder", diskTodayIDs(t, dataDir), want)
	if m.statusMessage != "Reordered planning list" {
		t.Fatalf("status = %q, want reorder success", m.statusMessage)
	}
	cross.loadedProjects = []*models.Project{project}
	enterCrossMode(cross, ModeToday)
	assertOrder(t, "Today after rebuild", visibleTaskIDs(cross), want)
	assertTreeUnchanged(t, project, []string{tasks[0].ID, tasks[1].ID})
}

// TestOutlinerPlanningOrderJKFilteredAndScoped locks FR17-002: when the visible
// planning rows are narrowed by a filter or a single-project scope, J/K swaps the
// two *visible* identities in the durable order rather than swapping the cursor
// with a hidden global neighbor. Hidden planning members stay intact and unmoved
// relative to the requested pair.
func TestOutlinerPlanningOrderJKFilteredAndScoped(t *testing.T) {
	t.Run("filtered Next swaps visible neighbors, not the hidden middle task", func(t *testing.T) {
		// Durable focus queue: keep-one, hide-me, keep-two. A tag facet hides
		// the middle task, so keep-one's visible neighbor is keep-two.
		project, tasks := planningOrderProject(t, t.TempDir(), "p-fnext", "fnext", "keep-one", "hide-me", "keep-two")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]), locatorOf(project, tasks[2]),
		)
		tasks[0].Tags = []string{"keep"}
		tasks[2].Tags = []string{"keep"}
		cross.filter.Tags = map[string]bool{"keep": true}
		enterCrossMode(cross, ModeNext)

		if got := visibleTaskIDs(cross); len(got) != 2 {
			t.Fatalf("filtered Next visible rows = %v, want 2 (hidden middle pruned)", got)
		}
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on keep-one")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J in filtered Next produced no reorder command")
		}

		// keep-one swaps with its visible neighbor keep-two; the hidden hide-me
		// stays put at index 1. The buggy behavior would have swapped keep-one
		// with hide-me (the durable neighbor), leaving keep-one visually stuck.
		assertOrder(t, "filtered focus queue after J", diskNextOrderIDs(t, dataDir),
			[]string{tasks[2].ID, tasks[1].ID, tasks[0].ID})
	})

	t.Run("filtered Today swaps visible neighbors, not the hidden middle task", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-ftoday", "ftoday", "keep-one", "hide-me", "keep-two")
		for _, task := range tasks {
			task.ApplyWorkflowState(models.StateBacklog)
		}
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedTodayOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]), locatorOf(project, tasks[2]),
		)
		tasks[0].Tags = []string{"keep"}
		tasks[2].Tags = []string{"keep"}
		cross.filter.Tags = map[string]bool{"keep": true}
		enterCrossMode(cross, ModeToday)

		if got := visibleTaskIDs(cross); len(got) != 2 {
			t.Fatalf("filtered Today visible rows = %v, want 2 (hidden middle pruned)", got)
		}
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on keep-one")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J in filtered Today produced no reorder command")
		}

		assertOrder(t, "filtered daily plan after J", diskTodayIDs(t, dataDir),
			[]string{tasks[2].ID, tasks[1].ID, tasks[0].ID})
		assertTreeUnchanged(t, project, []string{tasks[0].ID, tasks[1].ID, tasks[2].ID})
	})

	t.Run("single-project scope swaps within scope, leaving other projects unmoved", func(t *testing.T) {
		dir := t.TempDir()
		p1, t1 := planningOrderProject(t, dir, "scope-1", "s-one", "A", "C")
		p2, t2 := planningOrderProject(t, dir, "scope-2", "s-two", "B", "D")
		m, cross, dataDir := newPlanningOrderApp(t, p1, p2)
		// Interleave: A(p1), B(p2), C(p1), D(p2).
		seedNextOrderOrder(t, m,
			locatorOf(p1, t1[0]), locatorOf(p2, t2[0]), locatorOf(p1, t1[1]), locatorOf(p2, t2[1]),
		)
		// Scope to project one: only A and C are visible.
		cross.scopedProjectID = p1.ID
		enterCrossMode(cross, ModeNext)

		if got := visibleTaskIDs(cross); len(got) != 2 {
			t.Fatalf("scoped Next visible rows = %v, want 2 (project one only)", got)
		}
		if !cross.moveCursorToProjectTask(t1[0].ID, p1.ID) {
			t.Fatal("cursor did not land on A")
		}

		// J on A: its visible neighbor is C (B is out of scope). Swap A and C in
		// the durable order; the other project's B and D never move.
		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J in scoped Next produced no reorder command")
		}

		want := []string{t1[1].ID, t2[0].ID, t1[0].ID, t2[1].ID} // C, B, A, D
		assertOrder(t, "scoped focus queue after J", diskNextOrderIDs(t, dataDir), want)
	})

	t.Run("J is a no-op at the visible boundary under a filter", func(t *testing.T) {
		// Only keep-one is visible; the rows below it are all hidden, so J on
		// keep-one finds no visible planning neighbor and must not swap with them.
		project, tasks := planningOrderProject(t, t.TempDir(), "p-bound", "bound", "keep-one", "hide-me", "hide-two")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]), locatorOf(project, tasks[2]),
		)
		tasks[0].Tags = []string{"keep"}
		cross.filter.Tags = map[string]bool{"keep": true}
		enterCrossMode(cross, ModeNext)

		if got := visibleTaskIDs(cross); len(got) != 1 {
			t.Fatalf("filtered visible rows = %v, want 1", got)
		}
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on keep-one")
		}

		if pressReorderKey(t, m, 'J') {
			t.Fatal("J at the visible boundary emitted a reorder command; it must be a no-op")
		}
		// Durable order untouched: no swap with the hidden tasks below.
		assertOrder(t, "focus queue after boundary no-op", diskNextOrderIDs(t, dataDir),
			[]string{tasks[0].ID, tasks[1].ID, tasks[2].ID})
	})
}

func TestHierarchicalReorderSwapsAdjacentVisibleQualifiedSiblings(t *testing.T) {
	project, tasks := planningOrderProject(t, t.TempDir(), "hierarchy", "hierarchy", "keep-one", "hide-me", "keep-two")
	firstID, hiddenID, lastID := tasks[0].ID, tasks[1].ID, tasks[2].ID
	for _, task := range tasks {
		task.ApplyWorkflowState(models.StateBacklog)
	}
	m, cross, _ := newPlanningOrderApp(t, project)
	cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	tasks[0].Tags = []string{"keep"}
	tasks[2].Tags = []string{"keep"}
	cross.filter.Tags = map[string]bool{"keep": true}
	enterCrossMode(cross, ModeBacklog)
	if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
		t.Fatal("cursor did not land on keep-one")
	}
	if !cross.simplifiedReorderEligible() {
		t.Fatal("hierarchical row was not reorder eligible")
	}
	if !pressReorderKey(t, m, 'J') {
		t.Fatal("J on a hierarchical row produced no reorder command")
	}
	if m.err != nil {
		t.Fatalf("hierarchical reorder failed: %v", m.err)
	}
	if cross.crossReloadProjectID != project.ID || cross.crossReloadTaskID != firstID {
		t.Fatalf("reload cursor anchor = %s/%s, want %s/%s",
			cross.crossReloadProjectID, cross.crossReloadTaskID, project.ID, firstID)
	}
	cross.loadedProjects = []*models.Project{project}
	cross.crossForceRebuild = true
	cross.crossProjectRebuildRows()
	cross.restoreCrossOutlinerCursorAfterReload(cross.cursor, cross.viewScrollOffset)
	if task, selectedProject := cross.getTaskRef(); task == nil || selectedProject == nil ||
		task.ID != firstID || selectedProject.ID != "hierarchy" {
		t.Fatalf("cursor after reorder = %v/%v, want hierarchy/%s", selectedProject, task, firstID)
	}

	// The two adjacent visible siblings swap explicitly around the hidden
	// sibling. A model-adjacent MoveDown would incorrectly produce
	// hide-me, keep-one, keep-two.
	assertTreeUnchanged(t, project, []string{lastID, hiddenID, firstID})
}

// visibleTaskIDs returns the task IDs of the outliner's current task rows in
// render order, ignoring headers and separators.
func visibleTaskIDs(m *OutlinerModel) []string {
	var ids []string
	for _, row := range m.rows {
		if row.isTaskRow() && row.task != nil {
			ids = append(ids, row.task.ID)
		}
	}
	return ids
}

// locatorOf builds a project-qualified locator for a seeded task.
func locatorOf(project *models.Project, task *models.Task) config.ProjectTaskLocator {
	return config.ProjectTaskLocator{ProjectID: project.ID, TaskID: task.ID}
}

// assertTreeUnchanged reloads the project from disk and asserts its root task IDs
// still match wantRootIDs in order, proving a planning reorder never rewrote the
// task hierarchy.
func assertTreeUnchanged(t *testing.T, project *models.Project, wantRootIDs []string) {
	t.Helper()
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	roots := fresh.GetRootTasks()
	got := make([]string, 0, len(roots))
	for _, r := range roots {
		got = append(got, r.ID)
	}
	assertOrder(t, "task tree roots", got, wantRootIDs)
}
