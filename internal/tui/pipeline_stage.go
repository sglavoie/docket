package tui

import "github.com/sglavoie/tt/internal/tui/presentation"

// pipelineStage returns the pipeline stage name for a given view type.
// Returns "" for legacy Execute-accent views and views outside the named pipeline stages.
func pipelineStage(vt presentation.PresentationID) string {
	switch vt {
	case presentation.ViewTaskComposer:
		return "Capture"
	case presentation.ViewCrossProjectOutliner:
		return "Triage"
	case presentation.ViewGlobalNotes:
		return "Support"
	default:
	}
	return ""
}

// pipelineNextHint returns a contextual "next step" hint string for a view.
// Returns "" for views that should show the default help text.
func pipelineNextHint(vt presentation.PresentationID) string {
	switch vt {
	case presentation.ViewTaskComposer:
		return "Next: Inbox triage | Tab cycles modes"
	case presentation.ViewCrossProjectOutliner:
		return "Next: Tab cycles modes | t plans Today"
	default:
	}
	return ""
}
