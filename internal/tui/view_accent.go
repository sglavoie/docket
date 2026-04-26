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

		// Capture stage (coral/salmon)
		ViewQuickCapture: AccentCapture,
		ViewBulkCapture:  AccentCapture,

		// Organize stage (sky blue)
		ViewAllTasks:             AccentOrganize,
		ViewGlobalInboxProcessor: AccentOrganize,
		ViewInboxProcessor:       AccentOrganize,
		ViewCrossProjectOutliner: AccentOrganize,
		ViewCrossProjectKanban:   AccentOrganize,

		// Plan stage (green)
		ViewFocusView:        AccentPlan,
		ViewProjectFocusView: AccentPlan,
		ViewFocusBoard:       AccentPlan,
		ViewDueDateOverview:  AccentPlan,
		ViewDailyReview:      AccentPlan,
		ViewPriorityView:     AccentPlan,

		// Execute stage (cyan)
		ViewProjectTree:       AccentExecute,
		ViewOutliner:          AccentExecute,
		ViewTemporaryOutliner: AccentExecute,
		ViewKanban:            AccentExecute,

		// Review stage (yellow)
		ViewWeeklyReview:     AccentReview,
		ViewMetricsDashboard: AccentReview,
		ViewRecentActivity:   AccentReview,

		// Support stage (muted gray)
		ViewGlobalBlockers:     AccentSupport,
		ViewGlobalNotes:        AccentSupport,
		ViewNotes:              AccentSupport,
		ViewWorkflowRules:      AccentSupport,
		ViewNotificationCenter: AccentSupport,

		// Search / Command (magenta)
		ViewGlobalSearch:   AccentSearch,
		ViewCommandPalette: AccentSearch,

		// Pipeline Hub (neutral — spans all stages)
		ViewPipelineHub: AccentDashboard,
	}
}

// ViewAccentColor returns the accent color for a view type, or nil if unmapped.
func ViewAccentColor(vt ViewType) color.Color {
	if viewAccentMap == nil {
		return nil
	}
	return viewAccentMap[vt]
}
