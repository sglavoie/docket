package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

const manageHubViewMenuOverlayKey = "manage-view-menu"

type manageHubViewMenuMsg struct{ Open bool }

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
	Tags               TagCoordinator
	Wire               func(View)
	ResolveProject     func(string) *models.Project
	CurrentProject     func() *models.Project
	ActiveContext      func() ContextScope
	ActiveProjectFocus func() string
	LoadedProjects     func() []*models.Project
	OperationRegistry  *OperationRegistry
	OperationExecutor  *OperationExecutor
}

type manageHubPickerEntry struct {
	name        string
	description string
	params      ManageParams
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

	pickerTitle   string
	pickerEntries []manageHubPickerEntry
	pickerCursor  int
}

func NewManageHubModel(deps ManageHubDependencies) *ManageHubModel {
	if deps.Runtime == nil {
		deps.Runtime = config.EmptyRuntimeState("")
	}
	return &ManageHubModel{
		deps: deps,
		params: ManageParams{
			Module: ManageNotes,
			Scope:  AppEntityScope(),
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
	if len(m.pickerEntries) > 0 {
		return m.updatePicker(msg)
	}
	if m.active == nil {
		return m, nil
	}

	if key, ok := msg.(tea.KeyPressMsg); ok && !m.HasTextInput() && !m.IsConfirmActive() {
		if key.String() == "v" {
			if m.OpenViewMenu() {
				return m, func() tea.Msg { return manageHubViewMenuMsg{Open: true} }
			}
			return m, nil
		}
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
		case "x":
			return syntheticKey('d')
		}
	}
	return key
}

func (m *ManageHubModel) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "v":
		m.closePicker()
		return m, func() tea.Msg { return manageHubViewMenuMsg{Open: false} }
	case "j", "down":
		m.pickerCursor = min(m.pickerCursor+1, len(m.pickerEntries)-1)
	case "k", "up":
		m.pickerCursor = max(m.pickerCursor-1, 0)
	case "g", "home":
		m.pickerCursor = 0
	case "G", "end":
		m.pickerCursor = len(m.pickerEntries) - 1
	case "enter":
		if m.pickerCursor >= 0 && m.pickerCursor < len(m.pickerEntries) {
			entry := m.pickerEntries[m.pickerCursor]
			m.closePicker()
			return m, func() tea.Msg {
				return ManageHubModuleSelectedMsg{Params: entry.params}
			}
		}
	}
	return m, nil
}

func (m *ManageHubModel) closePicker() {
	m.pickerTitle = ""
	m.pickerEntries = nil
	m.pickerCursor = 0
}

// OpenViewMenu exposes route-owned scope choices. It is intentionally local to
// the hub: selecting an option replaces route params
// and never pushes a tracked child route.
func (m *ManageHubModel) OpenViewMenu() bool {
	params := m.params
	switch {
	case manageModuleSupportsScope(params.Module):
		m.pickerTitle = "Select scope"
		m.pickerEntries = []manageHubPickerEntry{{
			name: "All projects", description: "App scope",
			params: ManageParams{Module: params.Module, Scope: AppEntityScope()},
		}}
		for _, project := range m.deps.Runtime.ActiveProjects() {
			if project == nil || project.ID == "" {
				continue
			}
			m.pickerEntries = append(m.pickerEntries, manageHubPickerEntry{
				name: project.Name, description: "Project scope",
				params: ManageParams{Module: params.Module, Scope: ProjectEntityScope(project.ID)},
			})
		}
	default:
		return false
	}

	m.pickerCursor = 0
	for i, entry := range m.pickerEntries {
		if manageParamsEqual(entry.params, params) {
			m.pickerCursor = i
			break
		}
	}
	return len(m.pickerEntries) > 0
}

// OpenProjectPicker is the fail-safe for a project-only legacy deep link that
// no longer has a valid project. The user remains inside Manage and chooses a
// valid project scope instead of receiving a nil-backed model.
func (m *ManageHubModel) OpenProjectPicker(module ManageModule) bool {
	if !manageModuleSupportsScope(module) {
		return false
	}
	m.pickerTitle = "Select project"
	m.pickerEntries = nil
	for _, project := range m.deps.Runtime.ActiveProjects() {
		if project == nil || project.ID == "" {
			continue
		}
		m.pickerEntries = append(m.pickerEntries, manageHubPickerEntry{
			name: project.Name, description: module.String(),
			params: ManageParams{Module: module, Scope: ProjectEntityScope(project.ID)},
		})
	}
	m.pickerCursor = 0
	return len(m.pickerEntries) > 0
}

