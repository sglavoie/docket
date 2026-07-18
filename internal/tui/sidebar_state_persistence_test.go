package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

func requireProjectSidebarStateMsg(t *testing.T, cmd tea.Cmd) projectSidebarStateChangedMsg {
	t.Helper()
	for _, msg := range drainTUITestCmd(t, cmd) {
		if stateMsg, ok := msg.(projectSidebarStateChangedMsg); ok {
			return stateMsg
		}
	}
	t.Fatal("command did not emit projectSidebarStateChangedMsg")
	return projectSidebarStateChangedMsg{}
}

func assertSidebarState(t *testing.T, name string, sidebar *SidebarComponent, enabled, focused bool) {
	t.Helper()
	if sidebar.IsEnabled() != enabled || sidebar.IsFocused() != focused {
		t.Fatalf("%s sidebar = (enabled=%v focused=%v), want (enabled=%v focused=%v)",
			name, sidebar.IsEnabled(), sidebar.IsFocused(), enabled, focused)
	}
}

func reloadProjectSidebarEnabled(t *testing.T, app *AppModel, projectID string) bool {
	t.Helper()
	reloaded, err := config.LoadRuntimeState(app.runtime.DataDir())
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	state, ok := reloaded.Machine().GetProjectViewState(projectID)
	if !ok {
		t.Fatalf("reloaded project view state for %q is missing", projectID)
	}
	return state.SidebarEnabled
}

func TestProjectSidebarVisibilityPersistsAndSynchronizesLiveViews(t *testing.T) {
	app, project := newMachineLayoutTestApp(t)
	tree := NewProjectTreeModel(project, app.configManager, app.runtime)
	outliner := NewOutlinerModel(project, app.configManager, app.runtime)
	temporary := NewOutlinerModel(project, app.configManager, app.runtime)
	app.projectTree = tree
	app.outliner = outliner
	app.views.Register(ViewProjectTree, tree)
	app.views.Register(ViewOutliner, outliner)
	app.views.Register(ViewTemporaryOutliner, temporary)

	// The focused-sidebar tab path hides the initiating Outliner. Peers are
	// deliberately focused too so the synchronization assertion also proves
	// disabling blurs every matching live sidebar.
	tree.sidebar.Focus()
	outliner.sidebar.Focus()
	temporary.sidebar.Focus()
	_, cmd := outliner.Update(syntheticSpecialKey(tea.KeyTab))
	msg := requireProjectSidebarStateMsg(t, cmd)
	if msg.ProjectID != project.ID || msg.Enabled {
		t.Fatalf("sidebar message = %+v, want project %q disabled", msg, project.ID)
	}
	if handled, _ := app.handleGlobalSaveMessage(msg); !handled {
		t.Fatal("project sidebar state message was not handled")
	}

	assertSidebarState(t, "tree", tree.sidebar, false, false)
	assertSidebarState(t, "outliner", outliner.sidebar, false, false)
	assertSidebarState(t, "temporary outliner", temporary.sidebar, false, false)
	if enabled := reloadProjectSidebarEnabled(t, app, project.ID); enabled {
		t.Fatal("reloaded sidebar state = enabled, want disabled")
	}
	freshDisabled := NewOutlinerModel(project, app.configManager, mustReloadRuntime(t, app.runtime.DataDir()))
	if freshDisabled.sidebar.IsEnabled() {
		t.Fatal("fresh outliner sidebar = enabled, want restored disabled state")
	}

	// Saving each live peer after the immediate commit must not reverse it with
	// stale model state.
	app.saveProjectTreeState()
	app.saveOutlinerFilterState()
	app.saveOutlinerFilterStateFrom(temporary)
	if enabled := reloadProjectSidebarEnabled(t, app, project.ID); enabled {
		t.Fatal("sidebar state reverted to enabled after peer saves")
	}

	// Hidden -> visible uses the normal keymap path. Re-enable from the temporary
	// Outliner and prove that the same shared preference survives another reload.
	_, cmd = temporary.Update(syntheticSpecialKey(tea.KeyTab))
	msg = requireProjectSidebarStateMsg(t, cmd)
	if msg.ProjectID != project.ID || !msg.Enabled {
		t.Fatalf("sidebar message = %+v, want project %q enabled", msg, project.ID)
	}
	app.handleGlobalSaveMessage(msg)
	assertSidebarState(t, "tree", tree.sidebar, true, false)
	assertSidebarState(t, "outliner", outliner.sidebar, true, false)
	assertSidebarState(t, "temporary outliner", temporary.sidebar, true, false)
	if enabled := reloadProjectSidebarEnabled(t, app, project.ID); !enabled {
		t.Fatal("reloaded sidebar state = disabled, want enabled")
	}
}

