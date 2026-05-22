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
	default:
	}
	return ""
}

// captureNextHint returns the next-step hint shown after a successful first-task quick-capture.
// taskCountBefore is the number of tasks in the project before the capture; returns "" otherwise.
func captureNextHint(taskCountBefore int) string {
	if taskCountBefore == 0 {
		return "plan it today (;w) or process inbox (ctrl+i)"
	}
	return ""
}

// pipelineNextHintOnboarding returns a special next-step hint for empty-state views.
// Called when no tasks exist yet so beginners get an actionable prompt.
func pipelineNextHintOnboarding(vt ViewType) string {
	if vt == ViewAllTasks {
		return "Next: capture a task (ctrl+i) or open Quick Capture (:)"
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
	default:
	}
	return ""
}
