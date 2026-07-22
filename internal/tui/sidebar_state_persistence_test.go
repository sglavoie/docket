package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

const (
	enableAllSidebarsCommand  = "enable sidebar for all projects"
	disableAllSidebarsCommand = "disable sidebar for all projects"
)

func newPersistentSidebarTestApp(t *testing.T, projects ...*models.Project) *AppModel {
	t.Helper()
	dataDir := t.TempDir()
	seed := newTestConfigAt(dataDir)
	setTestProjects(seed, projects)
	runtime := testRuntimeAt(dataDir, seed)
	return &AppModel{
		configManager:        config.NewManager(filepath.Join(dataDir, ".tt.json")),
		runtime:              runtime,
		projects:             make(map[string]*models.Project),
		views:                NewViewRegistry(),
		viewStack:            []PresentationID{ViewDashboard},
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
		projectLastView:      make(map[string]PresentationID),
		width:                80,
		height:               24,
	}
}

func requireSidebarStateMsg(t *testing.T, cmd tea.Cmd) projectSidebarStateChangedMsg {
	t.Helper()
	for _, msg := range drainTUITestCmd(t, cmd) {
		if stateMsg, ok := msg.(projectSidebarStateChangedMsg); ok {
			return stateMsg
		}
	}
	t.Fatal("command did not emit projectSidebarStateChangedMsg")
	return projectSidebarStateChangedMsg{}
}

func assertSidebar(t *testing.T, name string, sidebar *SidebarComponent, enabled, focused bool) {
	t.Helper()
	if sidebar.IsEnabled() != enabled || sidebar.IsFocused() != focused {
		t.Fatalf("%s sidebar = (enabled=%v focused=%v), want (enabled=%v focused=%v)",
			name, sidebar.IsEnabled(), sidebar.IsFocused(), enabled, focused)
	}
}

func TestProjectSidebarTogglePersistsAcrossRecreationAndReload(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Project")
	app := newPersistentSidebarTestApp(t, project)
	app.currentProject = project
	outliner := NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner = outliner
	app.views.Register(ViewOutliner, outliner)
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner}

	// Visible/tree -> visible/sidebar changes focus only and must not persist.
	_, focusCmd := outliner.Update(syntheticSpecialKey(tea.KeyTab))
	if focusCmd != nil {
		t.Fatal("focus-only sidebar cycle emitted a persistence command")
	}
	// Visible/sidebar -> hidden emits the per-project preference message.
	_, hideCmd := outliner.Update(syntheticSpecialKey(tea.KeyTab))
	hideMsg := requireSidebarStateMsg(t, hideCmd)
	if hideMsg.ProjectID != project.ID || hideMsg.Enabled {
		t.Fatalf("hide message = %+v, want project %q disabled", hideMsg, project.ID)
	}
	if handled, _ := app.handleGlobalSaveMessage(hideMsg); !handled {
		t.Fatal("sidebar state message was not handled")
	}

	assertSidebar(t, "live", outliner.sidebar, false, false)
	if fresh := NewOutlinerModel(project, app.configManager, app.runtime); fresh.sidebar.IsEnabled() {
		t.Fatal("fresh outliner sidebar = enabled, want restored disabled state")
	}
	reloaded, err := config.LoadRuntimeState(app.runtime.DataDir())
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	if reloaded.Machine().GetProjectSidebarEnabled(project.ID) {
		t.Fatal("reloaded machine sidebar state = enabled, want disabled")
	}
	if fresh := NewOutlinerModel(project, app.configManager, reloaded); fresh.sidebar.IsEnabled() {
		t.Fatal("fresh reloaded outliner sidebar = enabled, want disabled")
	}

	// Hidden -> visible persists the inverse preference.
	_, showCmd := outliner.Update(syntheticSpecialKey(tea.KeyTab))
	showMsg := requireSidebarStateMsg(t, showCmd)
	if !showMsg.Enabled {
		t.Fatalf("show message = %+v, want enabled", showMsg)
	}
	app.handleGlobalSaveMessage(showMsg)
	reloaded, err = config.LoadRuntimeState(app.runtime.DataDir())
	if err != nil {
		t.Fatalf("reload enabled runtime: %v", err)
	}
	if !reloaded.Machine().GetProjectSidebarEnabled(project.ID) {
		t.Fatal("reloaded machine sidebar state = disabled, want enabled")
	}
}