// Activate validates and installs one collection module. App scope and project
// scope share one hub slot, so changing scope replaces the embedded model
// without creating another route identity.
func (m *ManageHubModel) Activate(params ManageParams) error {
	route, err := NewRoute(SecondaryTool, params)
	if err != nil {
		return err
	}
	params = route.Params().(ManageParams)
	m.params = params
	m.closePicker()
	project, err := m.projectForScope(params.Scope)
	if err != nil {
		return err
	}
	key := manageModuleCacheKey(params)
	// Contexts and Projects read live state at construction time, so cached
	// instances would show stale rows and Projects would not reopen in search.
	if key == m.activeKey && m.active != nil &&
		params.Module != ManageContexts && params.Module != ManageProjects {
		return nil
	}

	var view View
	switch params.Module {
	case ManageNotes:
		activeProject := project
		if activeProject == nil && m.deps.CurrentProject != nil {
			activeProject = m.deps.CurrentProject()
		}
		preferred := app.GlobalNoteLocation()
		if project != nil {
			preferred = app.ProjectNoteLocation(project.ID)
		}
		view = NewCombinedNotesViewModelWithService(
			storage.GlobalNotesDir(m.deps.Runtime.DataDir()),
			activeProject,
			preferred,
			m.deps.NoteService,
			m.deps.HistoryLedger,
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

func (m *ManageHubModel) projectForScope(scope EntityScope) (*models.Project, error) {
	if scope.Kind == ScopeApp {
		return nil, nil
	}
	if scope.Kind != ScopeProject || scope.ProjectID == "" {
		return nil, ErrProjectRequired
	}
	if m.deps.ResolveProject == nil {
		return nil, fmt.Errorf("project %q is unavailable", scope.ProjectID)
	}
	project := m.deps.ResolveProject(scope.ProjectID)
	if project == nil {
		return nil, fmt.Errorf("project %q is unavailable", scope.ProjectID)
	}
	return project, nil
}

func manageModuleCacheKey(params ManageParams) string {
	return fmt.Sprintf(
		"%d:%d:%s:%t:%s",
		params.Module,
		params.Scope.Kind,
		params.Scope.ProjectID,
		params.ProjectSet.Explicit,
		strings.Join(params.ProjectSet.ProjectIDs(), "\x1f"),
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
	if len(m.pickerEntries) > 0 {
		return m.viewPicker()
	}
	if m.active != nil {
		return m.active.View()
	}
	return tea.NewView("")
}

func (m *ManageHubModel) viewPicker() tea.View {
	width := max(m.width, 1)
	height := max(m.height, 1)
	title := renderFullscreenTitle(m.pickerTitle, m.params.Module.String(), m.AccentColor())
	footer := renderStatusBarFooter(
		HelpStyle.Render("j/k choose · Enter apply · Esc cancel"),
		MutedStyle.Render("Manage View"),
		width,
		m.AccentColor(),
	)
	lines := make([]string, 0, len(m.pickerEntries))
	for i, entry := range m.pickerEntries {
		marker := "  "
		style := BoldStyle
		if i == m.pickerCursor {
			marker = IconCursorRight + " "
			style = SelectedItemTextStyle
		}
		line := marker + style.Render(entry.name) + "  " + MutedStyle.Render(entry.description)
		if i == m.pickerCursor {
			line = SelectionRowStyle.Width(width).Render(truncateToVisualWidth(line, width))
		}
		lines = append(lines, line)
	}
	contentHeight := fullscreenContentHeightForBlocks(height, title, footer)
	content := fitBlockHeight(strings.Join(lines, "\n"), contentHeight)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, title, content, footer))
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
	if len(m.pickerEntries) > 0 {
		return true
	}
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
	if m.active == nil || len(m.pickerEntries) > 0 {
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
	if m.params.Module == ManageContexts || m.params.Module == ManageProjects {
		if provider, ok := m.active.(HelpProvider); ok {
			return append([]HelpItem(nil), provider.GetHelpItems()...)
		}
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
	if m.params.Scope.Kind == ScopeProject {
		return m.params.Module.String() + " · Project"
	}
	return m.params.Module.String() + " · All"
}

func (m *ManageHubModel) CommandContext() CommandContext {
	if view, ok := m.PresentationView(); ok {
		ctx := PresentationCommandContext(view)
		if m.params.Scope.Kind == ScopeProject {
			ctx.Scope = ScopeProject
			delete(ctx.Capabilities, CapabilityAppScope)
			ctx.Capabilities.Add(CapabilityProjectScope)
			ctx.FocusedProjectID = m.params.Scope.ProjectID
		}
		ctx.ScopeProjectIDs = m.ScopeProjectIDs()
		return ctx
	}
	return CommandContext{
		View:  ViewContextHub,
		Scope: ScopeApp,
		Capabilities: NewCapabilitySet(
			CapabilityCommandDiscovery,
			CapabilityAppScope,
		),
	}
}

// ScopeProjectIDs returns the exact project set represented by the active
// typed Manage route. App-scoped legacy modules continue to resolve all active
// projects when no explicit set was supplied.
func (m *ManageHubModel) ScopeProjectIDs() []string {
	if m.params.Scope.Kind == ScopeProject {
		return []string{m.params.Scope.ProjectID}
	}
	projects := projectsInProjectSet(m.deps.Runtime.ActiveProjects(), m.params.ProjectSet)
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return normalizedProjectIDs(ids)
}

// PresentationView exposes the temporary legacy name for command discovery.
// It never becomes a route identity.
func (m *ManageHubModel) PresentationView() (PresentationID, bool) {
	return manageModulePresentation(m.params)
}

func manageModulePresentation(params ManageParams) (PresentationID, bool) {
	switch params.Module {
	case ManageNotes:
		if params.Scope.Kind == ScopeProject {
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
