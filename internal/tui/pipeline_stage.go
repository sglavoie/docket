package tui

// pipelineStage returns the pipeline stage name for a given view type.
// Returns "" for legacy Execute-accent views and views outside the named pipeline stages.
func pipelineStage(vt PresentationID) string {
	switch vt {
	case ViewTaskComposer:
		return "Capture"
	case ViewCrossProjectOutliner:
		return "Triage"
	case ViewDailyReview, ViewRoadmap:
		return "Plan"
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
		return "plan it Today (t) or triage it in Inbox"
	}
	return ""
}

// pipelineNextHintOnboarding returns a special next-step hint for empty-state views.
// Called when no tasks exist yet so beginners get an actionable prompt.
func pipelineNextHintOnboarding(vt PresentationID) string {
	if vt == ViewCrossProjectOutliner {
		return "Next: capture a task (a)"
	}
	return ""
}

// pipelineNextHint returns a contextual "next step" hint string for a view.
// Returns "" for views that should show the default help text.
func pipelineNextHint(vt PresentationID) string {
	switch vt {
	case ViewTaskComposer:
		return "Next: Inbox triage | Tab cycles modes"
	case ViewCrossProjectOutliner:
		return "Next: Tab cycles modes | t plans Today"
	case ViewRoadmap:
		return "Next: All for Today | Enter project"
	case ViewDailyReview:
		return "ctrl+f weekly | Next: All for Today"
	case ViewWeeklyReview:
		return "Next: Esc back | a capture from All"
	case ViewMetricsDashboard, ViewRecentActivity:
		return "Next: Esc back | a capture from All"
	default:
	}
	return ""
}
