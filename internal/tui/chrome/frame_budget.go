package chrome

// This file is the measured half of the composed frame. HostedPanelLayout above
// divides one content area between a panel and its host; this divides the
// terminal between that content area and the shell-owned footer under it, which
// is the other place a row is spent without the view that loses it being asked.

const (
	// ShellFooterDividerRows is the divider every shell footer opens with. It is
	// drawn unconditionally, so it is a constant here rather than a measurement.
	ShellFooterDividerRows = 1

	// ShellFooterBaselineRows is the footer every renderer is sized against: that
	// divider and one row of content. The shell hands each view height-2, so this
	// is the zero point a measured footer is reported against and the reason
	// ShellFrameMetrics reports an excess rather than a height.
	ShellFooterBaselineRows = ShellFooterDividerRows + 1
)

// ShellFrameMetrics is the height of the shell-owned footer for one frame, split
// into the two things that footer says: what just happened, and what the keys
// still do. Both are measured from the same strings the render composes, so a
// reservation and a paint cannot disagree about how tall the footer is.
//
// It is a measurement and never a decision: it says nothing about which layer
// owns guidance, whether a status animates, or what a view should do with the
// rows. A view that has to fit under a taller footer reads ExcessRows and
// reserves it; a view that draws its own footer is handed the feedback line
// instead and pays for it out of its own body.
//
// The zero value is the baseline footer — no feedback and no controls beyond the
// one row every view is already sized against — so a view that is never told
// anything behaves exactly as it did before it could be told.
type ShellFrameMetrics struct {
	// FeedbackRows is what the transient status or error line costs, zero when
	// the footer carries neither.
	FeedbackRows int
	// ControlRows is what the live controls cost: an unfinished chord's
	// which-key row, or the active surface's guidance, pinned bar, or
	// breadcrumb. A suppressed occupant still costs its own rows, because
	// suppression blanks it rather than dropping it.
	ControlRows int
}

// FooterRows is the footer's whole height. The divider is always drawn, and so
// is one row under it: a footer with nothing to say is a blank line rather than
// a divider flush against the terminal edge, which is what every view is already
// sized against.
func (m ShellFrameMetrics) FooterRows() int {
	return ShellFooterDividerRows + max(m.FeedbackRows+m.ControlRows, 1)
}

// ExcessRows is what this footer costs beyond the budget the view was sized
// with. It is what a view reserves from its own content: subtracting it is the
// one place the compensating guesses each surface used to make — a guard row
// here, a re-measured guidance block there — are now made once, from the footer
// that was actually composed.
func (m ShellFrameMetrics) ExcessRows() int {
	return max(m.FooterRows()-ShellFooterBaselineRows, 0)
}
