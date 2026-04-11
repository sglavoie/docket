package tui

import "image/color"

// viewAccentMap maps ViewType to its category accent color.
// Unmapped views (pickers, editors, selectors) inherit from parent via stack walk.
var viewAccentMap map[ViewType]color.Color

// buildViewAccentMap initializes the accent map. Called from ApplyTheme so that
// palette overrides (if we add them later) are reflected on every theme change.
func buildViewAccentMap() {
	viewAccentMap = map[ViewType]color.Color{
		// Dashboard
		ViewDashboard: AccentDashboard,

		// Project Tree family (cyan)
		ViewProjectTree:          AccentProjectTree,
		ViewOutliner:             AccentProjectTree,
		ViewTemporaryOutliner:    AccentProjectTree,
		ViewCrossProjectOutliner: AccentProjectTree,
		ViewNotes:                AccentProjectTree,
		ViewGlobalNotes:          AccentProjectTree,

		// Focus family (green)
		ViewFocusView:        AccentFocus,
		ViewProjectFocusView: AccentFocus,
		ViewFocusBoard:       AccentFocus,

		// Priority family (orange)
		ViewPriorityView:    AccentPriority,
		ViewDueDateOverview: AccentPriority,

		// Review family (yellow)
		ViewDailyReview:          AccentReview,
		ViewWeeklyReview:         AccentReview,
		ViewGlobalInboxProcessor: AccentReview,
		ViewInboxProcessor:       AccentReview,

		// All Tasks (purple)
		ViewAllTasks: AccentAllTasks,

		// Metrics (teal)
		ViewMetricsDashboard: AccentMetrics,

		// Search / Command (magenta)
		ViewGlobalSearch:   AccentSearch,
		ViewCommandPalette: AccentSearch,
	}
}

// ViewAccentColor returns the accent color for a view type, or nil if unmapped.
func ViewAccentColor(vt ViewType) color.Color {
	if viewAccentMap == nil {
		return nil
	}
	return viewAccentMap[vt]
}
