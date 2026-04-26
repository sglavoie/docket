package tui

// pipelineStage returns the pipeline stage name for a given view type.
// Returns "" for Execute-stage views and views that don't belong to a named stage.
func pipelineStage(vt ViewType) string {
	switch vt {
	case ViewQuickCapture, ViewBulkCapture:
		return "Capture"
	case ViewAllTasks, ViewGlobalInboxProcessor, ViewCrossProjectOutliner, ViewCrossProjectKanban:
		return "Organize"
	case ViewFocusView, ViewFocusBoard, ViewDueDateOverview, ViewDailyReview:
		return "Plan"
	case ViewWeeklyReview, ViewMetricsDashboard, ViewRecentActivity:
		return "Review"
	case ViewGlobalBlockers, ViewGlobalNotes, ViewWorkflowRules, ViewNotificationCenter:
		return "Support"
	}
	return ""
}

// pipelineNextHint returns a contextual "next step" hint string for a pipeline stage.
// Returns "" for stages that should show the default help text (Support, or no stage).
func pipelineNextHint(stage string) string {
	switch stage {
	case "Capture":
		return "Next: ctrl+i inbox | ;q backlog"
	case "Organize":
		return "Next: ;q backlog | ;f today"
	case "Plan":
		return "Next: Enter project | ;w review"
	case "Review":
		return "Next: ;i capture | ;q backlog"
	}
	return ""
}
