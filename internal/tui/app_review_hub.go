package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) ensureReviewHub() *ReviewHubModel {
	if m.reviewHub != nil {
		return m.reviewHub
	}
	runtime := m.runtime
	if runtime == nil {
		runtime = config.EmptyRuntimeState("")
	}
	m.reviewHub = NewReviewHubModel(runtime, m.configManager, m.wireSecondaryCoordinators)
	m.reviewHub.SetPlanningIndicators(m.focusQueueIDs, m.dailyPlanIDs)
	m.reviewHub.SetSize(m.width, m.height-2)
	return m.reviewHub
}

// activateReviewHub installs one named module as a mode of the typed Review
// destination. The route remains authoritative while PresentationID identifies
// its implementation renderer.
func (m *AppModel) activateReviewHub(module ReviewModule, project *models.Project) tea.Cmd {
	return m.activateReviewHubWithProjectSet(module, project, ProjectSetScope{})
}

func (m *AppModel) activateReviewHubWithProjectSet(
	module ReviewModule,
	project *models.Project,
	projectSet ProjectSetScope,
) tea.Cmd {
	route, err := NewRoute(SecondaryTool, ReviewParams{Module: module, ProjectSet: projectSet})
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	params := route.Params().(ReviewParams)

	hub := m.ensureReviewHub()
	hub.SetPlanningIndicators(m.focusQueueIDs, m.dailyPlanIDs)
	if err := hub.ActivateParams(params, project); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}

	active := hub.ActiveModuleView()
	if active != nil {
		active.SetSize(m.width, m.height-2)
		m.wireSecondaryCoordinators(active)
		m.registerReviewModulePresentation(params.Module, project, active)
	}
	m.metricsDashboard = hub.metrics
	m.roadmap = hub.roadmap

	if err := m.navigation.PushChild(route, m.currentFocusTarget()); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}
	if m.startupRoute != nil && m.startupRoute.ID() == SecondaryTool {
		m.startupRoute = nil
		m.startupView = ViewCrossProjectOutliner
	}
	m.setAccentOnView(hub)
	if active != nil {
		m.setAccentOnView(active)
		return active.Init()
	}
	return nil
}

func (m *AppModel) registerReviewModulePresentation(module ReviewModule, project *models.Project, view View) {
	if m.views == nil || view == nil {
		return
	}
	presentation, ok := reviewModulePresentation(module, project)
	if ok {
		m.views.Register(presentation, view)
	}
}
