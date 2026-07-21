package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ReviewHubModuleSelectedMsg changes the active Review Hub mode without
// introducing a child route. The shell owns the corresponding typed route
// replacement so Esc can return to the landing menu first.
type ReviewHubModuleSelectedMsg struct {
	Module  ReviewModule
	Project *models.Project
}

// ShowReviewHubMsg opens the Review destination at a named module. It is the
// typed entry point used by startup routing and direct Review deep links.
type ShowReviewHubMsg struct {
	Module  ReviewModule
	Project *models.Project
}

type reviewHubEntry struct {
	module      ReviewModule
	name        string
	description string
}

var reviewHubEntries = []reviewHubEntry{
	{module: ReviewDaily, name: "Daily", description: "Plan today and review recent work"},
	{module: ReviewWeekly, name: "Weekly", description: "Review project health and the week ahead"},
	{module: ReviewActivity, name: "Activity", description: "Browse recent and completed tasks"},
	{module: ReviewMetrics, name: "Metrics", description: "Inspect completion, cycle time, and throughput"},
	{module: ReviewRoadmap, name: "Roadmap", description: "Explore timelines, forecasts, and critical paths"},
}

var reviewModuleNames = [...]string{
	"Review",
	"Daily Review",
	"Weekly Review",
	"Activity",
	"Metrics",
	"Roadmap",
}

func (module ReviewModule) String() string {
	if int(module) < len(reviewModuleNames) {
		return reviewModuleNames[module]
	}
	return fmt.Sprintf("ReviewModule(%d)", module)
}

// ReviewHubModel is the sole live Review destination. Existing review models
// remain responsible for their calculations, mutations, and rendering; the hub
// owns their lifecycle, landing navigation, scope, and module return semantics.
type ReviewHubModel struct {
	BaseView
	runtime       *config.RuntimeState
	configManager *config.Manager
	wire          func(View)

	module  ReviewModule
	cursor  int
	project *models.Project

	daily           *DailyReviewModel
	weekly          *WeeklyReviewModel
	activity        *RecentActivityModel
	activityScopeID string
	metrics         *MetricsDashboardModel
	metricsScopeID  string
	roadmap         *RoadmapModel
	roadmapScopeID  string

	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool
}

func NewReviewHubModel(runtime *config.RuntimeState, manager *config.Manager, wire func(View)) *ReviewHubModel {
	return &ReviewHubModel{
		runtime:       runtime,
		configManager: manager,
		wire:          wire,
		module:        ReviewLanding,
	}
}

func (m *ReviewHubModel) Init() tea.Cmd {
	if active := m.ActiveModuleView(); active != nil {
		return active.Init()
	}
	return nil
}

func (m *ReviewHubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.module == ReviewLanding {
		return m.updateLanding(msg)
	}
	active := m.ActiveModuleView()
	if active == nil {
		return m, nil
	}
	updated, cmd := active.Update(msg)
	if view, ok := updated.(View); ok {
		m.replaceActiveModule(view)
	}
	return m, cmd
}

func (m *ReviewHubModel) updateLanding(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "j", "down":
		m.cursor = min(m.cursor+1, len(reviewHubEntries)-1)
	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(reviewHubEntries) - 1
	case "enter":
		return m.selectLandingEntry(m.cursor)
	case "1", "2", "3", "4", "5":
		return m.selectLandingEntry(int(key.String()[0] - '1'))
	}
	return m, nil
}

func (m *ReviewHubModel) selectLandingEntry(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(reviewHubEntries) {
		return m, nil
	}
	entry := reviewHubEntries[index]
	project := m.project
	return m, func() tea.Msg {
		return ReviewHubModuleSelectedMsg{Module: entry.module, Project: project}
	}
}

func (m *ReviewHubModel) View() tea.View {
	if active := m.ActiveModuleView(); active != nil {
		return active.View()
	}
	return m.viewLanding()
}