func TestOutlinerStateSaveIncludesSidebarVisibility(t *testing.T) {
	app, project := newMachineLayoutTestApp(t)
	outliner := NewOutlinerModel(project, app.configManager, app.runtime)

	outliner.sidebar.SetEnabled(false)
	app.saveOutlinerFilterStateFrom(outliner)
	if enabled := reloadProjectSidebarEnabled(t, app, project.ID); enabled {
		t.Fatal("saved outliner sidebar state = enabled, want disabled")
	}

	outliner.sidebar.SetEnabled(true)
	app.saveOutlinerFilterStateFrom(outliner)
	if enabled := reloadProjectSidebarEnabled(t, app, project.ID); !enabled {
		t.Fatal("saved outliner sidebar state = disabled, want enabled")
	}
}

func TestSidebarFocusOnlyCycleDoesNotEmitPersistenceMessage(t *testing.T) {
	app, project := newMachineLayoutTestApp(t)
	app.setProjectSidebarEnabled(project.ID, true)

	tree := NewProjectTreeModel(project, app.configManager, app.runtime)
	if _, cmd := tree.Update(syntheticSpecialKey(tea.KeyTab)); cmd != nil {
		t.Fatal("project tree focus-only sidebar cycle emitted a command")
	}
	assertSidebarState(t, "tree", tree.sidebar, true, true)
	_, treeHideCmd := tree.Update(syntheticSpecialKey(tea.KeyTab))
	treeHideMsg := requireProjectSidebarStateMsg(t, treeHideCmd)
	if treeHideMsg.ProjectID != project.ID || treeHideMsg.Enabled {
		t.Fatalf("project tree hide message = %+v, want project %q disabled", treeHideMsg, project.ID)
	}
	_, treeShowCmd := tree.Update(syntheticSpecialKey(tea.KeyTab))
	treeShowMsg := requireProjectSidebarStateMsg(t, treeShowCmd)
	if treeShowMsg.ProjectID != project.ID || !treeShowMsg.Enabled {
		t.Fatalf("project tree show message = %+v, want project %q enabled", treeShowMsg, project.ID)
	}

	outliner := NewOutlinerModel(project, app.configManager, app.runtime)
	if _, cmd := outliner.Update(syntheticSpecialKey(tea.KeyTab)); cmd != nil {
		t.Fatal("outliner focus-only sidebar cycle emitted a command")
	}
	assertSidebarState(t, "outliner", outliner.sidebar, true, true)
	if enabled := reloadProjectSidebarEnabled(t, app, project.ID); !enabled {
		t.Fatal("focus-only cycles changed persisted sidebar visibility")
	}

	// Cross-project Outliner visibility remains governed by its separate global
	// preference and never emits the per-project message.
	crossOutliner := NewOutlinerModel(nil, app.configManager, app.runtime)
	crossOutliner.sidebar.SetEnabled(false)
	if _, cmd := crossOutliner.Update(syntheticSpecialKey(tea.KeyTab)); cmd != nil {
		t.Fatal("cross-project outliner emitted a per-project sidebar command")
	}
}

func TestProjectTreeSidebarTabHandlesNilProject(t *testing.T) {
	app, project := newMachineLayoutTestApp(t)
	tree := NewProjectTreeModel(project, app.configManager, app.runtime)
	tree.project = nil
	tree.nodes = nil
	tree.sidebar.SetEnabled(false)

	wantStates := []struct {
		enabled bool
		focused bool
	}{
		{enabled: true, focused: false},
		{enabled: true, focused: true},
		{enabled: false, focused: false},
	}

	for i, want := range wantStates {
		if _, cmd := tree.Update(syntheticSpecialKey(tea.KeyTab)); cmd != nil {
			t.Fatalf("tab cycle %d emitted a per-project command for a nil project", i+1)
		}
		assertSidebarState(t, "nil-project tree", tree.sidebar, want.enabled, want.focused)
	}
}

func mustReloadRuntime(t *testing.T, dataDir string) *config.RuntimeState {
	t.Helper()
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	return runtime
}
