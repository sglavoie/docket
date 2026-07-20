package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

const (
	enableAllSidebarsCommand  = "enable sidebar for all projects"
	disableAllSidebarsCommand = "disable sidebar for all projects"
)

func availableBulkSidebarCommand(t *testing.T, registry *CommandRegistry, view ViewType) *Command {
	t.Helper()
	commands := registry.GetForView(view)
	enable := findCommand(commands, enableAllSidebarsCommand)
	disable := findCommand(commands, disableAllSidebarsCommand)
	if (enable == nil) == (disable == nil) {
		t.Fatalf("bulk sidebar commands = enable:%v disable:%v, want exactly one", enable != nil, disable != nil)
	}
	if enable != nil {
		return enable
	}
	return disable
}

func registerSidebarCommands(t *testing.T, app *AppModel) *CommandRegistry {
	t.Helper()
	registry := NewCommandRegistry()
	if err := registry.RegisterBatch(app.sidebarCommands()); err != nil {
		t.Fatalf("RegisterBatch: %v", err)
	}
	return registry
}

func TestSidebarCommandsUseRememberedAndFallbackProjectState(t *testing.T) {
	app := newTestAppModel()
	alpha := testutil.NewTestProject("Alpha")
	beta := testutil.NewTestProject("Beta")
	archived := testutil.NewTestProject("Archived")
	archived.Archived = true
	sandbox := testutil.NewTestProject("Sandbox")
	sandbox.Sandbox = true
	internal := testutil.NewTestProject("Internal")
	internal.Internal = true
	setTestAppProjects(app, alpha, beta, archived, sandbox, internal)

	machine := cloneMachineState(app.runtime.Machine())
	machine.SetProjectViewState(alpha.ID, config.ProjectViewState{SidebarEnabled: true})
	machine.SetProjectViewState(beta.ID, config.ProjectViewState{SidebarEnabled: false})
	machine.SetProjectViewState(archived.ID, config.ProjectViewState{SidebarEnabled: false})
	machine.SetProjectViewState(sandbox.ID, config.ProjectViewState{SidebarEnabled: false})
	machine.SetProjectViewState(internal.ID, config.ProjectViewState{SidebarEnabled: false})
	machine.CurrentProject = beta.ID
	app.runtime.SetMachineInMemory(machine)
	app.currentProject = nil
	registry := registerSidebarCommands(t, app)

	if cmd := availableBulkSidebarCommand(t, registry, ViewDashboard); cmd.Name != enableAllSidebarsCommand {
		t.Fatalf("remembered disabled active project exposed %q, want %q", cmd.Name, enableAllSidebarsCommand)
	}

	for _, project := range []*models.Project{archived, sandbox, internal} {
		t.Run("excluded_"+project.Slug, func(t *testing.T) {
			machine := cloneMachineState(app.runtime.Machine())
			machine.CurrentProject = project.ID
			app.runtime.SetMachineInMemory(machine)
			if cmd := availableBulkSidebarCommand(t, registry, ViewDashboard); cmd.Name != disableAllSidebarsCommand {
				t.Fatalf("remembered excluded project exposed %q, want %q from first active project", cmd.Name, disableAllSidebarsCommand)
			}
		})
	}

	machine = cloneMachineState(app.runtime.Machine())
	machine.CurrentProject = ""
	firstActive := app.runtime.ActiveProjects()[0]
	machine.SetProjectViewState(firstActive.ID, config.ProjectViewState{SidebarEnabled: true})
	app.runtime.SetMachineInMemory(machine)
	if cmd := availableBulkSidebarCommand(t, registry, ViewDashboard); cmd.Name != disableAllSidebarsCommand {
		t.Fatalf("enabled first active project exposed %q, want %q", cmd.Name, disableAllSidebarsCommand)
	}

	setTestAppProjects(app)
	machine = cloneMachineState(app.runtime.Machine())
	machine.CurrentProject = ""
	app.runtime.SetMachineInMemory(machine)
	if cmd := availableBulkSidebarCommand(t, registry, ViewDashboard); cmd.Name != disableAllSidebarsCommand {
		t.Fatalf("default enabled state exposed %q, want %q", cmd.Name, disableAllSidebarsCommand)
	}
}

