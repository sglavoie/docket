package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/alerts"
)

// alertCenterItem is a flattened item for cursor-based navigation.
type alertCenterItem struct {
	isHeader  bool   // true for group headers (not selectable)
	group     int    // index into the active snapshot's Groups
	alert     int    // index into group.Alerts (-1 for headers)
	label     string // pre-rendered display text (title only for alerts)
	details   string // reasoning/context appended after title
	dismissed bool   // true if this alert is in the dismissed set (only when showDismissed)
}

// NotificationCenterModel shows proactive alerts grouped by category.
type NotificationCenterModel struct {
	BaseView
	InteractiveViewState

	snapshot      *alerts.AlertSnapshot // filtered snapshot (what's normally shown)
	rawSnapshot   *alerts.AlertSnapshot // unfiltered snapshot (includes dismissed)
	dismissedSet  map[string]bool       // keys of currently dismissed alerts
	showDismissed bool                  // true = show all alerts including dismissed
	items         []alertCenterItem     // flattened items for navigation
	sectionStarts []int                 // cursor indices of first selectable item per section
	expandedItem  int                   // index of expanded item, -1 = none
}

// NewNotificationCenterModel creates an alert center from the current snapshot.
func NewNotificationCenterModel(filtered, raw *alerts.AlertSnapshot, dismissedSet map[string]bool) *NotificationCenterModel {
	m := &NotificationCenterModel{
		snapshot:     filtered,
		rawSnapshot:  raw,
		dismissedSet: dismissedSet,
		expandedItem: -1,
	}
	m.buildItems()
	// Position cursor on first selectable item
	for i, item := range m.items {
		if !item.isHeader {
			m.cursor = i
			break
		}
	}
	return m
}

// Init implements tea.Model.
func (m *NotificationCenterModel) Init() tea.Cmd {
	return nil
}

// activeSnapshot returns the snapshot to use for rendering: raw when showing
// dismissed alerts, filtered otherwise.
func (m *NotificationCenterModel) activeSnapshot() *alerts.AlertSnapshot {
	if m.showDismissed && m.rawSnapshot != nil {
		return m.rawSnapshot
	}
	return m.snapshot
}
