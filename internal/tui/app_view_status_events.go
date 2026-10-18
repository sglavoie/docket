package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// statusStamper is the second status surface: views embedding
// InteractiveViewState own a status line the shell does not write.
type statusStamper interface {
	StatusStamp() uint64
}

// viewStatusOwner is that same status line seen by the expiry timer: it stamps
// generations, declares whether the current message is transient, and clears
// itself when its tick comes back.
//
// Every method here must be exported. An unexported method name is qualified by
// its declaring package in a Go method set, so once InteractiveViewState moved
// down to internal/tui/listsurface, a lowercase clearStatus() in this literal
// became unsatisfiable by any view — a silent ok == false that no build, vet, or
// lint pass reports. The assertion below is what makes that failure loud.
type viewStatusOwner interface {
	statusStamper
	StatusIsTransient() bool
	StatusText() string
	ClearStatus()
}

// The Outliner is the view this interface exists for, so its conformance is
// asserted at compile time rather than discovered as a dead type switch.
var _ viewStatusOwner = (*outliner.OutlinerModel)(nil)

// Notes keeps its own status line and error styling, so it answers the same
// interface directly rather than through InteractiveViewState.
var _ viewStatusOwner = (*notes.NotesViewModel)(nil)

// footerFlashProbe is the status identity compared across one Update. The
// shell's own message carries a sequence, but the error beside it has ~87
// assignment sites and no clock, so it is compared by text. The view stamp is
// only meaningful alongside the view it came from, hence the presentation ID.
type footerFlashProbe struct {
	statusText    string
	statusSeq     uint64
	errText       string
	view          presentation.PresentationID
	viewStatusSeq uint64
	// pathStamps is the generation every stamping view along the live path held,
	// so a view this Update surfaced is compared against its own earlier stamp:
	// closing capture onto the Outliner changes the view and the Outliner's
	// status in one Update, and neither the capture's stamp nor none at all is
	// the right thing to measure that status against.
	pathStamps []viewStatusStamp
}

type viewStatusStamp struct {
	view presentation.PresentationID
	seq  uint64
}

// stampFor reports the generation view held when the probe was taken, and
// whether it was on the path at all. A view's own stamp outranks the active
// one, since a bottom panel such as capture is the active view without being
// the presentation it stands over.
func (p footerFlashProbe) stampFor(view presentation.PresentationID) (uint64, bool) {
	for _, stamp := range p.pathStamps {
		if stamp.view == view {
			return stamp.seq, true
		}
	}
	if view == p.view {
		return p.viewStatusSeq, true
	}
	return 0, false
}

func (m *AppModel) captureFooterFlashProbe() footerFlashProbe {
	probe := footerFlashProbe{
		statusSeq:  m.feedback.seq,
		statusText: m.feedback.message,
		view:       m.currentView(),
	}
	if m.feedback.err != nil {
		probe.errText = m.feedback.err.Error()
	}
	if stamper, ok := m.activeView().(statusStamper); ok {
		probe.viewStatusSeq = stamper.StatusStamp()
	}
	if m.viewNav.views != nil {
		for _, view := range m.statusStampingPath() {
			if stamper, ok := m.viewNav.views.Get(view).(statusStamper); ok {
				probe.pathStamps = append(probe.pathStamps, viewStatusStamp{view: view, seq: stamper.StatusStamp()})
			}
		}
	}
	return probe
}

// armFooterFlashSince starts a flash when this Update changed status text to
// something worth interrupting for. Cleared text arms nothing: neither
// clearStatus nor the auto-clear timers advance a sequence, and a vanished error
// fails the non-empty test.
//
// Routine text changes nothing about the divider and still supersede: a success,
// an informational line, and a view-owned notice are all quiet, and the one they
// replace stops animating under them. The two questions are deliberately
// separate — which change happened, then whether that change animates — because
// a change that arms nothing is not the same as no change at all, and only the
// first of those retires what was running.
func (m *AppModel) armFooterFlashSince(before footerFlashProbe) tea.Cmd {
	after := m.captureFooterFlashProbe()

	severity := flashNone
	switch {
	case after.errText != "" && after.errText != before.errText:
		severity = flashError
	case after.statusSeq != before.statusSeq:
		severity = m.feedback.severity
	case after.view == before.view && after.viewStatusSeq != before.viewStatusSeq:
		severity = flashInfo
	}
	if severity == flashNone {
		// No status changed, so nothing was superseded either: a flash already
		// running belongs to text still on screen and keeps its remaining frames.
		return nil
	}
	if !severity.animates() {
		m.feedback.retireFlash()
		return nil
	}

	return footerFlashTick(m.feedback.armFlash(severity))
}

// statusStampingPath is every presentation whose own status line can be live:
// the navigation path, or the Outliner a path-less launch stands on.
func (m *AppModel) statusStampingPath() []presentation.PresentationID {
	if path := m.presentationPath(); len(path) > 0 {
		return path
	}
	return []presentation.PresentationID{m.currentView()}
}

// scheduleViewStatusExpiry arms the auto-clear for a view-owned status message
// this Update introduced. It reads the same before/after comparison as the
// flash, since a new message is what both observe, but it decides for itself: a
// view notice is quiet and still expires on time, so how long a message lasts
// never follows from whether it animated. A sticky message schedules nothing and
// stays until its view clears it; the Outliner writes none, because a live state
// descriptor belongs on the context row rather than in the channel a piece of
// feedback is about to want back.
func (m *AppModel) scheduleViewStatusExpiry(before footerFlashProbe) tea.Cmd {
	if m.viewNav.views == nil {
		return nil
	}
	// Every view on the path is asked, not just the one on top: a notice can be
	// written onto the Outliner while capture or help stands over it, and its
	// timer has to start when the notice does rather than when the cover lifts.
	var expiries []tea.Cmd
	for _, view := range m.statusStampingPath() {
		owner, ok := m.viewNav.views.Get(view).(viewStatusOwner)
		if !ok || !owner.StatusIsTransient() {
			continue
		}
		seq := owner.StatusStamp()
		if held, known := before.stampFor(view); known && held == seq {
			continue
		}
		expiries = append(expiries, status.ViewStatusTimeout(view, seq, owner.StatusText()))
	}
	return tea.Batch(expiries...)
}

// The count itself lives on the feedback cluster, which also supplies the
// accessor injected into every Outliner; see feedback_controller.go.

// rememberFeedbackSince follows the same generations as status expiry, keeping
// the complete latest feedback after its display timer or navigation clears it.
// Ambient advisory lines are not status messages and never enter this record.
func (m *AppModel) rememberFeedbackSince(before footerFlashProbe) {
	after := m.captureFooterFlashProbe()
	if after.errText != "" && after.errText != before.errText {
		m.feedback.rememberFeedback(after.errText, "error", 0)
		return
	}
	if m.feedback.message != "" && (after.statusSeq != before.statusSeq || after.statusText != before.statusText) {
		m.feedback.rememberFeedback(m.feedback.message, "shell", after.statusSeq)
		return
	}
	if m.viewNav.views == nil {
		return
	}
	for _, view := range m.statusStampingPath() {
		owner, ok := m.viewNav.views.Get(view).(viewStatusOwner)
		if !ok || !owner.StatusIsTransient() || owner.StatusText() == "" {
			continue
		}
		if held, known := before.stampFor(view); known && held == owner.StatusStamp() {
			continue
		}
		m.feedback.rememberFeedback(owner.StatusText(), fmt.Sprintf("view:%v:%p", view, owner), owner.StatusStamp())
	}
}
