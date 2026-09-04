package tui

import (
	"fmt"
	"image/color"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/route"
)

// The module-selection message this hub answers is route.ManageHubModuleSelectedMsg.
// It sits beside route.ManageParams because Notes raises it from below.

// ManageHubDependencies collects the existing transaction-backed collection
// seams. The hub owns navigation and lifecycle only; mutations remain in the
// application service and feature coordinators used by the embedded models.
type ManageHubDependencies struct {
	Runtime            *config.RuntimeState
	NoteService        notes.ProjectNoteService
	HistoryLedger      *storage.WorkspaceLedger
	Wire               func(route.View)
	ResolveProject     func(string) *models.Project
	ActiveProjectFocus func() string
	TaskCounts         func() map[string]projectTaskTally
	OperationRegistry  *operation.OperationRegistry
	OperationExecutor  *operation.OperationExecutor
	DispatchKey        operation.KeyDispatch
}

// ManageHubModel is the sole live Manage destination for collection and log
// surfaces. Existing models remain responsible for rendering and mutations;
// their local/global variants are selected by typed route scope rather than by
// independent navigation identities.
type ManageHubModel struct {
	listsurface.BaseView
	deps ManageHubDependencies

	params    route.ManageParams
	active    route.View
	activeKey string
}

func NewManageHubModel(deps ManageHubDependencies) *ManageHubModel {
	if deps.Runtime == nil {
		deps.Runtime = config.EmptyRuntimeState("")
	}
	return &ManageHubModel{
		deps: deps,
		params: route.ManageParams{
			Module:     route.ManageNotes,
			NotesScope: route.AllNotesScope(),
		},
	}
}

func (m *ManageHubModel) Init() tea.Cmd {
	if m.active != nil {
		return m.active.Init()
	}
	return nil
}

func (m *ManageHubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.active == nil {
		return m, nil
	}

	updated, cmd := m.active.Update(msg)
	if view, ok := updated.(route.View); ok {
		m.active = view
	}
	return m, cmd
}

// Activate validates and installs one collection module. App scope and project
// scope share one hub slot, so changing scope replaces the embedded model
// without creating another route identity.
func (m *ManageHubModel) Activate(params route.ManageParams) error {
	previousParams := m.params
	target, err := route.NewRoute(route.SecondaryTool, params)
	if err != nil {
		return err
	}
	params = target.Params().(route.ManageParams)
	m.params = params
	if _, err := m.projectForNotesScope(params); err != nil {
		return err
	}
	key := manageModuleCacheKey(params)
	if params.Module == route.ManageNotes && previousParams.Module == route.ManageNotes {
		if manager, ok := m.active.(*notes.NotesViewModel); ok {
			manager.SetScope(params.NotesScope)
			m.activeKey = key
			m.prepareActiveView()
			return nil
		}
	}
	// Projects reads live state at construction time, so a cached instance would
	// show stale rows.
	if key == m.activeKey && m.active != nil && params.Module != route.ManageProjects {
		return nil
	}

	var view route.View
	switch params.Module {
	case route.ManageNotes:
		manager := notes.NewNotesViewModelWithService(
			storage.GlobalNotesDir(m.deps.Runtime.DataDir()),
			params.NotesScope,
			m.deps.NoteService,
			m.deps.HistoryLedger,
			m.deps.Runtime.AvailableProjects()...,
		)
		manager.SetOperationSystem(m.deps.OperationRegistry, m.deps.DispatchKey)
		view = manager
	case route.ManageProjects:
		hub := NewProjectHubModel(m.deps.Runtime, params.ProjectHubMode)
		hub.SetOperationSystem(m.deps.OperationRegistry, m.deps.OperationExecutor, m.deps.DispatchKey)
		if m.deps.ActiveProjectFocus != nil {
			hub.SetProjectFocus(m.deps.ActiveProjectFocus())
		}
		if m.deps.TaskCounts != nil {
			hub.SetTaskCounts(m.deps.TaskCounts())
		}
		view = hub
	default:
		return fmt.Errorf("unsupported manage module %s", params.Module)
	}

	m.active = view
	m.activeKey = key
	m.prepareActiveView()
	return nil
}

func (m *ManageHubModel) projectForNotesScope(params route.ManageParams) (*models.Project, error) {
	if params.Module != route.ManageNotes || params.NotesScope.Kind != route.NotesScopeProject {
		return nil, nil
	}
	if params.NotesScope.ProjectID == "" {
		return nil, route.ErrProjectRequired
	}
	if m.deps.ResolveProject == nil {
		return nil, fmt.Errorf("project %q is unavailable", params.NotesScope.ProjectID)
	}
	project := m.deps.ResolveProject(params.NotesScope.ProjectID)
	if project == nil {
		return nil, fmt.Errorf("project %q is unavailable", params.NotesScope.ProjectID)
	}
	return project, nil
}

func manageModuleCacheKey(params route.ManageParams) string {
	return fmt.Sprintf(
		"%d:%d:%s:%d",
		params.Module,
		params.NotesScope.Kind,
		params.NotesScope.ProjectID,
		params.ProjectHubMode,
	)
}

func (m *ManageHubModel) prepareActiveView() {
	if m.active == nil {
		return
	}
	m.active.SetSize(m.Width(), m.Height())
	if setter, ok := m.active.(route.AccentColorSetter); ok {
		setter.SetAccentColor(m.AccentColor())
	}
	if m.deps.Wire != nil {
		m.deps.Wire(m.active)
	}
}

func (m *ManageHubModel) Params() route.ManageParams { return m.params }