func (m *ReviewHubModel) viewLanding() tea.View {
	width := max(m.width, 1)
	height := max(m.height, 1)
	title := renderFullscreenTitle("Review", "Reviews and reports", m.AccentColor())
	footer := renderStatusBarFooter(
		HelpStyle.Render("j/k choose · Enter open · 1-5 direct · Esc Home · ? help"),
		MutedStyle.Render("Review"),
		width,
		m.AccentColor(),
	)

	lines := make([]string, 0, len(reviewHubEntries)*2+1)
	for i, entry := range reviewHubEntries {
		marker := "  "
		nameStyle := BoldStyle
		if i == m.cursor {
			marker = IconCursorRight + " "
			nameStyle = SelectedItemTextStyle
		}
		name := fmt.Sprintf("%d  %-9s", i+1, entry.name)
		line := marker + nameStyle.Render(name) + " " + MutedStyle.Render(entry.description)
		line = truncateToVisualWidth(line, width)
		if i == m.cursor {
			line = SelectionRowStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}

	contentHeight := fullscreenContentHeightForBlocks(height, title, footer)
	content := fitBlockHeight(strings.Join(lines, "\n"), contentHeight)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, title, content, footer))
}

// Activate switches the hub mode and lazily creates the requested legacy model.
// A nil project selects app scope; project-aware modules are recreated only
// when their requested scope changes, preserving in-module state otherwise.
func (m *ReviewHubModel) Activate(module ReviewModule, project *models.Project) error {
	if module > ReviewRoadmap {
		return fmt.Errorf("unknown review module: %d", module)
	}
	m.module = module
	m.project = project
	if module == ReviewLanding {
		return nil
	}

	scopeID := reviewScopeID(project)
	var view View
	switch module {
	case ReviewLanding:
		return nil
	case ReviewDaily:
		if m.daily == nil {
			m.daily = NewDailyReviewModel(m.runtime, m.configManager)
			m.wireModule(m.daily)
		}
		m.daily.queueIDs, m.daily.dailyPlanIDs = m.queueIDs, m.dailyPlanIDs
		view = m.daily
	case ReviewWeekly:
		if m.weekly == nil {
			m.weekly = NewWeeklyReviewModel(m.runtime, m.configManager)
			m.wireModule(m.weekly)
		}
		m.weekly.queueIDs, m.weekly.dailyPlanIDs = m.queueIDs, m.dailyPlanIDs
		view = m.weekly
	case ReviewActivity:
		if m.activity == nil || m.activityScopeID != scopeID {
			if project == nil {
				m.activity = NewRecentActivityModel(m.runtime, m.configManager)
			} else {
				m.activity = NewProjectRecentActivityModel(m.runtime, m.configManager, project)
			}
			m.activityScopeID = scopeID
			m.wireModule(m.activity)
		}
		m.activity.queueIDs, m.activity.dailyPlanIDs = m.queueIDs, m.dailyPlanIDs
		view = m.activity
	case ReviewMetrics:
		if m.metrics == nil || m.metricsScopeID != scopeID {
			if project == nil {
				m.metrics = NewMetricsDashboardModel(m.configManager)
			} else {
				m.metrics = NewMetricsDashboardModelForProject(m.configManager, project)
			}
			m.metricsScopeID = scopeID
			m.wireModule(m.metrics)
		}
		view = m.metrics
	case ReviewRoadmap:
		if m.roadmap == nil || m.roadmapScopeID != scopeID {
			if project == nil {
				m.roadmap = NewRoadmapModel(m.runtime)
			} else {
				m.roadmap = NewRoadmapModelForProject(m.runtime, project)
			}
			m.roadmapScopeID = scopeID
			m.wireModule(m.roadmap)
		}
		view = m.roadmap
	}
	if view != nil {
		view.SetSize(m.width, m.height)
		if setter, ok := view.(AccentColorSetter); ok {
			setter.SetAccentColor(m.AccentColor())
		}
	}
	return nil
}

