package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// typedTestApp wires the same typed route, operation, and Outliner foundations
// as production without requiring file-backed project fixtures.
func typedTestApp(t *testing.T) *AppModel {
	t.Helper()
	model, project := newTestAppModelWithProject()
	service := app.NewService(newTestAppRepo(project), nil, nil)
	model.appService = service
	model.features = newFeatureCoordinators(service, service)
	model.secondaryFeatures = newSecondaryCoordinators(service, model.configManager)
	model.deps = newShellDeps(model.runtime, service, model.configManager, nil)
	model.operationRegistry = NewOperationRegistry()
	model.operationExecutor = NewOperationExecutor(model.operationRegistry)

	model.crossOutliner = NewOutlinerModel(nil, model.configManager, model.runtime)
	model.crossOutliner.loadedProjects = []*models.Project{project}
	model.crossOutliner.SetSize(model.width, model.height-2)
	model.views.Register(ViewCrossProjectOutliner, model.crossOutliner)

	if err := model.registerCoreInputOperations(); err != nil {
		t.Fatalf("register core input operations: %v", err)
	}
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task action operations: %v", err)
	}
	if err := model.registerFilterOperations(); err != nil {
		t.Fatalf("register filter operations: %v", err)
	}
	if err := model.registerScopeHubOperations(); err != nil {
		t.Fatalf("register scope hub operations: %v", err)
	}
	if err := model.registerProjectOperations(); err != nil {
		t.Fatalf("register project operations: %v", err)
	}
	if err := model.registerNoteOperations(); err != nil {
		t.Fatalf("register note operations: %v", err)
	}
	if err := model.registerTagOperations(); err != nil {
		t.Fatalf("register tag operations: %v", err)
	}
	if err := model.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified input strategy: %v", err)
	}
	model.wireOutlinerOperationExecutor(model.crossOutliner)
	model.ensureManageHub()
	return model
}

func scopeTestProject(id, name string, taskIDs ...string) *models.Project {
	tasks := make([]*models.Task, 0, len(taskIDs))
	for i, taskID := range taskIDs {
		tasks = append(tasks, &models.Task{
			ID:            taskID,
			ShortID:       i + 1,
			Title:         taskID,
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateReady,
		})
	}
	project := &models.Project{
		ID:          id,
		Name:        name,
		Slug:        id,
		Tasks:       tasks,
		NextShortID: len(tasks) + 1,
	}
	project.BuildTaskTree()
	return project
}

func taskRowProjectIDs(outliner *OutlinerModel) []string {
	if outliner == nil {
		return nil
	}
	seen := make(map[string]bool)
	ids := make([]string, 0)
	for _, row := range outliner.rows {
		if !row.isTaskRow() || row.ProjectID == "" || seen[row.ProjectID] {
			continue
		}
		seen[row.ProjectID] = true
		ids = append(ids, row.ProjectID)
	}
	return ids
}

func containsProject(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func scopeTestOutliner(projects ...*models.Project) *OutlinerModel {
	runtime := testConfigWithProjects(projects...)
	outliner := NewOutlinerModel(nil, nil, runtime)
	outliner.loadedProjects = projects
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	return outliner
}

func scopeVisibleProjectIDs(outliner *OutlinerModel) map[string]bool {
	ids := make(map[string]bool)
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil {
			ids[row.ProjectID] = true
		}
	}
	return ids
}

func newCrossOutlinerFromEnv(env *crossProjectTestEnv) *OutlinerModel {
	runtime := newTestConfigAt(env.dataDir)
	setTestProjects(runtime, env.projects)
	outliner := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(runtime.DataDir(), runtime)),
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[TaskIdentity]bool),
		MultiSelectState: MultiSelectState{
			selectedTaskIdentities: make(map[TaskIdentity]bool),
		},
		loadedProjects: env.projects,
	}
	outliner.bulk = env.bulk
	outliner.taskItem = env.taskItem
	return outliner
}

func newWorkModeOutliner(t *testing.T) *OutlinerModel {
	t.Helper()
	tasks := []*models.Task{
		makeTask("t-inbox", "Capture something", models.StateInbox),
		makeTask("t-next", "Do the thing", models.StateNext),
		makeTask("t-active", "In progress", ""),
	}
	project := renderFixtureProject(t, "proj-modes", "mod", "Modes Project", tasks)
	runtime := newTestConfig()
	outliner := NewOutlinerModel(project, nil, testRuntimeAt(runtime.DataDir(), runtime))
	outliner.SetSize(120, 24)
	outliner.rebuildRows()
	return outliner
}

func renderFixtureProject(t *testing.T, id, slug, name string, tasks []*models.Task) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, name, t.TempDir())
	if err != nil {
		t.Fatalf("NewProject(%q): %v", id, err)
	}
	project.Slug = slug
	project.Tasks = tasks
	project.BuildTaskTree()
	return project
}

func renderFixtureTask(
	t *testing.T,
	id string,
	shortID int,
	taskType models.TaskType,
	title string,
	status models.WorkflowState,
	priority int,
	section models.WorkflowState,
) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, taskType, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.WorkflowState = legacyStateForStatusSection(status, string(section))
	task.Priority = priority
	return task
}

func findPlanningTaskRow(outliner *OutlinerModel, taskID string) (outlinerRow, bool) {
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil && row.task.ID == taskID {
			return row, true
		}
	}
	return outlinerRow{}, false
}

func countTaskRows(outliner *OutlinerModel, taskID string) int {
	count := 0
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil && row.task.ID == taskID {
			count++
		}
	}
	return count
}

func testProjectByID(t *testing.T, projects []*models.Project, projectID string) *models.Project {
	t.Helper()
	for _, project := range projects {
		if project != nil && project.ID == projectID {
			return project
		}
	}
	t.Fatalf("project %q not found", projectID)
	return nil
}

func visibleOutlinerTaskIDs(outliner *OutlinerModel) map[string]bool {
	ids := make(map[string]bool)
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil {
			ids[row.task.ID] = true
		}
	}
	return ids
}
