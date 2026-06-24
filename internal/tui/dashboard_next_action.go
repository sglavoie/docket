package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/config"
)

// renderDashboardRecommendationStrip composes the Next Action strip shown
// above the Dashboard project list. Returns the rendered string plus the
// number of chrome lines it consumes so the caller can shrink the project
// list's visible-row budget accordingly. The strip is suppressed when the
// dashboard is browsing archived projects or when the underlying renderer
// drops the row at narrow widths.
func (m *DashboardModel) renderDashboardRecommendationStrip() (string, int) {
	if m.showArchived {
		return "", 0
	}
	rec := m.computeDashboardRecommendation()
	strip := renderNextActionStrip(NextActionStripOpts{
		Action:      rec,
		Width:       m.width,
		HasProjects: len(m.projects) > 0,
		OpenLoops:   m.openLoops,
	})
	if strip == "" {
		return "", 0
	}
	return strip + "\n\n", 2
}

// computeDashboardRecommendation runs the shared recommendation engine using
// the dashboard's loaded project slice and the persisted daily plan. The
// engine is deterministic and side-effect free (session-3 contract); the
// dashboard recomputes on every render so reasons stay in sync with the
// clock (overdue / recently-active / stale rotate as time advances).
func (m *DashboardModel) computeDashboardRecommendation() RecommendedAction {
	var plan []config.DailyPlanItem
	if m.config != nil {
		plan = m.config.GetDailyPlan()
	}
	return Recommend(RecommendationEngineInput{
		Projects:  m.projects,
		DailyPlan: plan,
		Now:       time.Now(),
	})
}
