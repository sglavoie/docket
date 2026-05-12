package tui

// pipelineStage returns the pipeline stage name for a given view type.
// Returns "" for Execute-stage views and views that don't belong to a named stage.
func pipelineStage(vt ViewType) string {
	switch vt {
	case ViewQuickCapture, ViewBulkCapture:
		return "Capture"
	case ViewAllTasks, ViewGlobalInboxProcessor, ViewCrossProjectOutliner, ViewCrossProjectKanban:
		return "Organize"
	case ViewFocusView, ViewDueDateOverview, ViewDailyReview, ViewRoadmap:
		return "Plan"
	case ViewFocusBoard:
		return "Work"
	case ViewWeeklyReview, ViewMetricsDashboard, ViewRecentActivity:
		return "Review"
	case ViewGlobalBlockers, ViewGlobalNotes, ViewWorkflowRules, ViewNotificationCenter:
		return "Support"
	}
	return ""
}

// pipelineNextHint returns a contextual "next step" hint string for a view.
// Returns "" for views that should show the default help text.
func pipelineNextHint(vt ViewType) string {
	switch vt {
	case ViewQuickCapture, ViewBulkCapture:
		return "Next: ctrl+i inbox | ;q backlog"
	case ViewAllTasks, ViewGlobalInboxProcessor, ViewCrossProjectOutliner, ViewCrossProjectKanban:
		return "Next: ;q backlog | ;w today"
	case ViewFocusView:
		return "ctrl+t today | ctrl+f weekly | Next: ;w today"
	case ViewDueDateOverview, ViewRoadmap:
		return "Next: ;w today | Enter project"
	case ViewDailyReview:
		return "ctrl+f weekly | Next: ;w today"
	case ViewFocusBoard:
		return "ctrl+t backlog | Next: ;r review | ;; hub"
	case ViewWeeklyReview:
		return "ctrl+f daily | Next: ;i capture | ;q backlog"
	case ViewMetricsDashboard, ViewRecentActivity:
		return "Next: ;i capture | ;q backlog"
	}
	return ""
}
