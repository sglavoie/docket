package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// reevaluateAlerts loads all active projects and runs the alert engine,
// then applies the dismissal post-filter pipeline.
func (m *AppModel) reevaluateAlerts() {
	cfg := m.buildAlertConfig()
	activeProjects := m.config.GetActiveProjects()

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

	// Post-filter pipeline: apply dismissals
	dismissals := m.config.GetAlertDismissals()
	if len(dismissals) == 0 {
		m.dismissedSet = nil
		m.proactiveAlerts = rawSnapshot
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

	// Persist if dismissal list changed
	if len(surviving) != len(dismissals) {
		m.config.SetAlertDismissals(surviving)
		_ = m.configManager.Save(m.config)
	}

	m.dismissedSet = dismissedSet
	m.proactiveAlerts = alerts.FilterDismissed(rawSnapshot, dismissedSet)
}

// refreshNotificationCenter recreates the notification center if it's the current view,
// preserving the cursor position and showDismissed toggle state.
func (m *AppModel) refreshNotificationCenter() {
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

// buildAlertConfig creates an AlertConfig from the current config state.
func (m *AppModel) buildAlertConfig() alerts.AlertConfig {
	return alerts.AlertConfig{
		DailyReviewHour: m.config.GetAlertDailyReviewHour(),
		WeeklyReviewDay: m.config.GetAlertWeeklyReviewDay(),

		LastDailyReviewAt:  m.config.GetLastDailyReviewAt(),
		LastWeeklyReviewAt: m.config.GetLastWeeklyReviewAt(),

		DailyReviewEnabled:   m.config.GetAlertDailyReviewEnabled(),
		WeeklyReviewEnabled:  m.config.GetAlertWeeklyReviewEnabled(),
		OverdueEnabled:       m.config.GetAlertOverdueEnabled(),
		DueSoonEnabled:       m.config.GetAlertDueSoonEnabled(),
		StaleEnabled:         m.config.GetAlertStaleEnabled(),
		InboxEnabled:         m.config.GetAlertInboxEnabled(),
		BlockedEnabled:       m.config.GetAlertBlockedEnabled(),
		ProjectHealthEnabled: m.config.GetAlertProjectHealthEnabled(),

		DueSoonDays:      m.config.GetAlertDueSoonDays(),
		StaleDays:        m.config.GetAlertStaleDays(),
		InboxThreshold:   m.config.GetAlertInboxThreshold(),
		BlockedDays:      m.config.GetAlertBlockedDays(),
		ProjectStaleDays: m.config.GetAlertProjectStaleDays(),
	}
}

// alertCount returns the current proactive alert count, or 0 if not evaluated.
func (m *AppModel) alertCount() int {
	if m.proactiveAlerts == nil {
		return 0
	}
	return m.proactiveAlerts.TotalCount
}
