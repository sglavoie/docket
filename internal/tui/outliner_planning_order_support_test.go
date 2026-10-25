package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/operationhost"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/taskref"
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
func newPlanningOrderApp(t *testing.T, projects ...*models.Project) (*AppModel, *outliner.OutlinerModel, string) {
	t.Helper()
	dataDir := t.TempDir()
	for _, p := range projects {
		p.Path = filepath.Join(dataDir, "projects", p.Slug)
	}

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)
	repo := &failableScenarioRepo{scenarioRepo: newScenarioRepo(commitSvc, projects...)}
	repo.seed(t)

	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, projects)
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil).
		WithPlanningArtifacts(coordinator.ManagerPlanningArtifacts(manager))

	projectMap := make(map[string]*models.Project, len(projects))
	for _, p := range projects {
		projectMap[p.ID] = p
	}

	domain := newDomainController(runtime, manager)
	domain.service = svc
	domain.ledger = ledger
	domain.projects = projectMap

	m := &AppModel{
		domain:            domain,
		secondaryFeatures: coordinator.NewSecondaryCoordinators(svc),
		viewNav: viewNavigationController{
			navigation: route.NewNavigationState(),
			views:      route.NewViewRegistry(),
			width:      120,
			height:     40,
		},
		currentProject: projects[0],
		nextOrder:      []outliner.PlanningTask{},
		nextOrderIDs:   make(map[taskref.TaskIdentity]bool),
		today:          []outliner.PlanningTask{},
		todayIDs:       make(map[taskref.TaskIdentity]bool),
		operations:     operationhost.NewController(),
	}
	m.operations.BindExecutor()
	m.rebuildPlanningFromRuntime()

	cross := outliner.NewOutlinerModel(nil, manager, runtime)
	cross.Population.Projects = projects
	if len(projects) == 1 {
		cross.Scope.ProjectID = projects[0].ID
	}
	cross.SetSize(120, 40)
	m.crossOutliner = cross
	m.viewNav.views.Register(presentation.ViewCrossProjectOutliner, cross)
	if err := m.registerCoreInputOperations(); err != nil {
		t.Fatalf("register core operations: %v", err)
	}
	if err := m.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task action operations: %v", err)
	}
	if err := m.filters.RegisterFilterOperations(m); err != nil {
		t.Fatalf("register filter operations: %v", err)
	}
	if err := m.filters.RegisterSortOperations(m); err != nil {
		t.Fatalf("register sort operations: %v", err)
	}
	if err := m.registerProjectOperations(); err != nil {
		t.Fatalf("register project operations: %v", err)
	}
	if err := m.registerProjectActionSheetOperation(); err != nil {
		t.Fatalf("register project action sheet operation: %v", err)
	}
	if err := m.registerChecklistRowOperations(); err != nil {
		t.Fatalf("register checklist row operations: %v", err)
	}
	if err := m.operations.Registry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified input: %v", err)
	}
	m.wireOutlinerOperationExecutor(cross)
	return m, cross, dataDir
}

// enterCrossMode selects a mode on the cross Outliner and rebuilds its
// rows so the mode's live rows and flags are exactly what production renders.
func enterCrossMode(cross *outliner.OutlinerModel, id chrome.OutlinerModeID) {
	cross.SelectOutlinerMode(id)
	cross.Population.Invalidate()
	cross.CrossProjectRebuildRows()
}

// seedNextOrderOrder commits a workspace focus-queue order to disk so the global
// Next sequence is deterministic (rank order) rather than input order.
func seedNextOrderOrder(t *testing.T, m *AppModel, locators ...config.ProjectTaskLocator) {
	t.Helper()
	items := make([]config.ProjectTaskLocator, len(locators))
	for i, loc := range locators {
		items[i] = config.ProjectTaskLocator{TaskID: loc.TaskID, ProjectID: loc.ProjectID}
	}
	if err := commitPlanningToRuntime(m.domain.runtime, func(p *config.PlanningState) {
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
	if err := commitPlanningToRuntime(m.domain.runtime, func(p *config.PlanningState) {
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
	return pressReorderMsg(t, m, syntheticKey(key))
}

// pressReorderCtrlKey is pressReorderKey for the Ctrl+J/Ctrl+K edge moves.
func pressReorderCtrlKey(t *testing.T, m *AppModel, key rune) bool {
	t.Helper()
	return pressReorderMsg(t, m, syntheticCtrlKey(key))
}

func pressReorderMsg(t *testing.T, m *AppModel, key tea.KeyPressMsg) bool {
	t.Helper()
	handled, cmd := m.handleGlobalKeyPress(key)
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

// visibleTaskIDs returns the task IDs of the outliner's current task rows in
// render order, ignoring headers and separators.
func visibleTaskIDs(m *outliner.OutlinerModel) []string {
	var ids []string
	for _, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil {
			ids = append(ids, row.Task.ID)
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
