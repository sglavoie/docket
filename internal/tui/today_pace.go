package tui

import "github.com/sglavoie/tt/internal/app"

// todayPaceState is the read-only completion activity injected by AppModel
// immediately before an Outliner paint. The Today open count remains sourced
// from WorkspacePulse so the comparison uses the same live planning count as
// the rest of the title chrome.
type todayPaceState struct {
	completionActivity app.CompletionActivity
}

func (s *todayPaceState) setCompletionActivity(activity app.CompletionActivity) {
	s.completionActivity = activity
}

// completionActivityAware is implemented only by presentations that can render
// the Today reference-class comparison.
type completionActivityAware interface {
	setCompletionActivity(app.CompletionActivity)
}

type todayPaceReadout struct {
	openCount    int
	medianPerDay float64
}