func TestBulkSidebarCommandsPersistAllActiveProjectsAndSyncLiveViews(t *testing.T) {
	alpha := testutil.NewTestProject("Alpha")
	beta := testutil.NewTestProject("Beta")
	gamma := testutil.NewTestProject("Gamma")
	archived := testutil.NewTestProject("Archived")
	archived.Archived = true
	sandbox := testutil.NewTestProject("Sandbox")
	sandbox.Sandbox = true
	internal := testutil.NewTestProject("Internal")
	internal.Internal = true
	app := newPersistentSidebarTestApp(t, alpha, beta, gamma, archived, sandbox, internal)
	app.currentProject = alpha
	machine := cloneMachineState(app.runtime.Machine())
	machine.CurrentProject = alpha.ID
	app.runtime.SetMachineInMemory(machine)

	primary := NewOutlinerModel(alpha, app.configManager, app.runtime)
	primary.sidebar.Focus()
	app.outliner = primary
	app.views.Register(ViewOutliner, primary)
	temporary := NewOutlinerModel(beta, app.configManager, app.runtime)
	temporary.sidebar.Focus()
	app.views.Register(ViewTemporaryOutliner, temporary)
	preview := NewOutlinerModel(gamma, app.configManager, app.runtime)
	preview.sidebar.Focus()
	app.pinnedPreview = preview
	cross := NewOutlinerModel(nil, app.configManager, app.runtime)
	cross.sidebar.Focus()

	disable := findCommand(app.sidebarCommands(), disableAllSidebarsCommand)
	if disable == nil || !disable.Available() {
		t.Fatal("disable-all sidebar command should be available initially")
	}
	if msg := disable.Handler(); msg != nil {
		t.Fatalf("disable handler returned %T, want nil", msg)
	}

	for _, project := range []*models.Project{alpha, beta, gamma} {
		if app.runtime.Machine().GetProjectSidebarEnabled(project.ID) {
			t.Errorf("active project %q sidebar preference = enabled, want disabled", project.Name)
		}
	}
	for _, project := range []*models.Project{archived, sandbox, internal} {
		if !app.runtime.Machine().GetProjectSidebarEnabled(project.ID) {
			t.Errorf("excluded project %q sidebar preference changed", project.Name)
		}
	}
	assertSidebar(t, "primary", primary.sidebar, false, false)
	assertSidebar(t, "temporary", temporary.sidebar, false, false)
	assertSidebar(t, "preview", preview.sidebar, false, false)
	assertSidebar(t, "cross-project", cross.sidebar, true, true)

	reloaded, err := config.LoadRuntimeState(app.runtime.DataDir())
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	for _, project := range []*models.Project{alpha, beta, gamma} {
		if reloaded.Machine().GetProjectSidebarEnabled(project.ID) {
			t.Errorf("reloaded active project %q sidebar preference = enabled, want disabled", project.Name)
		}
	}
	if fresh := NewOutlinerModel(beta, app.configManager, reloaded); fresh.sidebar.IsEnabled() {
		t.Fatal("next project sidebar = enabled after disable-all, want disabled")
	}

	enable := findCommand(app.sidebarCommands(), enableAllSidebarsCommand)
	if enable == nil || !enable.Available() {
		t.Fatal("enable-all sidebar command should be available after disabling")
	}
	enable.Handler()
	for _, project := range []*models.Project{alpha, beta, gamma} {
		if !app.runtime.Machine().GetProjectSidebarEnabled(project.ID) {
			t.Errorf("active project %q sidebar preference = disabled after enable-all", project.Name)
		}
	}
}

func TestSidebarCommandsPreferLiveStateThenRememberedProject(t *testing.T) {
	alpha := testutil.NewTestProject("Alpha")
	beta := testutil.NewTestProject("Beta")
	app := newPersistentSidebarTestApp(t, alpha, beta)
	machine := cloneMachineState(app.runtime.Machine())
	machine.CurrentProject = beta.ID
	machine.SetProjectSidebarEnabled(alpha.ID, false)
	machine.SetProjectSidebarEnabled(beta.ID, false)
	app.runtime.SetMachineInMemory(machine)

	enable := findCommand(app.sidebarCommands(), enableAllSidebarsCommand)
	disable := findCommand(app.sidebarCommands(), disableAllSidebarsCommand)
	if enable == nil || disable == nil {
		t.Fatal("sidebar commands were not registered")
	}
	if !enable.Available() || disable.Available() {
		t.Fatal("dashboard command availability did not use remembered current-project state")
	}

	live := NewOutlinerModel(alpha, app.configManager, app.runtime)
	live.sidebar.SetEnabled(true)
	app.outliner = live
	app.views.Register(ViewOutliner, live)
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}
	if enable.Available() || !disable.Available() {
		t.Fatal("palette command availability did not prefer the live project sidebar state")
	}
}

func TestCloneMachineStateDoesNotAliasProjectSidebarPreferences(t *testing.T) {
	machine := config.NewMachineState()
	machine.SetProjectSidebarEnabled("project", false)
	cloned := cloneMachineState(machine)
	cloned.SetProjectSidebarEnabled("project", true)
	if machine.GetProjectSidebarEnabled("project") {
		t.Fatal("cloned machine sidebar map aliases the original")
	}
}
