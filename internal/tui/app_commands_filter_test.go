package tui

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestFilterCommands_FilterByTag_WithProjectTree(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "filter by tag")
	if cmd == nil {
		t.Fatal("expected 'filter by tag' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTagFilterMsg); !ok {
		t.Errorf("expected ShowTagFilterMsg, got %T", msg)
	}
}

func TestFilterCommands_FilterByTag_WithOutliner(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.pushView(ViewOutliner)

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "filter by tag")
	if cmd == nil {
		t.Fatal("expected 'filter by tag' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTagFilterMsg); !ok {
		t.Errorf("expected ShowTagFilterMsg, got %T", msg)
	}
}

func TestFilterCommands_FilterByTag_NilBoth(t *testing.T) {
	app := newTestAppModel()

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "filter by tag")
	if cmd == nil {
		t.Fatal("expected 'filter by tag' command")
	}
	msg := cmd.Handler()
	if msg != nil {
		t.Errorf("expected nil, got %T", msg)
	}
}

func TestFilterCommands_FilterByTag_WithCrossProjectOutlinerCatalogTags(t *testing.T) {
	app := newTestAppModel()
	alpha := &models.Project{
		ID:   "alpha",
		Name: "Alpha",
		Tags: []string{"catalog-only", "shared"},
		Tasks: []*models.Task{
			{Tags: []string{"shared", "task-only"}},
		},
	}
	beta := &models.Project{
		ID:   "beta",
		Name: "Beta",
		Tags: []string{"beta-only"},
	}

	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
	app.crossOutliner.loadedProjects = []*models.Project{alpha, beta}
	app.crossOutliner.filter.Tags = map[string]bool{"catalog-only": true}
	app.pushView(ViewCrossProjectOutliner)

	cmd := findCommand(app.filterCommands(), "filter by tag")
	if cmd == nil {
		t.Fatal("expected 'filter by tag' command")
	}
	msg, ok := cmd.Handler().(ShowTagFilterMsg)
	if !ok {
		t.Fatalf("expected ShowTagFilterMsg, got %T", msg)
	}
	if msg.Source != ViewCrossProjectOutliner {
		t.Fatalf("Source = %s, want %s", msg.Source, ViewCrossProjectOutliner)
	}
	if !msg.Selected["catalog-only"] {
		t.Fatalf("Selected = %v, want catalog-only preserved", msg.Selected)
	}

	got := summarizeTagInfos(msg.Tags)
	want := []string{"beta-only:0", "catalog-only:0", "shared:1", "task-only:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Tags = %v, want %v", got, want)
	}
}

func TestFilterCommands_FilterBySection(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "filter by section")
	if cmd == nil {
		t.Fatal("expected 'filter by section' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowSectionFilterSelectorMsg); !ok {
		t.Errorf("expected ShowSectionFilterSelectorMsg, got %T", msg)
	}
}

func TestFilterCommands_ClearFilters_ProjectTree(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	app.projectTree.filter.Priority = 1

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "clear filters")
	if cmd == nil {
		t.Fatal("expected 'clear filters' command")
	}
	cmd.Handler()

	if app.projectTree.filter.Priority != 0 {
		t.Errorf("expected Priority to be reset to 0, got %d", app.projectTree.filter.Priority)
	}
}

func TestFilterCommands_ClearFilters_Outliner(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.pushView(ViewOutliner)

	app.outliner.filter.Priority = 1

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "clear filters")
	if cmd == nil {
		t.Fatal("expected 'clear filters' command")
	}
	cmd.Handler()

	if app.outliner.filter.Priority != 0 {
		t.Errorf("expected Priority to be reset to 0, got %d", app.outliner.filter.Priority)
	}
}

func TestFilterCommands_JumpToTask(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "jump to task")
	if cmd == nil {
		t.Fatal("expected 'jump to task' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTaskJumperMsg); !ok {
		t.Errorf("expected ShowTaskJumperMsg, got %T", msg)
	}
}

func TestFilterCommands_JumpToTask_NilProject(t *testing.T) {
	app := newTestAppModel()

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "jump to task")
	if cmd == nil {
		t.Fatal("expected 'jump to task' command")
	}
	msg := cmd.Handler()
	if msg != nil {
		t.Errorf("expected nil, got %T", msg)
	}
}

func TestFilterCommands_TagManager(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "tag manager")
	if cmd == nil {
		t.Fatal("expected 'tag manager' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowTagManagerMsg); !ok {
		t.Errorf("expected ShowTagManagerMsg, got %T", msg)
	}
}

func TestFilterCommands_GlobalTagManager(t *testing.T) {
	app := newTestAppModel()

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "global tag manager")
	if cmd == nil {
		t.Fatal("expected 'global tag manager' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowGlobalTagManagerMsg); !ok {
		t.Errorf("expected ShowGlobalTagManagerMsg, got %T", msg)
	}
}

func TestFilterCommands_GlobalTagManagerAvailableInCrossProjectOutliner(t *testing.T) {
	app := newTestAppModel()

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "global tag manager")
	if cmd == nil {
		t.Fatal("expected 'global tag manager' command")
	}
	for _, view := range []ViewType{ViewDashboard, ViewCrossProjectOutliner} {
		if !containsView(cmd.Views, view) {
			t.Fatalf("global tag manager views = %v, want %s", cmd.Views, view)
		}
	}

	registry := NewCommandRegistry()
	if err := registry.RegisterBatch(cmds); err != nil {
		t.Fatalf("RegisterBatch: %v", err)
	}
	if cmd := findCommand(registry.GetForView(ViewCrossProjectOutliner), "global tag manager"); cmd == nil {
		t.Fatal("expected command registry to expose global tag manager for cross-project outliner")
	}
}

func TestFilterCommands_SectionManager(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "custom section manager")
	if cmd == nil {
		t.Fatal("expected 'custom section manager' command")
	}
	msg := cmd.Handler()
	if _, ok := msg.(ShowSectionManagerMsg); !ok {
		t.Errorf("expected ShowSectionManagerMsg, got %T", msg)
	}
}

func TestFilterCommands_OpenDataFolder(t *testing.T) {
	app := newTestAppModel()

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "open data folder")
	if cmd == nil {
		t.Fatal("expected 'open data folder' command")
	}
	msg := cmd.Handler()
	m, ok := msg.(OpenDataFolderMsg)
	if !ok {
		t.Errorf("expected OpenDataFolderMsg, got %T", msg)
	}
	if m.Project != nil {
		t.Errorf("expected nil project, got %v", m.Project)
	}
}

func TestFilterCommands_OpenProjectFolder(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.pushView(ViewProjectTree)

	cmds := app.filterCommands()
	cmd := findCommand(cmds, "open project folder")
	if cmd == nil {
		t.Fatal("expected 'open project folder' command")
	}
	msg := cmd.Handler()
	m, ok := msg.(OpenDataFolderMsg)
	if !ok {
		t.Errorf("expected OpenDataFolderMsg, got %T", msg)
	}
	if m.Project != project {
		t.Errorf("expected project to match current project")
	}
}
