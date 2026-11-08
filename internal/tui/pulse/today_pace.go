package pulse

import (
	"strconv"

	"github.com/sglavoie/docket/internal/app"
)

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

// TodayPaceReadout is what Today's middle region says about the day: the open
// load, what is already done, and one comparison against the workspace's own
// 30-day history. The comparison is the reader's own record rather than a
// deficit: a best day says so, an ordinary one names the usual rate, and an
// occasional user, whose all-days median is 0, is measured against the days
// they actually worked rather than told they usually finish nothing.
type TodayPaceReadout struct {
	OpenCount int
	DoneToday int
	// MedianPerDay is the median over all 30 window days, idle ones included.
	MedianPerDay float64
	// ActiveDayMedian is the median over the window's earlier days that
	// finished anything; it speaks only when MedianPerDay is 0.
	ActiveDayMedian float64
	// BestPriorDay is the most any earlier window day finished.
	BestPriorDay int
}

// NewTodayPaceReadout reads the comparison's inputs off one activity snapshot.
func NewTodayPaceReadout(open int, activity app.CompletionActivity) TodayPaceReadout {
	return TodayPaceReadout{
		OpenCount:       open,
		DoneToday:       activity.CompletedToday(),
		MedianPerDay:    activity.MedianPerDay(),
		ActiveDayMedian: activity.ActiveDayMedian(),
		BestPriorDay:    activity.BestPriorDay(),
	}
}

// comparison is the readout's one judgement in its full and short spellings,
// or two empty strings when the history supports none. A best day outranks the
// usual rate, since it is the news; the usual rate prefers all 30 days and
// falls back to the worked ones only when idle days would make it 0.
func (p TodayPaceReadout) comparison() (full, short string) {
	switch {
	case p.DoneToday > 0 && p.BestPriorDay > 0 && p.DoneToday > p.BestPriorDay:
		return "best day in 30 days", "best in 30d"
	case p.MedianPerDay > 0:
		rate := formatPaceRate(p.MedianPerDay)
		return "usually " + rate + "/day", "usually " + rate
	case p.ActiveDayMedian > 0:
		rate := formatPaceRate(p.ActiveDayMedian)
		return "usually " + rate + " on active days", "usually " + rate
	}
	return "", ""
}

func formatPaceRate(rate float64) string {
	return strconv.FormatFloat(rate, 'f', -1, 64)
}
