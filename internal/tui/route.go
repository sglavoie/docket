package tui

import "image/color"

// RouteScope classifies the data owned by the active route or presentation.
type RouteScope int

const (
	ScopeApp RouteScope = iota
	ScopeProject
	ScopeTask
)

// AccentCategory names the workflow-stage accent painted by a presentation.
// Concrete colors are resolved from the active theme at render time.
type AccentCategory int

const (
	AccentCatNone AccentCategory = iota
	AccentCatDashboard
	AccentCatCapture
	AccentCatTriage
	AccentCatPlan
	AccentCatWork
	AccentCatExecute
	AccentCatReview
	AccentCatSupport
	AccentCatSearch
)

// presentationScope derives command scope directly from the active renderer.
// Navigation identity is owned exclusively by RouteID and its typed params.
func presentationScope(view PresentationID) RouteScope {
	switch view {
	case ViewDashboard, ViewProjectList,
		ViewDailyReview, ViewWeeklyReview, ViewRecentActivity, ViewGlobalSearch,
		ViewGlobalBlockers, ViewGlobalNotes, ViewGlobalInboxProcessor,
		ViewGlobalTagManager, ViewSettings, ViewWorkflowRules, ViewTaskTemplates,
		ViewTutor, ViewAsk, ViewScoreProfileManager, ViewMetricsDashboard,
		ViewCrossProjectOutliner, ViewCrossProjectKanban, ViewFocusBoard,
		ViewAllTasks, ViewRoadmap, ViewNotificationCenter, ViewMessageLog:
		return ScopeApp
	case ViewTaskComments, ViewTaskDecisions, ViewTaskChecklist, ViewTaskDetail,
		ViewTaskAttachments, ViewTaskActivity:
		return ScopeTask
	default:
		return ScopeProject
	}
}

// presentationAccentCategory assigns a visual accent without introducing a
// second navigation registry.
func presentationAccentCategory(view PresentationID) AccentCategory {
	switch view {
	case ViewDashboard:
		return AccentCatDashboard
	case ViewQuickCapture, ViewBulkCapture:
		return AccentCatCapture
	case ViewAllTasks, ViewGlobalInboxProcessor, ViewInboxProcessor,
		ViewCrossProjectOutliner, ViewCrossProjectKanban:
		return AccentCatTriage
	case ViewProjectFocusView, ViewDailyReview, ViewPriorityView, ViewRoadmap:
		return AccentCatPlan
	case ViewFocusBoard:
		return AccentCatWork
	case ViewOutliner, ViewTemporaryOutliner, ViewKanban:
		return AccentCatExecute
	case ViewWeeklyReview, ViewMetricsDashboard, ViewRecentActivity:
		return AccentCatReview
	case ViewGlobalBlockers, ViewGlobalNotes, ViewNotes, ViewWorkflowRules,
		ViewNotificationCenter:
		return AccentCatSupport
	case ViewGlobalSearch, ViewCommandPalette:
		return AccentCatSearch
	default:
		return AccentCatNone
	}
}

// accentColorForCategory resolves an accent category against the active theme.
func accentColorForCategory(category AccentCategory) color.Color {
	switch category {
	case AccentCatDashboard:
		return AccentDashboard
	case AccentCatCapture:
		return AccentCapture
	case AccentCatTriage:
		return AccentTriage
	case AccentCatPlan:
		return AccentPlan
	case AccentCatWork:
		return AccentWork
	case AccentCatExecute:
		return AccentExecute
	case AccentCatReview:
		return AccentReview
	case AccentCatSupport:
		return AccentSupport
	case AccentCatSearch:
		return AccentSearch
	default:
		return nil
	}
}

// allPresentations returns the contiguous renderer inventory.
func allPresentations() []PresentationID {
	presentations := make([]PresentationID, 0, int(ViewConflictResolution)+1)
	for presentation := ViewDashboard; presentation <= ViewConflictResolution; presentation++ {
		presentations = append(presentations, presentation)
	}
	return presentations
}