func reviewScopeID(project *models.Project) string {
	if project == nil {
		return "app"
	}
	return "project:" + project.ID
}

func (m *ReviewHubModel) wireModule(view View) {
	if m.wire != nil {
		m.wire(view)
	}
}

func (m *ReviewHubModel) replaceActiveModule(view View) {
	switch m.module {
	case ReviewLanding:
	case ReviewDaily:
		m.daily, _ = view.(*DailyReviewModel)
	case ReviewWeekly:
		m.weekly, _ = view.(*WeeklyReviewModel)
	case ReviewActivity:
		m.activity, _ = view.(*RecentActivityModel)
	case ReviewMetrics:
		m.metrics, _ = view.(*MetricsDashboardModel)
	case ReviewRoadmap:
		m.roadmap, _ = view.(*RoadmapModel)
	}
}

func (m *ReviewHubModel) ActiveModuleView() View {
	switch m.module {
	case ReviewLanding:
		return nil
	case ReviewDaily:
		if m.daily != nil {
			return m.daily
		}
	case ReviewWeekly:
		if m.weekly != nil {
			return m.weekly
		}
	case ReviewActivity:
		if m.activity != nil {
			return m.activity
		}
	case ReviewMetrics:
		if m.metrics != nil {
			return m.metrics
		}
	case ReviewRoadmap:
		if m.roadmap != nil {
			return m.roadmap
		}
	}
	return nil
}

func (m *ReviewHubModel) Module() ReviewModule { return m.module }

func (m *ReviewHubModel) ProjectScope() *models.Project { return m.project }

func (m *ReviewHubModel) SetPlanningIndicators(queueIDs, dailyPlanIDs map[string]bool) {
	m.queueIDs, m.dailyPlanIDs = queueIDs, dailyPlanIDs
	if m.daily != nil {
		m.daily.queueIDs, m.daily.dailyPlanIDs = queueIDs, dailyPlanIDs
	}
	if m.weekly != nil {
		m.weekly.queueIDs, m.weekly.dailyPlanIDs = queueIDs, dailyPlanIDs
	}
	if m.activity != nil {
		m.activity.queueIDs, m.activity.dailyPlanIDs = queueIDs, dailyPlanIDs
	}
}

func (m *ReviewHubModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	for _, view := range m.cachedModuleViews() {
		view.SetSize(width, height)
	}
}

func (m *ReviewHubModel) SetAccentColor(accent color.Color) {
	m.BaseView.SetAccentColor(accent)
	for _, view := range m.cachedModuleViews() {
		if setter, ok := view.(AccentColorSetter); ok {
			setter.SetAccentColor(accent)
		}
	}
}

func (m *ReviewHubModel) cachedModuleViews() []View {
	views := make([]View, 0, 5)
	if m.daily != nil {
		views = append(views, m.daily)
	}
	if m.weekly != nil {
		views = append(views, m.weekly)
	}
	if m.activity != nil {
		views = append(views, m.activity)
	}
	if m.metrics != nil {
		views = append(views, m.metrics)
	}
	if m.roadmap != nil {
		views = append(views, m.roadmap)
	}
	return views
}

func (m *ReviewHubModel) ShouldCaptureEsc() bool {
	if caps, ok := m.ActiveModuleView().(ViewCapabilities); ok {
		return caps.ShouldCaptureEsc()
	}
	return false
}

func (m *ReviewHubModel) HasTextInput() bool {
	if caps, ok := m.ActiveModuleView().(ViewCapabilities); ok {
		return caps.HasTextInput()
	}
	return false
}

func (m *ReviewHubModel) IsFullHeight() bool { return true }

func (m *ReviewHubModel) HasPendingPrefix() bool {
	if provider, ok := m.ActiveModuleView().(PendingPrefixView); ok {
		return provider.HasPendingPrefix()
	}
	return false
}