func TestSidebarCommandsPreferLiveSingleProjectViewState(t *testing.T) {
	app, project := newTestAppModelWithProject()
	machine := cloneMachineState(app.runtime.Machine())
	machine.SetProjectViewState(project.ID, config.ProjectViewState{SidebarEnabled: false})
	app.runtime.SetMachineInMemory(machine)
	registry := registerSidebarCommands(t, app)

	outliner := NewOutlinerModel(project, app.configManager, app.runtime)
	outliner.sidebar.SetEnabled(false)
	app.outliner = outliner
	app.views.Register(ViewOutliner, outliner)
	app.viewStack = []ViewType{ViewDashboard, ViewOutliner}
	if cmd := availableBulkSidebarCommand(t, registry, ViewOutliner); cmd.Name != enableAllSidebarsCommand {
		t.Fatalf("live disabled Outliner exposed %q, want %q", cmd.Name, enableAllSidebarsCommand)
	}

	temporary := NewOutlinerModel(project, app.configManager, app.runtime)
	temporary.sidebar.SetEnabled(true)
	app.views.Register(ViewTemporaryOutliner, temporary)
	app.viewStack = []ViewType{ViewDashboard, ViewTemporaryOutliner}
	if cmd := availableBulkSidebarCommand(t, registry, ViewTemporaryOutliner); cmd.Name != disableAllSidebarsCommand {
		t.Fatalf("live enabled temporary Outliner exposed %q, want %q", cmd.Name, disableAllSidebarsCommand)
	}
}

