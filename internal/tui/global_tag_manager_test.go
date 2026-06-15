package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestOutlinerCtrlTOpensGlobalTagManagerInCrossProjectMode(t *testing.T) {
	m := &OutlinerModel{}

	_, cmd, handled := m.handleUtilityKey("ctrl+t")
	if !handled {
		t.Fatal("ctrl+t should be handled")
	}
	if cmd == nil {
		t.Fatal("expected ctrl+t to emit a command")
	}
	msg := cmd()
	if _, ok := msg.(ShowGlobalTagManagerMsg); !ok {
		t.Fatalf("ctrl+t emitted %T, want ShowGlobalTagManagerMsg", msg)
	}
}

func TestGlobalTagManagerIncludesCatalogOnlyTags(t *testing.T) {
	project := newGlobalTagTestProject(t, "alpha", "Alpha")
	project.Tags = []string{"catalog-only", "shared"}
	task := newGlobalTagTestTask(t, "task-1", "Tagged task")
	task.Tags = []string{"shared", "task-only"}
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()
	saveGlobalTagTestProject(t, project)

	cfg := testutil.NewTestConfig()
	cfg.Projects = []*models.Project{project}

	manager := NewGlobalTagManagerModel(cfg)

	catalogOnly := findGlobalTagInfo(t, manager.tags, "catalog-only")
	if catalogOnly.TaskCount != 0 {
		t.Fatalf("catalog-only TaskCount = %d, want 0", catalogOnly.TaskCount)
	}
	if len(catalogOnly.Projects) != 1 || catalogOnly.Projects[0].Project != project || catalogOnly.Projects[0].TaskCount != 0 {
		t.Fatalf("catalog-only Projects = %#v, want owning project with zero task count", catalogOnly.Projects)
	}

	shared := findGlobalTagInfo(t, manager.tags, "shared")
	if shared.TaskCount != 1 {
		t.Fatalf("shared TaskCount = %d, want 1", shared.TaskCount)
	}

	taskOnly := findGlobalTagInfo(t, manager.tags, "task-only")
	if taskOnly.TaskCount != 1 {
		t.Fatalf("task-only TaskCount = %d, want 1", taskOnly.TaskCount)
	}
}

func TestGlobalTagManagerNavigatesCatalogOnlyTag(t *testing.T) {
	project := newGlobalTagTestProject(t, "alpha", "Alpha")
	project.Tags = []string{"catalog-only"}
	saveGlobalTagTestProject(t, project)

	cfg := testutil.NewTestConfig()
	cfg.Projects = []*models.Project{project}

	manager := NewGlobalTagManagerModel(cfg)
	manager.cursor = globalTagIndex(t, manager.tags, "catalog-only")

	_, cmd := manager.handleNormalMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter on catalog-only tag to emit navigation command")
	}
	msg := cmd()
	nav, ok := msg.(GlobalTagNavigateMsg)
	if !ok {
		t.Fatalf("enter emitted %T, want GlobalTagNavigateMsg", msg)
	}
	if nav.Project != project || nav.Tag != "catalog-only" {
		t.Fatalf("navigation = (%v, %q), want (%v, catalog-only)", nav.Project, nav.Tag, project)
	}
}

func TestGlobalTagNavigateRespectsDefaultOutlinerView(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.config.DefaultProjectView = "outliner"
	project.Tags = []string{"catalog-only"}
	mockPersister := NewMockPersister()
	mockPersister.Projects[project.ID] = project
	app.persister = mockPersister
	app.pushView(ViewGlobalTagManager)

	handled, _ := app.handleTagMessages(GlobalTagNavigateMsg{Project: project, Tag: "catalog-only"})

	if !handled {
		t.Fatal("GlobalTagNavigateMsg should be handled")
	}
	if app.currentView() != ViewOutliner {
		t.Fatalf("current view = %s, want %s", app.currentView(), ViewOutliner)
	}
	if app.outliner == nil {
		t.Fatal("outliner should be initialized")
	}
	if !app.outliner.filter.Tags["catalog-only"] {
		t.Fatalf("outliner tag filter = %v, want catalog-only", app.outliner.filter.Tags)
	}
}

