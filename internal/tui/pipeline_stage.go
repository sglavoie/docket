package tui

// pipelineStage returns the pipeline stage name for a given view type.
// Returns "" for legacy Execute-accent views and views outside the named pipeline stages.
func pipelineStage(vt PresentationID) string {
	switch vt {
	case ViewQuickCapture, ViewBulkCapture:
		return "Capture"
	case ViewGlobalInboxProcessor, ViewCrossProjectOutliner:
		return "Triage"
	case ViewDailyReview, ViewRoadmap:
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
		return "plan it today (w) or process it from Manage > Inbox"
	}
	return ""
}

// pipelineNextHintOnboarding returns a special next-step hint for empty-state views.
// Called when no tasks exist yet so beginners get an actionable prompt.
func pipelineNextHintOnboarding(vt PresentationID) string {
	if vt == ViewCrossProjectOutliner {
		return "Next: capture a task (Ctrl+N)"
	}
	return ""
}

// pipelineNextHint returns a contextual "next step" hint string for a view.
// Returns "" for views that should show the default help text.
func pipelineNextHint(vt PresentationID) string {
	switch vt {
	case ViewQuickCapture, ViewBulkCapture:
		return "Next: Manage Inbox | Ctrl+G Planner"
	case ViewGlobalInboxProcessor, ViewCrossProjectOutliner:
		return "Next: Planner Next | Planner Today"
	case ViewRoadmap:
		return "Next: Planner Today | Enter project"
	case ViewDailyReview:
		return "ctrl+f weekly | Next: Planner Today"
	case ViewFocusBoard:
		return "ctrl+t backlog | Next: Ctrl+G Review"
	case ViewWeeklyReview:
		return "ctrl+f daily | Next: Ctrl+N capture | Planner Next"
	case ViewMetricsDashboard, ViewRecentActivity:
		return "Next: Ctrl+N capture | Planner Next"
	default:
	}
	return ""
}
