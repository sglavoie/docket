package tui

import (
	"fmt"
	"image/color"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ManageHubModuleSelectedMsg changes the active Manage mode without creating a
// child route. The shell owns the corresponding typed route replacement.
type ManageHubModuleSelectedMsg struct {
	Params ManageParams
}

// ShowManageHubMsg is the typed entry point used by startup routing and direct
// Manage deep links.
type ShowManageHubMsg struct {
	Params ManageParams
}

var manageModuleNames = [...]string{
	"Notes",
	"Contexts",
	"Projects",
}

func (module ManageModule) String() string {
	if int(module) < len(manageModuleNames) {
		return manageModuleNames[module]
	}
	return fmt.Sprintf("ManageModule(%d)", module)
}

// ManageHubDependencies collects the existing transaction-backed collection
// seams. The hub owns navigation and lifecycle only; mutations remain in the
// application service and feature coordinators used by the embedded models.
type ManageHubDependencies struct {
	Runtime            *config.RuntimeState
	ConfigManager      *config.Manager
	AppService         *app.Service
	NoteService        projectNoteService
	HistoryLedger      *storage.WorkspaceLedger
	Wire               func(View)
	ResolveProject     func(string) *models.Project
	ActiveContext      func() ContextScope
	ActiveProjectFocus func() string
	LoadedProjects     func() []*models.Project
	OperationRegistry  *OperationRegistry
	OperationExecutor  *OperationExecutor
}

// ManageHubModel is the sole live Manage destination for collection and log
// surfaces. Existing models remain responsible for rendering and mutations;
// their local/global variants are selected by typed route scope rather than by
// independent navigation identities.
type ManageHubModel struct {
	BaseView
	deps ManageHubDependencies

	params    ManageParams
	active    View
	activeKey string
}

func NewManageHubModel(deps ManageHubDependencies) *ManageHubModel {
	if deps.Runtime == nil {
		deps.Runtime = config.EmptyRuntimeState("")
	}
	return &ManageHubModel{
		deps: deps,
		params: ManageParams{
			Module:     ManageNotes,
			NotesScope: AllNotesScope(),
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

	if key, ok := msg.(tea.KeyPressMsg); ok && !m.HasTextInput() && !m.IsConfirmActive() {
		msg = m.normalizeCollectionKey(key)
	}

	updated, cmd := m.active.Update(msg)
	if view, ok := updated.(View); ok {
		m.active = view
	}
	return m, cmd
}

func (m *ManageHubModel) normalizeCollectionKey(key tea.KeyPressMsg) tea.Msg {
	switch m.params.Module { //nolint:exhaustive // Only collection modules translate shared action keys.
	case ManageNotes:
		switch key.String() {
		case "a":
			return syntheticKey('n')
		}
	}
	return key
}

// Activate validates and installs one collection module. App scope and project
// scope share one hub slot, so changing scope replaces the embedded model
// without creating another route identity.
func (m *ManageHubModel) Activate(params ManageParams) error {
	previousParams := m.params
	route, err := NewRoute(SecondaryTool, params)
	if err != nil {
		return err
	}
	params = route.Params().(ManageParams)
	m.params = params
	if _, err := m.projectForNotesScope(params); err != nil {
		return err
	}
	key := manageModuleCacheKey(params)
	if params.Module == ManageNotes && previousParams.Module == ManageNotes {
		if notes, ok := m.active.(*NotesViewModel); ok {
			notes.SetScope(params.NotesScope)
			m.activeKey = key
			m.prepareActiveView()
			return nil
		}
	}
	// Contexts and Projects read live state at construction time, so cached
	// instances would show stale rows and Projects would not reopen in search.
	if key == m.activeKey && m.active != nil &&
		params.Module != ManageContexts && params.Module != ManageProjects {
		return nil
	}

	var view View
	switch params.Module {
	case ManageNotes:
		view = NewNotesViewModelWithService(
			storage.GlobalNotesDir(m.deps.Runtime.DataDir()),
			params.NotesScope,
			m.deps.NoteService,
			m.deps.HistoryLedger,
			m.deps.Runtime.ActiveProjects()...,
		)
	case ManageContexts:
		active := AllContextScope()
		if m.deps.ActiveContext != nil {
			active = m.deps.ActiveContext()
		}
		hub := NewContextHubModel(m.deps.Runtime, m.deps.ConfigManager, m.deps.AppService, active)
		hub.SetOperationSystem(m.deps.OperationRegistry, m.deps.OperationExecutor)
		if m.deps.LoadedProjects != nil {
			hub.SetLoadedProjects(m.deps.LoadedProjects())
		}
		view = hub
	case ManageProjects:
		hub := NewProjectHubModel(m.deps.Runtime)
		hub.SetOperationSystem(m.deps.OperationRegistry, m.deps.OperationExecutor)
		if m.deps.ActiveProjectFocus != nil {
			hub.SetProjectFocus(m.deps.ActiveProjectFocus())
		}
		if m.deps.LoadedProjects != nil {
			hub.SetLoadedProjects(m.deps.LoadedProjects())
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

func (m *ManageHubModel) projectForNotesScope(params ManageParams) (*models.Project, error) {
	if params.Module != ManageNotes || params.NotesScope.Kind != NotesScopeProject {
		return nil, nil
	}
	if params.NotesScope.ProjectID == "" {
		return nil, ErrProjectRequired
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

func manageModuleCacheKey(params ManageParams) string {
	return fmt.Sprintf(
		"%d:%d:%s",
		params.Module,
		params.NotesScope.Kind,
		params.NotesScope.ProjectID,
	)
}

func (m *ManageHubModel) prepareActiveView() {
	if m.active == nil {
		return
	}
	m.active.SetSize(m.width, m.height)
	if setter, ok := m.active.(AccentColorSetter); ok {
		setter.SetAccentColor(m.AccentColor())
	}
	if m.deps.Wire != nil {
		m.deps.Wire(m.active)
	}
}

func (m *ManageHubModel) Params() ManageParams { return m.params }

func (m *ManageHubModel) ActiveModuleView() View { return m.active }

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
	if setter, ok := m.active.(AccentColorSetter); ok {
		setter.SetAccentColor(accent)
	}
}

func (m *ManageHubModel) ShouldCaptureEsc() bool {
	if caps, ok := m.active.(ViewCapabilities); ok {
		return caps.ShouldCaptureEsc()
	}
	return false
}

func (m *ManageHubModel) HasTextInput() bool {
	if caps, ok := m.active.(ViewCapabilities); ok {
		return caps.HasTextInput()
	}
	return false
}

func (m *ManageHubModel) IsFullHeight() bool {
	if m.active == nil {
		return true
	}
	if caps, ok := m.active.(ViewCapabilities); ok {
		return caps.IsFullHeight()
	}
	return false
}

func (m *ManageHubModel) GetConfirmDialog() *ConfirmDialog {
	if provider, ok := m.active.(ConfirmDialogProvider); ok {
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
	if provider, ok := m.active.(ActiveFilterProvider); ok {
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

func (m *ManageHubModel) GetHelpItems() []HelpItem {
	if provider, ok := m.active.(HelpProvider); ok {
		return append([]HelpItem(nil), provider.GetHelpItems()...)
	}
	return []HelpItem{
		{Key: "j/k", Description: "Move selection", Category: "Navigation"},
		{Key: "enter", Description: "Open selected item", Category: "Navigation"},
		{Key: "a/e/x", Description: "Add, edit, or delete", Category: "Actions"},
		{Key: "space", Description: "Select or toggle", Category: "Actions"},
		{Key: "/", Description: "Filter collection", Category: "Filters"},
		{Key: "v", Description: "Scope or view", Category: "View"},
	}
}

func (m *ManageHubModel) StatusBarLabel() string {
	if m.params.Module == ManageContexts || m.params.Module == ManageProjects {
		return m.params.Module.String()
	}
	switch m.params.NotesScope.Kind {
	case NotesScopeGlobal:
		return "Notes · Global"
	case NotesScopeProject:
		return "Notes · Project"
	default:
		return "Notes · All"
	}
}

func (m *ManageHubModel) CommandContext() CommandContext {
	if view, ok := m.PresentationView(); ok {
		ctx := PresentationCommandContext(view)
		if m.params.Module == ManageNotes && m.params.NotesScope.commandScope().Kind == ScopeProject {
			delete(ctx.Capabilities, CapabilityAppScope)
			ctx.Capabilities.Add(CapabilityProjectScope)
		} else {
			delete(ctx.Capabilities, CapabilityProjectScope)
			ctx.Capabilities.Add(CapabilityAppScope)
		}
		ctx.ScopeProjectIDs = m.ScopeProjectIDs()
		return ctx
	}
	return CommandContext{
		View: ViewContextHub,
		Capabilities: NewCapabilitySet(
			CapabilityCommandDiscovery,
			CapabilityAppScope,
		),
	}
}

// ScopeProjectIDs returns the project set represented by the active typed
// Manage route. All and Global Notes have app command scope and therefore
// expose all active projects; project Notes exposes only its selected owner.
func (m *ManageHubModel) ScopeProjectIDs() []string {
	if m.params.Module == ManageNotes && m.params.NotesScope.Kind == NotesScopeProject {
		return []string{m.params.NotesScope.ProjectID}
	}
	projects := m.deps.Runtime.ActiveProjects()
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return normalizedProjectIDs(ids)
}

// PresentationView exposes the active renderer identity for command discovery.
// It never becomes a route identity.
func (m *ManageHubModel) PresentationView() (PresentationID, bool) {
	return manageModulePresentation(m.params)
}

func manageModulePresentation(params ManageParams) (PresentationID, bool) {
	switch params.Module {
	case ManageNotes:
		if params.NotesScope.Kind == NotesScopeProject {
			return ViewNotes, true
		}
		return ViewGlobalNotes, true
	case ManageContexts:
		return ViewContextHub, true
	case ManageProjects:
		return ViewProjectHub, true
	default:
		return 0, false
	}
}
