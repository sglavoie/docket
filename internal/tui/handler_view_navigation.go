package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// handleNavigateToView dispatches a NavigateToViewMsg to the appropriate Show message.
func (m *AppModel) handleNavigateToView(msg NavigateToViewMsg) (bool, tea.Cmd) { //nolint:gocyclo
	project := m.currentProject

	switch msg.PresentationID {
	case ViewGlobalSearch:
		// Search is now the palette itself; the alias closes the current
		// palette and opens a fresh typed Palette overlay.
		return m.handleSearchMessages(ShowCommandPaletteMsg{})
	case ViewNotesSearch:
		params := ManageParams{Module: ManageNotes, Scope: AppEntityScope()}
		if project != nil {
			params.Scope = ProjectEntityScope(project.ID)
		}
		activateCmd := m.activateManageHub(params)
		if project == nil {
			return true, activateCmd
		}
		search := NewNotesSearchModel(project)
		m.showView(ViewNotesSearch, search)
		return true, tea.Batch(activateCmd, search.Init())
	case ViewDailyReview:
		return m.handleViewMessages(ShowDailyReviewMsg{})
	case ViewWeeklyReview:
		return m.handleViewMessages(ShowWeeklyReviewMsg{})
	case ViewRecentActivity:
		return m.handleViewMessages(ShowRecentActivityMsg{})
	case ViewProjectRecentActivity:
		if project != nil {
			return m.handleViewMessages(ShowProjectRecentActivityMsg{Project: project})
		}
	case ViewCrossProjectOutliner:
		return m.handleViewMessages(ShowGlobalOutlinerMsg{Options: msg.OutlinerOptions})
	case ViewMetricsDashboard:
		return m.handleViewMessages(ShowMetricsDashboardMsg{})
	case ViewRoadmap:
		return m.handleViewMessages(ShowRoadmapMsg{})
	case ViewBlockers:
		if project != nil {
			if err := storage.LoadProject(project); err != nil {
				logging.Warn("failed to load project %s: %v", project.Name, err)
			}
			return m.handleViewMessages(ShowBlockersMsg{Project: project})
		}
		return m.handleViewMessages(ShowBlockersMsg{})
	case ViewGlobalInboxProcessor:
		return m.handleInboxMessages(ShowGlobalInboxProcessorMsg{})
	case ViewGlobalTagManager:
		return m.handleTagMessages(ShowGlobalTagManagerMsg{})
	case ViewGlobalNotes:
		return m.handleViewMessages(ShowGlobalNotesMsg{})
	case ViewGlobalBlockers:
		return m.handleViewMessages(ShowGlobalBlockersMsg{})
	case ViewInboxProcessor:
		return m.handleInboxMessages(ShowInboxProcessorMsg{Project: project})
	case ViewTagManager:
		return m.handleTagMessages(ShowTagManagerMsg{Project: project})
	case ViewNotificationCenter:
		return m.handleViewMessages(ShowNotificationCenterMsg{})
	case ViewMessageLog:
		return m.handleViewMessages(ShowMessageLogMsg{})
	case ViewNotes:
		if project != nil {
			return m.handleViewMessages(ShowNotesMsg{Project: project})
		}
		return m.handleViewMessages(ShowNotesMsg{})
	case ViewSettings:
		return m.handleViewMessages(SettingsMsg{})
	case ViewWorkflowRules:
		return m.handleViewMessages(ShowWorkflowRulesMsg{})
	case ViewScoreProfileManager:
		projectID := ""
		if project != nil {
			projectID = project.ID
		}
		return m.handleViewMessages(ShowScoreProfileManagerMsg{ProjectID: projectID})
	case ViewTaskTemplates:
		return m.handleViewMessages(ShowTaskTemplatesMsg{})
	case ViewOutliner:
		if project != nil {
			if err := storage.LoadProject(project); err != nil {
				logging.Warn("failed to load project %s: %v", project.Name, err)
			}
			return m.handleViewMessages(ShowOutlinerMsg{Project: project, Options: msg.OutlinerOptions})
		}
	default:
	}
	return true, nil
}