func (m *ManageHubModel) ActiveModuleView() route.View { return m.active }

func (m *ManageHubModel) View() tea.View {
	if m.active != nil {
		return m.active.View()
	}
	return tea.NewView("")
}

func (m *ManageHubModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	if m.active != nil {
		m.active.SetSize(width, height)
	}
}

func (m *ManageHubModel) SetAccentColor(accent color.Color) {
	m.BaseView.SetAccentColor(accent)
	if setter, ok := m.active.(route.AccentColorSetter); ok {
		setter.SetAccentColor(accent)
	}
}

func (m *ManageHubModel) SetWorkspacePulse(workspacePulse pulse.WorkspacePulse) {
	m.BaseView.SetWorkspacePulse(workspacePulse)
	if injector, ok := m.active.(pulse.WorkspacePulseAware); ok {
		injector.SetWorkspacePulse(workspacePulse)
	}
}

func (m *ManageHubModel) ShouldCaptureEsc() bool {
	if caps, ok := m.active.(route.ViewCapabilities); ok {
		return caps.ShouldCaptureEsc()
	}
	return false
}

func (m *ManageHubModel) HasTextInput() bool {
	if caps, ok := m.active.(route.ViewCapabilities); ok {
		return caps.HasTextInput()
	}
	return false
}

func (m *ManageHubModel) IsFullHeight() bool {
	if m.active == nil {
		return true
	}
	if caps, ok := m.active.(route.ViewCapabilities); ok {
		return caps.IsFullHeight()
	}
	return false
}

func (m *ManageHubModel) GetConfirmDialog() *chrome.ConfirmDialog {
	if provider, ok := m.active.(chrome.ConfirmDialogProvider); ok {
		return provider.GetConfirmDialog()
	}
	return nil
}

func (m *ManageHubModel) IsConfirmActive() bool {
	if provider, ok := m.active.(interface{ IsConfirmActive() bool }); ok {
		return provider.IsConfirmActive()
	}
	return m.GetConfirmDialog() != nil && m.GetConfirmDialog().IsActive()
}

func (m *ManageHubModel) IsMultiSelectActive() bool {
	if provider, ok := m.active.(interface{ IsMultiSelectActive() bool }); ok {
		return provider.IsMultiSelectActive()
	}
	return false
}

func (m *ManageHubModel) ActiveFilterCount() int {
	if provider, ok := m.active.(route.ActiveFilterProvider); ok {
		return provider.ActiveFilterCount()
	}
	return 0
}

func (m *ManageHubModel) getTaskRef() (*models.Task, *models.Project) {
	if provider, ok := m.active.(taskReferenceProvider); ok {
		return provider.getTaskRef()
	}
	return nil, nil
}

func (m *ManageHubModel) GetHelpItems() []operation.HelpItem {
	if provider, ok := m.active.(help.HelpProvider); ok {
		return append([]operation.HelpItem(nil), provider.GetHelpItems()...)
	}
	// Both collections answer HelpProvider, so an unactivated hub has no grammar
	// to describe rather than a generic one.
	return nil
}

func (m *ManageHubModel) StatusBarLabel() string {
	if m.params.Module == route.ManageProjects {
		return m.params.Module.String()
	}
	switch m.params.NotesScope.Kind {
	case route.NotesScopeGlobal:
		return "Notes · Global"
	case route.NotesScopeProject:
		return "Notes · Project"
	default:
		return "Notes · All"
	}
}

func (m *ManageHubModel) CommandContext() operation.CommandContext {
	if view, ok := m.PresentationView(); ok {
		ctx := route.PresentationCommandContext(view)
		if m.params.Module == route.ManageNotes && m.params.NotesScope.CommandScope().Kind == route.ScopeProject {
			delete(ctx.Capabilities, operation.CapabilityAppScope)
			ctx.Capabilities.Add(operation.CapabilityProjectScope)
		} else {
			delete(ctx.Capabilities, operation.CapabilityProjectScope)
			ctx.Capabilities.Add(operation.CapabilityAppScope)
		}
		ctx.ScopeProjectIDs = m.ScopeProjectIDs()
		return ctx
	}
	return operation.CommandContext{
		View: presentation.ViewProjectHub,
		Capabilities: operation.NewCapabilitySet(
			operation.CapabilityCommandDiscovery,
			operation.CapabilityAppScope,
		),
	}
}

// ScopeProjectIDs returns the project set represented by the active typed
// Manage route. All and Global Notes have app command scope and therefore
// expose every available note owner; project Notes exposes only its selected
// owner.
func (m *ManageHubModel) ScopeProjectIDs() []string {
	if m.params.Module == route.ManageNotes && m.params.NotesScope.Kind == route.NotesScopeProject {
		return []string{m.params.NotesScope.ProjectID}
	}
	projects := m.deps.Runtime.AvailableProjects()
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return operation.NormalizedProjectIDs(ids)
}

// PresentationView exposes the active renderer identity for command discovery.
// It never becomes a route identity.
func (m *ManageHubModel) PresentationView() (presentation.PresentationID, bool) {
	return manageModulePresentation(m.params)
}

func manageModulePresentation(params route.ManageParams) (presentation.PresentationID, bool) {
	switch params.Module {
	case route.ManageNotes:
		if params.NotesScope.Kind == route.NotesScopeProject {
			return presentation.ViewNotes, true
		}
		return presentation.ViewGlobalNotes, true
	case route.ManageProjects:
		return presentation.ViewProjectHub, true
	default:
		return 0, false
	}
}
