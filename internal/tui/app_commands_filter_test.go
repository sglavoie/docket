package tui

import "testing"

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