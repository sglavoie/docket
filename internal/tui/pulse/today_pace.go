package pulse

import "github.com/sglavoie/docket/internal/app"

// TodayPaceState is the read-only completion activity injected by AppModel
// immediately before an Outliner paint. The Today open count remains sourced
// from WorkspacePulse so the comparison uses the same live planning count as
// the rest of the title chrome.
type TodayPaceState struct {
	CompletionActivity app.CompletionActivity
}

func (s *TodayPaceState) SetCompletionActivity(activity app.CompletionActivity) {
	s.CompletionActivity = activity
}

// CompletionActivityAware is implemented only by presentations that can render
// the Today reference-class comparison.
type CompletionActivityAware interface {
	SetCompletionActivity(app.CompletionActivity)
}

type TodayPaceReadout struct {
	OpenCount    int
	MedianPerDay float64
}
