package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

func TestOtherCommands_ProjectFocus(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "project focus queue")
	if cmd == nil {
		t.Fatal("expected to find 'project focus queue' command")
	}
	for _, alias := range cmd.Aliases {
		if alias == "project active tasks" {
			t.Fatal("project Focus Queue command should not describe queued work as active")
		}
	}

	result := cmd.Handler()
	if _, ok := result.(ShowProjectFocusViewMsg); !ok {
		t.Errorf("expected ShowProjectFocusViewMsg, got %T", result)
	}
}

func TestOtherCommands_ProjectFocus_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.currentProject = nil

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "project focus queue")
	if cmd == nil {
		t.Fatal("expected to find 'project focus queue' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFocusQueuePaletteHasNoLegacyRemovalCommand(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := NewCommandRegistry()
	for i, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch[%d] failed: %v", i, err)
		}
	}

	for _, view := range []PresentationID{ViewProjectFocusView} {
		if cmd := findCommand(registry.GetForView(view), "remove from focus"); cmd != nil {
			t.Fatalf("%s palette should not expose legacy queue removal: %+v", view, cmd)
		}
	}
}

func TestOtherCommands_BlockersView(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "blockers view")
	if cmd == nil {
		t.Fatal("expected to find 'blockers view' command")
	}

	result := cmd.Handler()
	msg, ok := result.(ShowBlockersMsg)
	if !ok {
		t.Fatalf("expected ShowBlockersMsg, got %T", result)
	}
	if msg.Project == nil {
		t.Error("expected Project to be set")
	}
}

func TestOtherCommands_BlockersView_DashboardUsesSelectedProject(t *testing.T) {
	app, firstProject := newTestAppModelWithProject()
	secondProject := testutil.NewTestProjectWithTasks("Second Project")
	addTestAppProject(app, secondProject)
	app.currentProject = firstProject
	app.dashboard = NewDashboardModel(app.runtime)
	visible := app.dashboard.visibleProjects()
	if len(visible) < 2 {
		t.Fatalf("expected at least 2 visible projects, got %d", len(visible))
	}
	app.dashboard.cursor = len(visible) - 1
	selectedProject := visible[app.dashboard.cursor]
	if selectedProject == firstProject {
		app.currentProject = secondProject
	}

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "blockers view")
	if cmd == nil {
		t.Fatal("expected to find 'blockers view' command")
	}

	result := cmd.Handler()
	msg, ok := result.(ShowBlockersMsg)
	if !ok {
		t.Fatalf("expected ShowBlockersMsg, got %T", result)
	}
	if msg.Project == nil || msg.Project.ID != selectedProject.ID {
		t.Errorf("expected selected dashboard project %q, got %#v", selectedProject.Name, msg.Project)
	}
}
