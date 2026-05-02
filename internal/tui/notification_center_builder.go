package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/alerts"
)

// buildItems flattens alert groups into navigable items, ordered by severity (urgent first).
func (m *NotificationCenterModel) buildItems() {
	m.items = nil
	m.sectionStarts = nil
	snap := m.activeSnapshot()
	if snap == nil || len(snap.Groups) == 0 {
		return
	}

	// Sort groups by max severity descending
	groups := make([]alerts.AlertGroup, len(snap.Groups))
	copy(groups, snap.Groups)
	sort.SliceStable(groups, func(i, j int) bool {
		return maxSeverity(groups[i].Alerts) > maxSeverity(groups[j].Alerts)
	})

	for gi, group := range groups {
		// Group header
		m.items = append(m.items, alertCenterItem{
			isHeader: true,
			group:    gi,
			alert:    -1,
			label:    group.Label,
		})
		// Record the first selectable item index for this section
		m.sectionStarts = append(m.sectionStarts, len(m.items))
		// Individual alerts
		for ai, a := range group.Alerts {
			m.items = append(m.items, alertCenterItem{
				isHeader:  false,
				group:     gi,
				alert:     ai,
				label:     a.Title,
				details:   a.Details,
				dismissed: m.showDismissed && m.dismissedSet[a.DismissalKey()],
			})
		}
	}
}

// rebuildForShowDismissed switches to showing the raw snapshot and rebuilds items.
func (m *NotificationCenterModel) rebuildForShowDismissed() {
	m.buildItems()
	m.clampCursor()
}

// maxSeverity returns the maximum severity value in a slice of alerts.
func maxSeverity(as []alerts.Alert) int {
	max := 0
	for _, a := range as {
		if a.Severity > max {
			max = a.Severity
		}
	}
	return max
}

// sortedGroups returns a copy of the active snapshot's groups sorted by severity.
func (m *NotificationCenterModel) sortedGroups() []alerts.AlertGroup {
	snap := m.activeSnapshot()
	if snap == nil {
		return nil
	}
	groups := make([]alerts.AlertGroup, len(snap.Groups))
	copy(groups, snap.Groups)
	sort.SliceStable(groups, func(i, j int) bool {
		return maxSeverity(groups[i].Alerts) > maxSeverity(groups[j].Alerts)
	})
	return groups
}