func (m *ReviewHubModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if provider, ok := m.ActiveModuleView().(PrefixPopupProvider); ok {
		return provider.GetPrefixPopupInfo()
	}
	return nil
}

func (m *ReviewHubModel) GetConfirmDialog() *ConfirmDialog {
	if provider, ok := m.ActiveModuleView().(ConfirmDialogProvider); ok {
		return provider.GetConfirmDialog()
	}
	return nil
}

func (m *ReviewHubModel) IsConfirmActive() bool {
	if provider, ok := m.ActiveModuleView().(interface{ IsConfirmActive() bool }); ok {
		return provider.IsConfirmActive()
	}
	return false
}

func (m *ReviewHubModel) IsMultiSelectActive() bool {
	if provider, ok := m.ActiveModuleView().(interface{ IsMultiSelectActive() bool }); ok {
		return provider.IsMultiSelectActive()
	}
	return false
}

func (m *ReviewHubModel) ActiveFilterCount() int {
	if provider, ok := m.ActiveModuleView().(ActiveFilterProvider); ok {
		return provider.ActiveFilterCount()
	}
	return 0
}

func (m *ReviewHubModel) getTaskRef() (*models.Task, *models.Project) {
	if provider, ok := m.ActiveModuleView().(taskReferenceProvider); ok {
		return provider.getTaskRef()
	}
	return nil, nil
}

func (m *ReviewHubModel) GetHelpItems() []HelpItem {
	if provider, ok := m.ActiveModuleView().(HelpProvider); ok {
		return provider.GetHelpItems()
	}
	return []HelpItem{
		{Key: "j/k", Description: "Choose module", Category: "Navigation"},
		{Key: "enter", Description: "Open module", Category: "Navigation"},
		{Key: "1-5", Description: "Open named module", Category: "Navigation"},
		{Key: "esc", Description: "Return Home", Category: "Navigation"},
	}
}

func (m *ReviewHubModel) StatusBarLabel() string { return m.module.String() }

func (m *ReviewHubModel) SupportsTaskUndoRedo() bool {
	return m.module == ReviewDaily || m.module == ReviewWeekly || m.module == ReviewActivity
}

func (m *ReviewHubModel) CommandContext() CommandContext {
	if view, ok := reviewModulePresentation(m.module, m.project); ok && m.module != ReviewLanding {
		ctx := PresentationCommandContext(view)
		if m.project != nil {
			ctx.Scope = ScopeProject
			delete(ctx.Capabilities, CapabilityAppScope)
			ctx.Capabilities.Add(CapabilityProjectScope)
		}
		return ctx
	}
	return CommandContext{
		View:  ViewDailyReview,
		Scope: ScopeApp,
		Capabilities: NewCapabilitySet(
			CapabilityCommandDiscovery,
			CapabilityAppScope,
		),
	}
}

func (m *ReviewHubModel) Refresh() {
	if m.daily != nil {
		m.daily.reloadData()
		m.daily.clampCursor()
	}
	if m.weekly != nil {
		m.weekly.reloadData()
		m.weekly.clampCursor()
	}
	if m.activity != nil {
		m.activity.loadTasks()
		m.activity.clampCursor()
	}
	if m.metrics != nil {
		m.metrics.computeMetrics()
	}
	if m.roadmap != nil {
		m.roadmap.computeData()
	}
}

func reviewModulePresentation(module ReviewModule, project *models.Project) (PresentationID, bool) {
	switch module {
	case ReviewLanding, ReviewDaily:
		return ViewDailyReview, true
	case ReviewWeekly:
		return ViewWeeklyReview, true
	case ReviewActivity:
		if project != nil {
			return ViewProjectRecentActivity, true
		}
		return ViewRecentActivity, true
	case ReviewMetrics:
		return ViewMetricsDashboard, true
	case ReviewRoadmap:
		return ViewRoadmap, true
	default:
		return 0, false
	}
}
