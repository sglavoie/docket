package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ManageHubModuleSelectedMsg changes the active Manage mode without creating a
// child route. The shell owns the corresponding typed route replacement.
type ManageHubModuleSelectedMsg struct {
	Params       ManageParams
	RulesMode    ManageRulesMode
	HasRulesMode bool
}

// ShowManageHubMsg is the typed entry point used by startup routing and direct
// Manage deep links.
type ShowManageHubMsg struct {
	Params ManageParams
}

const manageHubViewMenuOverlayKey = "manage-view-menu"

type manageHubViewMenuMsg struct{ Open bool }

// ManageRulesMode is hub-owned because the sealed Manage route intentionally
// carries Rules as one module rather than exposing Workflow and Scoring as
// separate routes.
type ManageRulesMode uint8

const (
	ManageRulesWorkflow ManageRulesMode = iota
	ManageRulesScoring
)

func (mode ManageRulesMode) String() string {
	switch mode {
	case ManageRulesWorkflow:
		return "Workflow"
	case ManageRulesScoring:
		return "Scoring"
	default:
		return fmt.Sprintf("ManageRulesMode(%d)", mode)
	}
}

var manageModuleNames = [...]string{
	"Notes",
	"Blockers",
	"Rules",
	"Templates",
	"Tags",
	"Notifications",
	"Settings",
	"Contexts",
}

func (module ManageModule) String() string {
	if int(module) < len(manageModuleNames) {
		return manageModuleNames[module]
	}
	return fmt.Sprintf("ManageModule(%d)", module)
}

func (mode NotificationMode) String() string {
	switch mode {
	case NotificationAlerts:
		return "Alerts"
	case NotificationLog:
		return "Log"
	default:
		return fmt.Sprintf("NotificationMode(%d)", mode)
	}
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
	Blockers           BlockerCoordinator
	Tags               TagCoordinator
	Wire               func(View)
	ResolveProject     func(string) *models.Project
	CurrentProject     func() *models.Project
	NewSettings        func() *SettingsModel
	ActiveContext      func() ContextScope
	ActiveProjectFocus func() string
	LoadedProjects     func() []*models.Project
	OperationRegistry  *OperationRegistry
	OperationExecutor  *OperationExecutor
}

type manageHubPickerEntry struct {
	name         string
	description  string
	params       ManageParams
	rulesMode    ManageRulesMode
	hasRulesMode bool
}

