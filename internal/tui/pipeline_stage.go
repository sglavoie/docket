package tui

// pipelineStage returns the pipeline stage name for a given view type.
// Returns "" for legacy Execute-accent views and views outside the named pipeline stages.
func pipelineStage(vt PresentationID) string {
	switch vt {
	case ViewTaskComposer:
		return "Capture"
	case ViewCrossProjectOutliner:
		return "Triage"
	case ViewGlobalNotes:
		return "Support"
	default:
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
	default:
	}
	return ""
}
