package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
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
		NoteService:    noteService,
		HistoryLedger:  historyLedger,
		Wire:           m.wireSecondaryCoordinators,
		ResolveProject: m.activeProjectByID,
		ActiveProjectFocus: func() string {
			if m.crossOutliner == nil {
				return ""
			}
			return m.crossOutliner.scopedProjectID
		},
		TaskCounts:        m.projectHubTaskCounts,
		OperationRegistry: m.operationRegistry,
		OperationExecutor: m.operationExecutor,
		DispatchKey:       m.dispatchOperationKey,
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
	m.manageHub.SetSize(m.width, m.height-2)
	return m.manageHub
}

// activateManageHub installs one scoped collection mode as a mode of the typed
// Manage destination. No embedded legacy model enters route
// history; PresentationID remains only a presentation observation for commands and
// existing overlay return messages.
func (m *AppModel) activateManageHub(params ManageParams) tea.Cmd {
	route, err := NewRoute(SecondaryTool, params)
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	params = route.Params().(ManageParams)

	hub := m.ensureManageHub()
	missingProject := params.Module == ManageNotes &&
		params.NotesScope.Kind == NotesScopeProject &&
		m.activeProjectByID(params.NotesScope.ProjectID) == nil
	if missingProject {
		params.NotesScope = AllNotesScope()
		route, err = NewRoute(SecondaryTool, params)
		if err != nil {
			return func() tea.Msg { return ErrorMsg{Err: err} }
		}
		params = route.Params().(ManageParams)
	}
	if activateErr := hub.Activate(params); activateErr != nil {
		return func() tea.Msg { return ErrorMsg{Err: activateErr} }
	}
	if missingProject {
		m.setInfoStatus("Project is unavailable — showing All Notes")
	}

	active := hub.ActiveModuleView()
	if active != nil {
		active.SetSize(m.width, m.height-2)
		m.wireSecondaryCoordinators(active)
		m.registerManageModulePresentation(params, active)
	}

	if m.navigation.Primary.Current().ID() == SecondaryTool {
		m.navigation.Overlays.CancelAll()
		err = m.navigation.Primary.ReplaceCurrentMode(route)
	} else {
		err = m.navigation.PushChild(route)
	}
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	m.setAccentOnView(hub)
	if active != nil {
		m.setAccentOnView(active)
		return active.Init()
	}
	return nil
}

func (m *AppModel) registerManageModulePresentation(params ManageParams, view View) {
	if m.views == nil || view == nil {
		return
	}
	presentation, ok := manageModulePresentation(params)
	if !ok {
		return
	}
	m.views.Register(presentation, view)
}

func (m *AppModel) handleManageHubMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ManageHubModuleSelectedMsg:
		return true, m.activateManageHub(msg.Params)
	default:
		return false, nil
	}
}

func (m *AppModel) handleInvalidManageProjectScope() (bool, tea.Cmd) {
	_, params, ok := m.manageHubPrimary()
	if !ok || params.Module != ManageNotes ||
		params.NotesScope.Kind != NotesScopeProject ||
		m.activeProjectByID(params.NotesScope.ProjectID) != nil {
		return false, nil
	}
	if m.currentProject != nil && m.currentProject.ID == params.NotesScope.ProjectID {
		m.currentProject = nil
	}
	if notes := m.getActiveNotesView(); notes != nil {
		notes.RefreshWorkspace(m.availableProjects()...)
	}
	params.NotesScope = AllNotesScope()
	m.setInfoStatus("Project was removed — showing All Notes")
	return true, m.activateManageHub(params)
}

// refreshActiveNotesManager rebuilds the live owner catalog after a workspace
// lifecycle change. If the current project scope disappeared, both the model
// and the canonical typed route are replaced immediately instead of waiting for
// the next keypress.
func (m *AppModel) refreshActiveNotesManager() tea.Cmd {
	notes := m.getActiveNotesView()
	if notes == nil {
		return nil
	}
	if !notes.RefreshWorkspace(m.availableProjects()...) {
		return nil
	}
	if m.manageHub == nil {
		notes.setErrorStatus("Project is unavailable; reopen Notes to use All Notes")
		return nil
	}

	if m.currentProject != nil && m.currentProject.ID == m.manageHub.params.NotesScope.ProjectID {
		m.currentProject = nil
	}
	params := m.manageHub.params
	params.NotesScope = AllNotesScope()
	message := "Project is unavailable — showing All Notes"
	m.setInfoStatus(message)
	cmd := m.activateManageHub(params)
	if active := m.getActiveNotesView(); active != nil {
		active.setStatus(message)
	}
	return cmd
}