func TestBulkSidebarCommandsUpdateOnlyActiveProjectsAndLivePeers(t *testing.T) {
	app := newTestAppModel()
	alpha := testutil.NewTestProject("Alpha")
	beta := testutil.NewTestProject("Beta")
	gamma := testutil.NewTestProject("Gamma")
	archived := testutil.NewTestProject("Archived")
	archived.Archived = true
	sandbox := testutil.NewTestProject("Sandbox")
	sandbox.Sandbox = true
	internal := testutil.NewTestProject("Internal")
	internal.Internal = true
	setTestAppProjects(app, alpha, beta, gamma, archived, sandbox, internal)

	crossEnabled := false
	machine := cloneMachineState(app.runtime.Machine())
	machine.CurrentProject = alpha.ID
	machine.CrossOutlinerSidebarEnabled = &crossEnabled
	machine.SetProjectViewState(alpha.ID, config.ProjectViewState{SidebarEnabled: true, FilterType: "alpha-filter"})
	machine.SetProjectViewState(beta.ID, config.ProjectViewState{SidebarEnabled: false, PinnedTagTab: "beta-tag"})
	machine.SetProjectViewState(gamma.ID, config.ProjectViewState{SidebarEnabled: true, OutlinerFilterStage: "gamma-stage"})
	machine.SetProjectViewState(archived.ID, config.ProjectViewState{SidebarEnabled: true, FilterStatus: "archived-status"})
	machine.SetProjectViewState(sandbox.ID, config.ProjectViewState{SidebarEnabled: false, FilterPriority: 4})
	machine.SetProjectViewState(internal.ID, config.ProjectViewState{SidebarEnabled: true, ViewMode: 3})
	app.runtime.SetMachineInMemory(machine)
	registry := registerSidebarCommands(t, app)

	outliner := NewOutlinerModel(beta, app.configManager, app.runtime)
	outliner.sidebar.Focus()
	app.outliner = outliner
	app.views.Register(ViewOutliner, outliner)
	temporary := NewOutlinerModel(gamma, app.configManager, app.runtime)
	temporary.sidebar.Focus()
	app.views.Register(ViewTemporaryOutliner, temporary)
	cross := NewOutlinerModel(nil, app.configManager, app.runtime)
	cross.sidebar.SetEnabled(false)
	cross.sidebar.Focus()
	app.crossOutliner = cross
	app.views.Register(ViewCrossProjectOutliner, cross)

	disable := availableBulkSidebarCommand(t, registry, ViewDashboard)
	if disable.Name != disableAllSidebarsCommand {
		t.Fatalf("initial command = %q, want %q", disable.Name, disableAllSidebarsCommand)
	}
	if msg := disable.Handler(); msg != nil {
		t.Fatalf("disable handler returned %T, want nil", msg)
	}

	assertProjectState := func(projectID string, enabled bool, checkUnrelated func(config.ProjectViewState) bool) {
		t.Helper()
		state, ok := app.runtime.Machine().GetProjectViewState(projectID)
		if !ok {
			t.Fatalf("project %q has no view state", projectID)
		}
		if state.SidebarEnabled != enabled {
			t.Errorf("project %q SidebarEnabled = %v, want %v", projectID, state.SidebarEnabled, enabled)
		}
		if !checkUnrelated(state) {
			t.Errorf("project %q unrelated view state was changed: %+v", projectID, state)
		}
	}
	assertProjectState(alpha.ID, false, func(state config.ProjectViewState) bool { return state.FilterType == "alpha-filter" })
	assertProjectState(beta.ID, false, func(state config.ProjectViewState) bool { return state.PinnedTagTab == "beta-tag" })
	assertProjectState(gamma.ID, false, func(state config.ProjectViewState) bool { return state.OutlinerFilterStage == "gamma-stage" })
	assertProjectState(archived.ID, true, func(state config.ProjectViewState) bool { return state.FilterStatus == "archived-status" })
	assertProjectState(sandbox.ID, false, func(state config.ProjectViewState) bool { return state.FilterPriority == 4 })
	assertProjectState(internal.ID, true, func(state config.ProjectViewState) bool { return state.ViewMode == 3 })
	for name, sidebar := range map[string]*SidebarComponent{
		"outliner": outliner.sidebar, "temporary outliner": temporary.sidebar,
	} {
		if sidebar.IsEnabled() || sidebar.IsFocused() {
			t.Errorf("%s sidebar = enabled:%v focused:%v, want disabled and blurred", name, sidebar.IsEnabled(), sidebar.IsFocused())
		}
	}
	if cross.sidebar.IsEnabled() || !cross.sidebar.IsFocused() {
		t.Errorf("cross-project sidebar changed to enabled:%v focused:%v", cross.sidebar.IsEnabled(), cross.sidebar.IsFocused())
	}
	if got := app.runtime.Machine().CrossOutlinerSidebarEnabled; got == nil || *got {
		t.Fatalf("CrossOutlinerSidebarEnabled = %v, want unchanged false", got)
	}

	enable := availableBulkSidebarCommand(t, registry, ViewDashboard)
	if enable.Name != enableAllSidebarsCommand {
		t.Fatalf("command after disable = %q, want %q", enable.Name, enableAllSidebarsCommand)
	}
	if msg := enable.Handler(); msg != nil {
		t.Fatalf("enable handler returned %T, want nil", msg)
	}
	assertProjectState(alpha.ID, true, func(state config.ProjectViewState) bool { return state.FilterType == "alpha-filter" })
	assertProjectState(beta.ID, true, func(state config.ProjectViewState) bool { return state.PinnedTagTab == "beta-tag" })
	assertProjectState(gamma.ID, true, func(state config.ProjectViewState) bool { return state.OutlinerFilterStage == "gamma-stage" })
	assertProjectState(archived.ID, true, func(state config.ProjectViewState) bool { return state.FilterStatus == "archived-status" })
	assertProjectState(sandbox.ID, false, func(state config.ProjectViewState) bool { return state.FilterPriority == 4 })
	assertProjectState(internal.ID, true, func(state config.ProjectViewState) bool { return state.ViewMode == 3 })
	for name, sidebar := range map[string]*SidebarComponent{
		"outliner": outliner.sidebar, "temporary outliner": temporary.sidebar,
	} {
		if !sidebar.IsEnabled() {
			t.Errorf("%s sidebar remained disabled after enable", name)
		}
	}
	if got := app.runtime.Machine().CrossOutlinerSidebarEnabled; got == nil || *got {
		t.Fatalf("CrossOutlinerSidebarEnabled after enable = %v, want unchanged false", got)
	}
}
