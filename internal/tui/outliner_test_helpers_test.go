package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// typedTestApp wires the same typed route, operation, and Outliner foundations
// as production without requiring file-backed project fixtures.
func typedTestApp(t *testing.T) *AppModel {
	t.Helper()
	model, project := newTestAppModelWithProject()
	service := app.NewServiceWithArtifacts(newTestAppRepo(project), nil, nil, nil)
	model.domain.service = service
	model.features = coordinator.NewFeatureCoordinators(service)
	model.secondaryFeatures = coordinator.NewSecondaryCoordinators(service)
	model.domain.ledger = nil
	model.operations.Registry = operation.NewOperationRegistry()
	model.operations.Executor = operation.NewOperationExecutor(model.operations.Registry)

	model.crossOutliner = outliner.NewOutlinerModel(nil, model.domain.manager, model.domain.runtime)
	model.crossOutliner.Population.Projects = []*models.Project{project}
	model.crossOutliner.SetSize(model.viewNav.viewSize())
	model.viewNav.views.Register(presentation.ViewCrossProjectOutliner, model.crossOutliner)

	if err := model.registerCoreInputOperations(); err != nil {
		t.Fatalf("register core input operations: %v", err)
	}
	if err := model.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task action operations: %v", err)
	}
	if err := model.filters.RegisterFilterOperations(model); err != nil {
		t.Fatalf("register filter operations: %v", err)
	}
	if err := model.filters.RegisterSortOperations(model); err != nil {
		t.Fatalf("register sort operations: %v", err)
	}
	if err := model.registerProjectHubOperations(); err != nil {
		t.Fatalf("register project hub operations: %v", err)
	}
	if err := model.registerProjectOperations(); err != nil {
		t.Fatalf("register project operations: %v", err)
	}
	if err := model.registerProjectActionSheetOperation(); err != nil {
		t.Fatalf("register project action sheet operation: %v", err)
	}
	if err := model.registerProjectBulkOperations(); err != nil {
		t.Fatalf("register project bulk operations: %v", err)
	}
	if err := model.registerNoteOperations(); err != nil {
		t.Fatalf("register note operations: %v", err)
	}
	if err := model.registerNotesSurfaceOperations(); err != nil {
		t.Fatalf("register notes surface operations: %v", err)
	}
	if err := model.registerAttachmentOperations(); err != nil {
		t.Fatalf("register attachment operations: %v", err)
	}
	if err := model.registerCommentOperations(); err != nil {
		t.Fatalf("register comment operations: %v", err)
	}
	if err := model.registerChecklistRowOperations(); err != nil {
		t.Fatalf("register checklist row operations: %v", err)
	}
	if err := model.operations.RegisterCatalog(nil, surfaceOperationGates()); err != nil {
		t.Fatalf("register surface input strategies: %v", err)
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
			WorkflowState: models.StateBacklog,
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

func taskRowProjectIDs(outlinerView *outliner.OutlinerModel) []string {
	if outlinerView == nil {
		return nil
	}
	seen := make(map[string]bool)
	ids := make([]string, 0)
	for _, row := range outlinerView.Rows {
		if !row.IsTaskRow() || row.ProjectID == "" || seen[row.ProjectID] {
			continue
		}
		seen[row.ProjectID] = true
		ids = append(ids, row.ProjectID)
	}
	return ids
}

func scopeTestOutliner(projects ...*models.Project) *outliner.OutlinerModel {
	runtime := testConfigWithProjects(projects...)
	outlinerView := outliner.NewOutlinerModel(nil, nil, runtime)
	outlinerView.Population.Projects = projects
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	return outlinerView
}

func findPlanningTaskRow(outlinerView *outliner.OutlinerModel, taskID string) (outliner.OutlinerRow, bool) {
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			return row, true
		}
	}
	return outliner.OutlinerRow{}, false
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

func visibleOutlinerTaskIDs(outlinerView *outliner.OutlinerModel) map[string]bool {
	ids := make(map[string]bool)
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil {
			ids[row.Task.ID] = true
		}
	}
	return ids
}

func primaryRowsTask(id string, state models.WorkflowState) *models.Task {
	return &models.Task{
		ID:            id,
		Title:         id,
		WorkflowState: state,
		UpdatedAt:     time.Now(),
	}
}

func primaryRowsProject(id string, tasks ...*models.Task) *models.Project {
	project := &models.Project{ID: id, Name: id, Slug: id, Tasks: tasks}
	project.BuildTaskTree()
	return project
}

func primaryRowsOutliner(projects ...*models.Project) (*outliner.OutlinerModel, *config.RuntimeState) {
	runtime := testConfigWithProjects(projects...)
	outlinerView := outliner.NewOutlinerModel(nil, nil, runtime)
	outlinerView.Population.Projects = projects
	return outlinerView, runtime
}

// searchRankedList holds a query and shows it as the flat ranked list. The tier,
// score, and seating assertions are all about that presentation; a query alone
// narrows the mode's own lanes instead, which the D7 tests cover separately.
func searchRankedList(outlinerView *outliner.OutlinerModel, query string) {
	outlinerView.SetSearchQuery(query)
	if !outlinerView.ShowsRankedList() {
		outlinerView.ToggleRankedList()
	}
}

func searchVisibleTaskIDs(outlinerView *outliner.OutlinerModel) []string {
	result := make([]string, 0)
	for _, identity := range qualifiedVisibleTaskIDs(outlinerView) {
		result = append(result, identity.TaskID)
	}
	return result
}

// recentTask seeds an open task whose last activity is dayOffset days in the
// past. Recent's terminal-state admission is asserted beside the outliner,
// whose copy of this helper takes a state.
func recentTask(id string, dayOffset int) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	task.UpdatedAt = time.Now().AddDate(0, 0, -dayOffset)
	return task
}

func containsProject(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
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
	title string,
	status models.WorkflowState,
	priority int,
	section models.WorkflowState,
) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.WorkflowState = legacyStateForStatusSection(status, string(section))
	task.Priority = priority
	return task
}

// applyOutlinerFilterOperation runs one facet against a bare Outliner. The
// facet cluster takes its shell per call, so a test that only exercises the
// query needs nothing else on the shell but the view itself.
func applyOutlinerFilterOperation(
	view *outliner.OutlinerModel,
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	shell := &AppModel{crossOutliner: view}
	return shell.filters.ExecuteFilterOperation(shell, invocation)
}
