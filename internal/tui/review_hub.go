package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ShowReviewHubMsg opens the Review destination at a named module. It is the
// typed entry point used by startup routing and direct Review deep links.
type ShowReviewHubMsg struct {
	Module  ReviewModule
	Project *models.Project
}

var reviewModuleNames = [...]string{
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
// owns their lifecycle, scope, and module return semantics.
type ReviewHubModel struct {
	BaseView
	runtime       *config.RuntimeState
	configManager *config.Manager
	wire          func(View)

	params  ReviewParams
	project *models.Project

	// Daily Review and Activity are Outliner computed modes, not hub submodels,
	// so the hub only owns the remaining direct review tools.
	weekly         *WeeklyReviewModel
	weeklyScopeID  string
	metrics        *MetricsDashboardModel
	metricsScopeID string
	roadmap        *RoadmapModel
	roadmapScopeID string

	queueIDs     map[TaskIdentity]bool
	dailyPlanIDs map[TaskIdentity]bool
}

func NewReviewHubModel(runtime *config.RuntimeState, manager *config.Manager, wire func(View)) *ReviewHubModel {
	return &ReviewHubModel{
		runtime:       runtime,
		configManager: manager,
		wire:          wire,
		params:        ReviewParams{Module: ReviewDaily},
	}
}

func (m *ReviewHubModel) Init() tea.Cmd {
	if active := m.ActiveModuleView(); active != nil {
		return active.Init()
	}
	return nil
}

func (m *ReviewHubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *ReviewHubModel) View() tea.View {
	if active := m.ActiveModuleView(); active != nil {
		return active.View()
	}
	return tea.NewView("")
}

// Activate switches the hub mode and lazily creates the requested legacy model.
// A nil project selects app scope; project-aware modules are recreated only
// when their requested scope changes, preserving in-module state otherwise.
func (m *ReviewHubModel) Activate(module ReviewModule, project *models.Project) error {
	return m.ActivateParams(ReviewParams{Module: module}, project)
}

// ActivateParams switches the hub using canonical typed route parameters. The
// project pointer remains a retained adapter for project-aware Metrics and
// Roadmap models; Weekly Review consumes the route's exact project set.
func (m *ReviewHubModel) ActivateParams(params ReviewParams, project *models.Project) error {
	route, err := NewRoute(SecondaryTool, params)
	if err != nil {
		return err
	}
	params = route.Params().(ReviewParams)
	if params.Module > ReviewRoadmap {
		return fmt.Errorf("unknown review module: %d", params.Module)
	}
	m.params = params
	m.project = project

	scopeID := reviewScopeID(project)
	var view View
	switch params.Module {
	case ReviewDaily, ReviewActivity:
		// Daily Review and Activity are Outliner modes.
		return nil
	case ReviewWeekly:
		weeklyScopeID := projectSetScopeID(params.ProjectSet)
		if m.weekly == nil || m.weeklyScopeID != weeklyScopeID {
			m.weekly = NewWeeklyReviewModelForProjectSet(m.runtime, m.configManager, params.ProjectSet)
			m.weeklyScopeID = weeklyScopeID
			m.wireModule(m.weekly)
		}
		m.weekly.queueIDs, m.weekly.dailyPlanIDs = m.queueIDs, m.dailyPlanIDs
		view = m.weekly
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

func projectSetScopeID(scope ProjectSetScope) string {
	scope = canonicalProjectSetScope(scope)
	if !scope.Explicit {
		return "app"
	}
	return "set:" + strings.Join(scope.ProjectIDs(), "\x1f")
}

func (m *ReviewHubModel) wireModule(view View) {
	if m.wire != nil {
		m.wire(view)
	}
}

func (m *ReviewHubModel) replaceActiveModule(view View) {
	switch m.params.Module {
	case ReviewDaily, ReviewActivity:
	case ReviewWeekly:
		m.weekly, _ = view.(*WeeklyReviewModel)
	case ReviewMetrics:
		m.metrics, _ = view.(*MetricsDashboardModel)
	case ReviewRoadmap:
		m.roadmap, _ = view.(*RoadmapModel)
	}
}

func (m *ReviewHubModel) ActiveModuleView() View {
	switch m.params.Module {
	case ReviewDaily, ReviewActivity:
		return nil
	case ReviewWeekly:
		if m.weekly != nil {
			return m.weekly
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

func (m *ReviewHubModel) Module() ReviewModule { return m.params.Module }

func (m *ReviewHubModel) Params() ReviewParams { return m.params }

func (m *ReviewHubModel) ProjectScope() *models.Project { return m.project }

func (m *ReviewHubModel) SetPlanningIndicators(queueIDs, dailyPlanIDs map[TaskIdentity]bool) {
	m.queueIDs, m.dailyPlanIDs = queueIDs, dailyPlanIDs
	if m.weekly != nil {
		m.weekly.queueIDs, m.weekly.dailyPlanIDs = queueIDs, dailyPlanIDs
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
	views := make([]View, 0, 3)
	if m.weekly != nil {
		views = append(views, m.weekly)
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
	return nil
}

func (m *ReviewHubModel) StatusBarLabel() string { return m.params.Module.String() }

func (m *ReviewHubModel) SupportsTaskUndoRedo() bool {
	return m.params.Module == ReviewWeekly
}

func (m *ReviewHubModel) CommandContext() CommandContext {
	if view, ok := reviewModulePresentation(m.params.Module, m.project); ok {
		ctx := PresentationCommandContext(view)
		if m.project != nil {
			ctx.Scope = ScopeProject
			delete(ctx.Capabilities, CapabilityAppScope)
			ctx.Capabilities.Add(CapabilityProjectScope)
			ctx.FocusedProjectID = m.project.ID
		}
		ctx.ScopeProjectIDs = m.ScopeProjectIDs()
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

// ScopeProjectIDs returns the exact project set captured by Weekly Review's
// route, or the retained project/app scope for the other review tools.
func (m *ReviewHubModel) ScopeProjectIDs() []string {
	if m.project != nil {
		return []string{m.project.ID}
	}
	projects := projectsInProjectSet(m.runtime.ActiveProjects(), m.params.ProjectSet)
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return normalizedProjectIDs(ids)
}

func (m *ReviewHubModel) Refresh() {
	if m.weekly != nil {
		m.weekly.reloadData()
		m.weekly.clampCursor()
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
	case ReviewWeekly:
		return ViewWeeklyReview, true
	case ReviewMetrics:
		return ViewMetricsDashboard, true
	case ReviewRoadmap:
		return ViewRoadmap, true
	default:
		return 0, false
	}
}
