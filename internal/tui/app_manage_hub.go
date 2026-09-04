package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

func (m *AppModel) ensureManageHub() *projecthub.ManageHubModel {
	runtime := m.runtime
	if runtime == nil {
		runtime = config.EmptyRuntimeState("")
	}
	var historyLedger *storage.WorkspaceLedger
	if m.deps != nil {
		historyLedger = m.deps.historyLedger()
	}
	var noteService notes.ProjectNoteService
	if m.appService != nil {
		noteService = m.appService
	}
	deps := projecthub.ManageHubDependencies{
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
		TaskCounts:         m.projectHubTaskCounts,
		OperationRegistry:  m.operationRegistry,
		OperationExecutor:  m.operationExecutor,
		DispatchKey:        m.dispatchOperationKey,
		PreviewTaskPainter: newProjectPreviewPainter,
	}
	if m.manageHub == nil {
		m.manageHub = projecthub.NewManageHubModel(deps)
	} else {
		m.manageHub.Rewire(deps)
	}
	m.manageHub.SetSize(m.width, m.height-2)
	return m.manageHub
}

// activateManageHub installs one scoped collection mode as a mode of the typed
// Manage destination. No embedded legacy model enters route
// history; PresentationID remains only a presentation observation for commands and
// existing overlay return messages.
func (m *AppModel) activateManageHub(params route.ManageParams) tea.Cmd {
	target, err := route.NewRoute(route.SecondaryTool, params)
	if err != nil {
		return func() tea.Msg { return status.ErrorMsg{Err: err} }
	}
	params = target.Params().(route.ManageParams)

	hub := m.ensureManageHub()
	missingProject := params.Module == route.ManageNotes &&
		params.NotesScope.Kind == route.NotesScopeProject &&
		m.activeProjectByID(params.NotesScope.ProjectID) == nil
	if missingProject {
		params.NotesScope = route.AllNotesScope()
		target, err = route.NewRoute(route.SecondaryTool, params)
		if err != nil {
			return func() tea.Msg { return status.ErrorMsg{Err: err} }
		}
		params = target.Params().(route.ManageParams)
	}
	if activateErr := hub.Activate(params); activateErr != nil {
		return func() tea.Msg { return status.ErrorMsg{Err: activateErr} }
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

	if m.navigation.Primary.Current().ID() == route.SecondaryTool {
		m.navigation.Overlays.CancelAll()
		err = m.navigation.Primary.ReplaceCurrentMode(target)
	} else {
		err = m.navigation.PushChild(target)
	}
	if err != nil {
		return func() tea.Msg { return status.ErrorMsg{Err: err} }
	}
	m.setAccentOnView(hub)
	if active != nil {
		m.setAccentOnView(active)
		return active.Init()
	}
	return nil
}

func (m *AppModel) registerManageModulePresentation(params route.ManageParams, view route.View) {
	if m.views == nil || view == nil {
		return
	}
	presentation, ok := projecthub.ManageModulePresentation(params)
	if !ok {
		return
	}
	m.views.Register(presentation, view)
}

func (m *AppModel) handleManageHubMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case route.ManageHubModuleSelectedMsg:
		return true, m.activateManageHub(msg.Params)
	default:
		return false, nil
	}
}

func (m *AppModel) handleInvalidManageProjectScope() (bool, tea.Cmd) {
	_, params, ok := m.manageHubPrimary()
	if !ok || params.Module != route.ManageNotes ||
		params.NotesScope.Kind != route.NotesScopeProject ||
		m.activeProjectByID(params.NotesScope.ProjectID) != nil {
		return false, nil
	}
	if m.currentProject != nil && m.currentProject.ID == params.NotesScope.ProjectID {
		m.currentProject = nil
	}
	if view := m.getActiveNotesView(); view != nil {
		view.RefreshWorkspace(m.availableProjects()...)
	}
	params.NotesScope = route.AllNotesScope()
	m.setInfoStatus("Project was removed — showing All Notes")
	return true, m.activateManageHub(params)
}

// refreshActiveNotesManager rebuilds the live owner catalog after a workspace
// lifecycle change. If the current project scope disappeared, both the model
// and the canonical typed route are replaced immediately instead of waiting for
// the next keypress.
func (m *AppModel) refreshActiveNotesManager() tea.Cmd {
	view := m.getActiveNotesView()
	if view == nil {
		return nil
	}
	if !view.RefreshWorkspace(m.availableProjects()...) {
		return nil
	}
	if m.manageHub == nil {
		view.SetErrorStatus("Project is unavailable; reopen Notes to use All Notes")
		return nil
	}

	if m.currentProject != nil && m.currentProject.ID == m.manageHub.Params().NotesScope.ProjectID {
		m.currentProject = nil
	}
	params := m.manageHub.Params()
	params.NotesScope = route.AllNotesScope()
	message := "Project is unavailable — showing All Notes"
	m.setInfoStatus(message)
	cmd := m.activateManageHub(params)
	if active := m.getActiveNotesView(); active != nil {
		active.SetStatus(message)
	}
	return cmd
}