func TestGlobalTagNavigateCanStillOpenProjectTreeWhenDefault(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.config.DefaultProjectView = "project_tree"
	project.Tags = []string{"catalog-only"}
	mockPersister := NewMockPersister()
	mockPersister.Projects[project.ID] = project
	app.persister = mockPersister
	app.pushView(ViewGlobalTagManager)

	handled, _ := app.handleTagMessages(GlobalTagNavigateMsg{Project: project, Tag: "catalog-only"})

	if !handled {
		t.Fatal("GlobalTagNavigateMsg should be handled")
	}
	if app.currentView() != ViewProjectTree {
		t.Fatalf("current view = %s, want %s", app.currentView(), ViewProjectTree)
	}
	if app.projectTree == nil {
		t.Fatal("project tree should be initialized")
	}
	if !app.projectTree.filter.Tags["catalog-only"] {
		t.Fatalf("project tree tag filter = %v, want catalog-only", app.projectTree.filter.Tags)
	}
}

func TestGlobalTagRenamePersistsCatalogDefinitionsAndTaskUsages(t *testing.T) {
	catalogProject := newGlobalTagTestProject(t, "catalog", "Catalog")
	catalogProject.Tags = []string{"legacy"}
	saveGlobalTagTestProject(t, catalogProject)

	taskProject := newGlobalTagTestProject(t, "tasks", "Tasks")
	task := newGlobalTagTestTask(t, "task-1", "Uses legacy")
	task.Tags = []string{"legacy"}
	taskProject.Tasks = []*models.Task{task}
	taskProject.BuildTaskTree()
	saveGlobalTagTestProject(t, taskProject)

	cfg := testutil.NewTestConfig()
	cfg.Projects = []*models.Project{catalogProject, taskProject}

	manager := NewGlobalTagManagerModel(cfg)
	cmd := manager.renameGlobalTag("legacy", "modern")
	if cmd == nil {
		t.Fatal("expected rename command")
	}
	msg := cmd()
	renamed, ok := msg.(GlobalTagRenamedMsg)
	if !ok {
		t.Fatalf("rename emitted %T, want GlobalTagRenamedMsg", msg)
	}
	if renamed.ProjectCount != 2 || renamed.TaskCount != 1 {
		t.Fatalf("rename counts = (%d projects, %d tasks), want (2 projects, 1 task)", renamed.ProjectCount, renamed.TaskCount)
	}

	if err := storage.LoadProject(catalogProject); err != nil {
		t.Fatalf("LoadProject(catalog): %v", err)
	}
	if !catalogProject.HasCatalogTag("modern") || catalogProject.HasCatalogTag("legacy") {
		t.Fatalf("catalog tags after rename = %v, want modern only", catalogProject.Tags)
	}

	if err := storage.LoadProject(taskProject); err != nil {
		t.Fatalf("LoadProject(tasks): %v", err)
	}
	if len(taskProject.Tasks) != 1 {
		t.Fatalf("task project has %d tasks, want 1", len(taskProject.Tasks))
	}
	if !taskProject.Tasks[0].HasTag("modern") || taskProject.Tasks[0].HasTag("legacy") {
		t.Fatalf("task tags after rename = %v, want modern only", taskProject.Tasks[0].Tags)
	}
}

func newGlobalTagTestProject(t *testing.T, id, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, name, t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	return project
}

func newGlobalTagTestTask(t *testing.T, id, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	return task
}

func saveGlobalTagTestProject(t *testing.T, project *models.Project) {
	t.Helper()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%s): %v", project.Name, err)
	}
}

func findGlobalTagInfo(t *testing.T, tags []GlobalTagInfo, name string) GlobalTagInfo {
	t.Helper()
	for _, tag := range tags {
		if tag.Name == name {
			return tag
		}
	}
	t.Fatalf("tag %q not found in %#v", name, tags)
	return GlobalTagInfo{}
}

func globalTagIndex(t *testing.T, tags []GlobalTagInfo, name string) int {
	t.Helper()
	for i, tag := range tags {
		if tag.Name == name {
			return i
		}
	}
	t.Fatalf("tag %q not found in %#v", name, tags)
	return 0
}