// ManageHubModel is the sole live Manage destination for collection and log
// surfaces. Existing models remain responsible for rendering and mutations;
// their local/global variants are selected by typed route scope rather than by
// independent navigation identities.
type ManageHubModel struct {
	BaseView
	deps ManageHubDependencies

	params    ManageParams
	rulesMode ManageRulesMode
	active    View
	activeKey string

	pickerTitle   string
	pickerEntries []manageHubPickerEntry
	pickerCursor  int

	filteredAlerts *alerts.AlertSnapshot
	rawAlerts      *alerts.AlertSnapshot
	dismissedSet   map[string]bool
	messageEntries []MessageLogEntry
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
	case ManageTags:
		switch key.String() {
		case "e":
			return syntheticKey('r')
		case "x":
			return syntheticKey('d')
		case "space":
			return syntheticKey('p')
		}
	case ManageRules:
		if m.rulesMode == ManageRulesScoring {
			if key.String() == "x" {
				return syntheticKey('d')
			}
			break
		}
		switch key.String() {
		case "a":
			return syntheticKey('n')
		case "x":
			return syntheticKey('d')
		}
	case ManageTemplates:
		switch key.String() {
		case "a":
			return syntheticKey('n')
		case "x":
			return syntheticKey('d')
		}
	case ManageNotifications:
		if m.params.NotificationMode == NotificationAlerts {
			switch key.String() {
			case "x":
				return syntheticKey('d')
			case "space":
				return tea.KeyPressMsg{Code: tea.KeyTab}
			}
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
				return ManageHubModuleSelectedMsg{
					Params: entry.params, RulesMode: entry.rulesMode, HasRulesMode: entry.hasRulesMode,
				}
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

// OpenViewMenu exposes route-owned scope and notification-mode choices. It is
// intentionally local to the hub: selecting an option replaces route params
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
	case params.Module == ManageRules:
		m.pickerTitle = "Rules mode"
		m.pickerEntries = []manageHubPickerEntry{
			{name: "Workflow", description: "Triggers and automated actions", params: params, rulesMode: ManageRulesWorkflow, hasRulesMode: true},
			{name: "Scoring", description: "Scoring profiles and assignment", params: params, rulesMode: ManageRulesScoring, hasRulesMode: true},
		}
	case params.Module == ManageNotifications:
		m.pickerTitle = "Notifications view"
		m.pickerEntries = []manageHubPickerEntry{
			{name: "Alerts", description: "Actionable notifications", params: ManageParams{Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationAlerts}},
			{name: "Log", description: "Recent status messages", params: ManageParams{Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationLog}},
		}
	default:
		return false
	}

	m.pickerCursor = 0
	for i, entry := range m.pickerEntries {
		if manageParamsEqual(entry.params, params) && (!entry.hasRulesMode || entry.rulesMode == m.rulesMode) {
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
	key := manageModuleCacheKey(params, m.rulesMode)
	// Notifications and Contexts read live state at construction time, so a
	// cached instance would show a stale snapshot.
	if key == m.activeKey && m.active != nil &&
		params.Module != ManageNotifications && params.Module != ManageContexts {
		return nil
	}

	var view View
	switch params.Module {
	case ManageNotes:
		if project == nil {
			view = NewGlobalNotesViewModelWithService(
				storage.GlobalNotesDir(m.deps.Runtime.DataDir()),
				m.deps.NoteService,
				m.deps.HistoryLedger,
				m.deps.Runtime.Machine(),
			)
		} else {
			view = NewNotesViewModelWithService(
				project,
				m.deps.NoteService,
				m.deps.HistoryLedger,
				m.deps.Runtime.Machine(),
			)
		}
	case ManageBlockers:
		if project == nil {
			view = NewGlobalBlockersModel(m.deps.Runtime, m.deps.Blockers)
		} else {
			view = NewBlockersModelWithEditor(project, m.deps.Runtime.Machine().GetEditor(), m.deps.Blockers)
		}
	case ManageRules:
		if m.rulesMode == ManageRulesScoring {
			projectID := ""
			if m.deps.CurrentProject != nil {
				if current := m.deps.CurrentProject(); current != nil {
					projectID = current.ID
				}
			}
			view = NewScoreProfileManagerModel(m.deps.Runtime, projectID)
		} else {
			view = NewWorkflowRulesModel(m.deps.Runtime)
		}
	case ManageTemplates:
		view = NewTaskTemplatesModel(m.deps.Runtime)
	case ManageTags:
		if project == nil {
			view = NewGlobalTagManagerModel(
				projectsInProjectSet(m.deps.Runtime.ActiveProjects(), params.ProjectSet),
				m.deps.Runtime,
				m.deps.Tags,
			)
		} else {
			view = NewTagManagerModelForProject(project, m.deps.Tags)
		}
	case ManageNotifications:
		if params.NotificationMode == NotificationLog {
			view = NewMessageLogViewModel(m.messageEntries)
		} else {
			view = NewNotificationCenterModel(m.filteredAlerts, m.rawAlerts, m.dismissedSet)
		}
	case ManageSettings:
		if m.deps.NewSettings == nil {
			return fmt.Errorf("settings dependencies are unavailable")
		}
		view = m.deps.NewSettings()
	case ManageContexts:
		active := AllContextScope()
		if m.deps.ActiveContext != nil {
			active = m.deps.ActiveContext()
		}
		hub := NewContextHubModel(m.deps.Runtime, m.deps.ConfigManager, m.deps.AppService, active)
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

func manageModuleCacheKey(params ManageParams, rulesMode ManageRulesMode) string {
	return fmt.Sprintf(
		"%d:%d:%s:%t:%s:%d:%d",
		params.Module,
		params.Scope.Kind,
		params.Scope.ProjectID,
		params.ProjectSet.Explicit,
		strings.Join(params.ProjectSet.ProjectIDs(), "\x1f"),
		params.NotificationMode,
		rulesMode,
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

func (m *ManageHubModel) SetNotificationData(filtered, raw *alerts.AlertSnapshot, dismissed map[string]bool) {
	m.filteredAlerts = filtered
	m.rawAlerts = raw
	m.dismissedSet = dismissed
}

func (m *ManageHubModel) SetMessageLogEntries(entries []MessageLogEntry) {
	m.messageEntries = append([]MessageLogEntry(nil), entries...)
	if m.params.Module == ManageNotifications && m.params.NotificationMode == NotificationLog {
		m.active = NewMessageLogViewModel(m.messageEntries)
		m.activeKey = manageModuleCacheKey(m.params, m.rulesMode)
		m.prepareActiveView()
	}
}

func (m *ManageHubModel) RefreshNotificationCenter() {
	if m.params.Module != ManageNotifications || m.params.NotificationMode != NotificationAlerts {
		return
	}
	var showDismissed bool
	var cursor int
	if previous, ok := m.active.(*NotificationCenterModel); ok {
		showDismissed = previous.showDismissed
		cursor = previous.cursor
	}
	next := NewNotificationCenterModel(m.filteredAlerts, m.rawAlerts, m.dismissedSet)
	next.showDismissed = showDismissed
	if showDismissed {
		next.rebuildForShowDismissed()
	}
	next.cursor = cursor
	next.clampCursor()
	m.active = next
	m.activeKey = manageModuleCacheKey(m.params, m.rulesMode)
	m.prepareActiveView()
}

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
	if m.params.Module == ManageRules || m.params.Module == ManageTemplates ||
		m.params.Module == ManageSettings || m.params.Module == ManageContexts {
		if provider, ok := m.active.(HelpProvider); ok {
			items := append([]HelpItem(nil), provider.GetHelpItems()...)
			if m.params.Module == ManageRules {
				items = append(items, HelpItem{Key: "v", Description: "Switch Workflow or Scoring", Category: "View"})
			}
			return items
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
	if m.params.Module == ManageNotifications {
		return "Notifications · " + m.params.NotificationMode.String()
	}
	if m.params.Module == ManageRules {
		return "Rules · " + m.rulesMode.String()
	}
	if m.params.Module == ManageContexts {
		return "Contexts"
	}
	if m.params.Scope.Kind == ScopeProject {
		return m.params.Module.String() + " · Project"
	}
	if m.params.Module == ManageTags && m.params.ProjectSet.Explicit {
		return m.params.Module.String() + " · Scope"
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
		}
		ctx.ScopeProjectIDs = m.ScopeProjectIDs()
		return ctx
	}
	return CommandContext{
		View:  ViewSettings,
		Scope: ScopeApp,
		Capabilities: NewCapabilitySet(
			CapabilityCommandDiscovery,
			CapabilityAppScope,
		),
	}
}

func (m *ManageHubModel) RulesMode() ManageRulesMode { return m.rulesMode }

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

// SetRulesMode replaces the Rules submode without changing the typed route.
// Activate performs the actual model construction so callers can update the
// route and renderer in one shell transaction.
func (m *ManageHubModel) SetRulesMode(mode ManageRulesMode) {
	if mode > ManageRulesScoring || mode == m.rulesMode {
		return
	}
	m.rulesMode = mode
	m.activeKey = ""
}

// PresentationView exposes the temporary legacy name for command discovery.
// It never becomes a route identity.
func (m *ManageHubModel) PresentationView() (PresentationID, bool) {
	if m.params.Module == ManageRules && m.rulesMode == ManageRulesScoring {
		return ViewScoreProfileManager, true
	}
	return manageModulePresentation(m.params)
}

func (m *ManageHubModel) RefreshWorkflowRules() {
	if m.params.Module != ManageRules || m.rulesMode != ManageRulesWorkflow {
		return
	}
	m.active = NewWorkflowRulesModel(m.deps.Runtime)
	m.activeKey = manageModuleCacheKey(m.params, m.rulesMode)
	m.prepareActiveView()
}

func (m *ManageHubModel) RefreshScoringProfiles() {
	if m.params.Module != ManageRules || m.rulesMode != ManageRulesScoring {
		return
	}
	projectID := ""
	if m.deps.CurrentProject != nil {
		if current := m.deps.CurrentProject(); current != nil {
			projectID = current.ID
		}
	}
	m.active = NewScoreProfileManagerModel(m.deps.Runtime, projectID)
	m.activeKey = manageModuleCacheKey(m.params, m.rulesMode)
	m.prepareActiveView()
}

func (m *ManageHubModel) RefreshTemplates() {
	if m.params.Module != ManageTemplates {
		return
	}
	m.active = NewTaskTemplatesModel(m.deps.Runtime)
	m.activeKey = manageModuleCacheKey(m.params, m.rulesMode)
	m.prepareActiveView()
}

func manageModulePresentation(params ManageParams) (PresentationID, bool) {
	switch params.Module {
	case ManageNotes:
		if params.Scope.Kind == ScopeProject {
			return ViewNotes, true
		}
		return ViewGlobalNotes, true
	case ManageBlockers:
		if params.Scope.Kind == ScopeProject {
			return ViewBlockers, true
		}
		return ViewGlobalBlockers, true
	case ManageRules:
		return ViewWorkflowRules, true
	case ManageTemplates:
		return ViewTaskTemplates, true
	case ManageTags:
		if params.Scope.Kind == ScopeProject {
			return ViewTagManager, true
		}
		return ViewGlobalTagManager, true
	case ManageNotifications:
		if params.NotificationMode == NotificationLog {
			return ViewMessageLog, true
		}
		return ViewNotificationCenter, true
	case ManageSettings:
		return ViewSettings, true
	case ManageContexts:
		return ViewContextHub, true
	default:
		return 0, false
	}
}
