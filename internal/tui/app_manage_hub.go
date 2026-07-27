package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *AppModel) ensureManageHub() *ManageHubModel {
	runtime := m.runtime
	if runtime == nil {
		runtime = config.EmptyRuntimeState("")
	}
	var historyLedger *storage.WorkspaceLedger
	if m.deps != nil {
		historyLedger = m.deps.historyLedger()
	}
	var noteService projectNoteService
	if m.appService != nil {
		noteService = m.appService
	}
	deps := ManageHubDependencies{
		Runtime:        runtime,
		ConfigManager:  m.configManager,
		AppService:     m.appService,
		NoteService:    noteService,
		HistoryLedger:  historyLedger,
		Blockers:       m.secondaryFeatures.Blocker,
		Tags:           m.secondaryFeatures.Tag,
		Wire:           m.wireSecondaryCoordinators,
		ResolveProject: m.projectByID,
		CurrentProject: func() *models.Project { return m.currentProject },
		NewSettings:    func() *SettingsModel { return NewSettingsModel(m.deps) },
		ActiveContext: func() ContextScope {
			if m.crossOutliner == nil {
				return AllContextScope()
			}
			return m.crossOutliner.activeContext
		},
		ActiveProjectFocus: func() string {
			if m.crossOutliner == nil {
				return ""
			}
			return m.crossOutliner.scopedProjectID
		},
		LoadedProjects: func() []*models.Project {
			if m.crossOutliner == nil {
				return nil
			}
			return m.crossOutliner.loadedProjects
		},
		OperationRegistry: m.operationRegistry,
		OperationExecutor: m.operationExecutor,
	}
	if m.manageHub == nil {
		m.manageHub = NewManageHubModel(deps)
	} else {
		// Test and embedding compositions may replace the optional service or
		// ledger after construction. Keep the hub on the shell's current seams
		// rather than retaining a stale history handle.
		m.manageHub.deps = deps
		if m.manageHub.params.Module == ManageNotes {
			m.manageHub.activeKey = ""
		}
	}
	m.manageHub.SetNotificationData(m.proactiveAlerts, m.rawAlerts, m.dismissedSet)
	m.manageHub.SetMessageLogEntries(m.messageLog.Entries())
	m.manageHub.SetSize(m.width, m.height-2)
	return m.manageHub
}

// activateManageHub installs one scoped collection or notification mode as a
// mode of the typed Manage destination. No embedded legacy model enters route
// history; PresentationID remains only a presentation observation for commands and
// existing overlay return messages.
func (m *AppModel) activateManageHub(params ManageParams) tea.Cmd {
	route, err := NewRoute(SecondaryTool, params)
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	params = route.Params().(ManageParams)

	hub := m.ensureManageHub()
	missingProject := params.Scope.Kind == ScopeProject && m.projectByID(params.Scope.ProjectID) == nil
	if missingProject {
		params.Scope = AppEntityScope()
		route, err = NewRoute(SecondaryTool, params)
		if err != nil {
			return func() tea.Msg { return ErrorMsg{Err: err} }
		}
	}
	if err := hub.Activate(params); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	projectPickerOpen := false
	if missingProject {
		projectPickerOpen = hub.OpenProjectPicker(params.Module)
		if !projectPickerOpen {
			m.setInfoStatus("No projects available")
		}
	}

	active := hub.ActiveModuleView()
	if active != nil {
		active.SetSize(m.width, m.height-2)
		m.wireSecondaryCoordinators(active)
		m.registerManageModulePresentation(params, active)
	}

	if err := m.navigation.PushChild(route, m.currentFocusTarget()); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	if m.startupRoute != nil && m.startupRoute.ID() == SecondaryTool {
		m.startupRoute = nil
		m.startupView = ViewCrossProjectOutliner
	}
	if projectPickerOpen {
		m.pushTypedOverlay(OverlayViewMenu, manageHubViewMenuOverlayKey)
	}
	m.setAccentOnView(hub)
	if active != nil {
		m.setAccentOnView(active)
		return active.Init()
	}
	return nil
}

func (m *AppModel) activateManageRules(mode ManageRulesMode) tea.Cmd {
	hub := m.ensureManageHub()
	hub.SetRulesMode(mode)
	return m.activateManageHub(ManageParams{Module: ManageRules, Scope: AppEntityScope()})
}

func (m *AppModel) activateManageProjectPicker(module ManageModule) tea.Cmd {
	return m.activateManageHub(ManageParams{Module: module, Scope: ProjectEntityScope("missing-project")})
}

func (m *AppModel) registerManageModulePresentation(params ManageParams, view View) {
	if m.views == nil || view == nil {
		return
	}
	presentation, ok := manageModulePresentation(params)
	if params.Module == ManageRules && m.manageHub != nil {
		presentation, ok = m.manageHub.PresentationView()
	}
	if !ok {
		return
	}
	m.views.Register(presentation, view)
}

func (m *AppModel) handleManageHubMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowManageHubMsg:
		return true, m.activateManageHub(msg.Params)
	case ManageHubModuleSelectedMsg:
		m.closeTypedOverlay(OverlayViewMenu, manageHubViewMenuOverlayKey)
		if msg.HasRulesMode {
			m.ensureManageHub().SetRulesMode(msg.RulesMode)
		}
		return true, m.activateManageHub(msg.Params)
	case manageHubViewMenuMsg:
		if msg.Open {
			m.pushTypedOverlay(OverlayViewMenu, manageHubViewMenuOverlayKey)
		} else {
			m.closeTypedOverlay(OverlayViewMenu, manageHubViewMenuOverlayKey)
		}
		return true, nil
	default:
		return false, nil
	}
}

func (m *AppModel) refreshManageNotificationCenter() {
	if m.manageHub == nil {
		return
	}
	m.manageHub.SetNotificationData(m.proactiveAlerts, m.rawAlerts, m.dismissedSet)
	m.manageHub.RefreshNotificationCenter()
	params := m.manageHub.Params()
	if active := m.manageHub.ActiveModuleView(); active != nil {
		m.registerManageModulePresentation(params, active)
	}
}

func (m *AppModel) refreshManageMessageLog() {
	if m.manageHub == nil {
		return
	}
	m.manageHub.SetMessageLogEntries(m.messageLog.Entries())
	params := m.manageHub.Params()
	if params.Module == ManageNotifications && params.NotificationMode == NotificationLog {
		if active := m.manageHub.ActiveModuleView(); active != nil {
			m.registerManageModulePresentation(params, active)
		}
	}
}

func (m *AppModel) handleInvalidManageProjectScope() (bool, tea.Cmd) {
	_, params, ok := m.manageHubPrimary()
	if !ok || params.Scope.Kind != ScopeProject || m.projectByID(params.Scope.ProjectID) != nil {
		return false, nil
	}
	if m.currentProject != nil && m.currentProject.ID == params.Scope.ProjectID {
		m.currentProject = nil
	}
	params.Scope = AppEntityScope()
	m.setInfoStatus("Project was removed — showing all " + params.Module.String())
	return true, m.activateManageHub(params)
}
