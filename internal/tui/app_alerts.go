package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// reevaluateAlerts loads all active projects and runs the alert engine,
// then applies the dismissal post-filter pipeline.
func (m *AppModel) reevaluateAlerts() {
	cfg := m.buildAlertConfig()
	activeProjects := m.activeProjects()

	var projectData []alerts.ProjectData
	for _, p := range activeProjects {
		if err := storage.LoadProject(p); err != nil {
			continue
		}
		projectData = append(projectData, alerts.ProjectData{
			Project: p,
			Tasks:   p.Tasks,
		})
	}

	rawSnapshot := alerts.Evaluate(cfg, projectData)
	m.rawAlerts = rawSnapshot

	// Post-filter pipeline: apply dismissals. Dismissals are planning-owned
	// bookkeeping, read from the cached planning domain (not config.Config).
	dismissals := m.runtime.Planning().GetAlertDismissals()
	if len(dismissals) == 0 {
		m.dismissedSet = nil
		m.proactiveAlerts = rawSnapshot
		if m.dashboard != nil {
			counts, _ := loadAttentionCounts(m.runtime, m.focusQueueIDs, m.dailyPlanIDs)
			m.dashboard.SetPipelineCounts(counts)
			m.dashboard.SetAttentionAlerts(m.proactiveAlerts)
		}
		return
	}

	now := time.Now()

	// Build set of active alert keys from the raw snapshot for GC
	activeKeys := make(map[string]bool)
	if rawSnapshot != nil {
		for _, g := range rawSnapshot.Groups {
			for _, a := range g.Alerts {
				activeKeys[a.DismissalKey()] = true
			}
		}
	}

	// Filter dismissals: expire time-based ones, GC condition-resolved ones
	var surviving []config.AlertDismissal
	dismissedSet := make(map[string]bool)
	for _, d := range dismissals {
		if alerts.ShouldExpireDismissal(d.Category, d.DismissedAt, now) {
			continue
		}
		if !activeKeys[d.Key] {
			continue // condition resolved, GC this dismissal
		}
		surviving = append(surviving, d)
		dismissedSet[d.Key] = true
	}

	// Persist if dismissal list changed. Dismissals are planning-owned, so this
	// commits only the planning domain under a checked commit rather than saving
	// the whole config.
	if len(surviving) != len(dismissals) {
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.SetAlertDismissals(surviving)
		}); err != nil {
			logging.Warn("failed to save alert dismissals: %v", err)
		}
	}

	m.dismissedSet = dismissedSet
	m.proactiveAlerts = alerts.FilterDismissed(rawSnapshot, dismissedSet)
	if m.dashboard != nil {
		counts, _ := loadAttentionCounts(m.runtime, m.focusQueueIDs, m.dailyPlanIDs)
		m.dashboard.SetPipelineCounts(counts)
		m.dashboard.SetAttentionAlerts(m.proactiveAlerts)
	}
}

// refreshNotificationCenter recreates the notification center if it's the current view,
// preserving the cursor position and showDismissed toggle state.
func (m *AppModel) refreshNotificationCenter() {
	if hub, params, ok := m.manageHubPrimary(); ok && params.Module == ManageNotifications && params.NotificationMode == NotificationAlerts {
		hub.SetNotificationData(m.proactiveAlerts, m.rawAlerts, m.dismissedSet)
		m.refreshManageNotificationCenter()
		return
	}
	if m.currentView() != ViewNotificationCenter {
		return
	}
	var showDismissed bool
	var prevCursor int
	if v := m.views.Get(ViewNotificationCenter); v != nil {
		if nc, ok := v.(*NotificationCenterModel); ok {
			showDismissed = nc.showDismissed
			prevCursor = nc.cursor
		}
	}
	nc := NewNotificationCenterModel(m.proactiveAlerts, m.rawAlerts, m.dismissedSet)
	nc.showDismissed = showDismissed
	if showDismissed {
		nc.rebuildForShowDismissed()
	}
	nc.cursor = prevCursor
	nc.clampCursor()
	nc.SetSize(m.width, m.height-2)
	m.views.Register(ViewNotificationCenter, nc)
}

// buildAlertConfig creates an AlertConfig from the cached configuration domains:
// alert enablement/threshold policy is Settings-owned, while the review
// completion timestamps are Planning-owned. Neither is read through config.Config.
func (m *AppModel) buildAlertConfig() alerts.AlertConfig {
	s := m.runtime.Settings()
	pl := m.runtime.Planning()
	return alerts.AlertConfig{
		DailyReviewHour: s.GetAlertDailyReviewHour(),
		WeeklyReviewDay: s.GetAlertWeeklyReviewDay(),

		LastDailyReviewAt:  pl.GetLastDailyReviewAt(),
		LastWeeklyReviewAt: pl.GetLastWeeklyReviewAt(),

		DailyReviewEnabled:   s.GetAlertDailyReviewEnabled(),
		WeeklyReviewEnabled:  s.GetAlertWeeklyReviewEnabled(),
		OverdueEnabled:       s.GetAlertOverdueEnabled(),
		DueSoonEnabled:       s.GetAlertDueSoonEnabled(),
		StaleEnabled:         s.GetAlertStaleEnabled(),
		InboxEnabled:         s.GetAlertInboxEnabled(),
		BlockedEnabled:       s.GetAlertBlockedEnabled(),
		ProjectHealthEnabled: s.GetAlertProjectHealthEnabled(),

		DueSoonDays:      s.GetAlertDueSoonDays(),
		StaleDays:        s.GetAlertStaleDays(),
		InboxThreshold:   s.GetAlertInboxThreshold(),
		BlockedDays:      s.GetAlertBlockedDays(),
		ProjectStaleDays: s.GetAlertProjectStaleDays(),
	}
}

// alertCount returns the current proactive alert count, or 0 if not evaluated.
func (m *AppModel) alertCount() int {
	if m.proactiveAlerts == nil {
		return 0
	}
	return m.proactiveAlerts.TotalCount
}
