package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// The rest of this file is the shell's side of a bounded bottom panel. It sits
// with the composition rather than in a file of its own because handing a
// presentation fewer rows and then drawing it inside them is one act, not two:
// the same arithmetic decides what the host is sized to and what the frame puts
// above the panel, and splitting it would leave the root shell carrying the
// same concern twice.

// hostedPanelHostRecord names the presentation the shell shrank for a bounded
// bottom panel and the budget it handed it. The record exists for the release:
// once the panel closes, only one view is owed its whole content area back, and
// a sweep over the registry would reset the scroll window of every other surface
// to say so.
type hostedPanelHostRecord struct {
	view   presentation.PresentationID
	height int
	active bool
}

// sizedView is satisfied by every list surface through its shared base. The
// shell reads the host's own size rather than trusting what it last handed it,
// so a re-registration that resets the host to the full budget is corrected on
// the next update instead of silently clipping the host's cursor row away.
type sizedView interface {
	Width() int
	Height() int
}

// hostedPanelFrame is the active view's panel layout together with the
// presentation it is drawn over: the entry directly beneath the panel's own
// overlay in the derived path, so a panel is hosted by whatever raised it rather
// than by a hardcoded surface.
type hostedPanelFrame struct {
	layout chrome.HostedPanelLayout
	host   presentation.PresentationID
	view   route.View
}

// hostedPanelLayout is what the active view says about its own presentation,
// read before any host is resolved. The panel sizes itself from this alone, so
// the shell has to honor it even when there is nothing to draw underneath.
func (m *AppModel) hostedPanelLayout() (chrome.HostedPanelLayout, bool) {
	active := m.activeView()
	if active == nil {
		return chrome.HostedPanelLayout{}, false
	}
	panel, ok := active.(route.HostedPanelView)
	if !ok {
		return chrome.HostedPanelLayout{}, false
	}
	layout := panel.HostedPanelLayout(m.viewNav.viewSize())
	return layout, layout.Active()
}

func (m *AppModel) hostedPanelFrame() (hostedPanelFrame, bool) {
	layout, ok := m.hostedPanelLayout()
	if !ok {
		return hostedPanelFrame{}, false
	}
	hostID, host, ok := m.hostedPanelHostView()
	if !ok {
		return hostedPanelFrame{}, false
	}
	return hostedPanelFrame{layout: layout, host: hostID, view: host}, true
}

// hostedPanelHostView resolves the renderer one step below the top of the
// derived presentation path. The hub is registry-external, so it answers for
// itself before the registry is consulted.
func (m *AppModel) hostedPanelHostView() (presentation.PresentationID, route.View, bool) {
	path := m.presentationPath()
	if len(path) < 2 {
		return 0, nil, false
	}
	hostID := path[len(path)-2]
	if view := m.presentationViewFor(hostID); view != nil {
		return hostID, view, true
	}
	return 0, nil, false
}

// presentationViewFor resolves a renderer by its presentation identity. It is
// how both budget-handing seams find the view they last spoke to: the registry
// is keyed that way and the hub, being registry-external, answers for itself
// first.
func (m *AppModel) presentationViewFor(view presentation.PresentationID) route.View {
	if m.manageHub != nil {
		if hub, ok := m.manageHub.PresentationView(); ok && hub == view {
			return m.manageHub
		}
	}
	if m.viewNav.views == nil {
		return nil
	}
	return m.viewNav.views.Get(view)
}

// noPanelDivider is the divider row of a frame no bounded panel opens in.
const noPanelDivider = -1

// composeHostedPanelContent stacks the panel under the presentation it was
// opened from. The host is re-rendered inside the rows the panel left it rather
// than covered by it, which is the whole reason the shell resizes it: a panel
// drawn on top of a full-height host would hide the row the host's cursor is on.
// The row it also returns is the panel's divider, its first row, or
// noPanelDivider when the content is no bounded panel.
func (m *AppModel) composeHostedPanelContent(panel string, contentHeight int) (string, int) {
	layout, ok := m.hostedPanelLayout()
	if !ok {
		return panel, noPanelDivider
	}
	hostRows := contentHeight - lipgloss.Height(panel)
	if hostRows < 1 {
		return panel, 0
	}
	frame, hosted := m.hostedPanelFrame()
	if !hosted || !layout.HostDrawn {
		// The panel asked for its share of the area whatever the shell found
		// above it, so the rows it declined are blank rather than its own: a
		// bottom panel that floated to the top row would read as a full view.
		return chrome.ComposeHostedPanel("", panel, hostRows), hostRows
	}
	return chrome.ComposeHostedPanel(frame.view.View().Content, panel, hostRows), hostRows
}

// syncHostedPanelHost hands the presentation under a bottom panel the budget the
// panel left it, and hands it back when the panel closes. It runs once per
// update rather than once per frame because a view resized on every frame is a
// view that never keeps a scroll window between two of them.
func (m *AppModel) syncHostedPanelHost() {
	frame, ok := m.hostedPanelFrame()
	if !ok {
		m.releaseHostedPanelHost()
		return
	}
	if m.hostedPanelHost.active && m.hostedPanelHost.view != frame.host {
		m.releaseHostedPanelHost()
	}
	width, _ := m.viewNav.viewSize()
	applied := m.hostedPanelHost
	m.hostedPanelHost = hostedPanelHostRecord{view: frame.host, height: frame.layout.HostRows, active: true}
	if sized, ok := frame.view.(sizedView); ok {
		if sized.Width() == width && sized.Height() == frame.layout.HostRows {
			return
		}
	} else if applied.active && applied.view == frame.host && applied.height == frame.layout.HostRows {
		// A host that cannot report its own size is taken at the record's word:
		// it still holds the budget this shell last handed it.
		return
	}
	applyHostedBudget(frame.view, width, frame.layout.HostRows)
}

func (m *AppModel) releaseHostedPanelHost() {
	if !m.hostedPanelHost.active {
		return
	}
	released := m.hostedPanelHost.view
	m.hostedPanelHost = hostedPanelHostRecord{}
	if view := m.presentationViewFor(released); view != nil {
		width, height := m.viewNav.viewSize()
		releaseHostedBudget(view, width, height)
	}
}

// applyHostedBudget and releaseHostedBudget are the two halves of the loan a
// bounded panel takes out of its host's content area. They exist because the
// shell has one act to perform and the host has two ways of answering it: a
// surface that keeps a scroll window says so through route.HostedBudgetView and
// is told which of the two this is, and every other surface is resized the way
// it always was.
func applyHostedBudget(view route.View, width, height int) {
	if budgeted, ok := view.(route.HostedBudgetView); ok {
		budgeted.SetHostedBudget(width, height)
		return
	}
	view.SetSize(width, height)
}

func releaseHostedBudget(view route.View, width, height int) {
	if budgeted, ok := view.(route.HostedBudgetView); ok {
		budgeted.ReleaseHostedBudget(width, height)
		return
	}
	view.SetSize(width, height)
}

// shellFrameRecipient names the presentation the shell last handed a footer
// budget and what it handed it. It exists for the same reason the hosted-panel
// record does: exactly one view is drawing under this footer, and a sweep over
// the registry would tell every other surface to reserve rows it is not paying
// for.
type shellFrameRecipient struct {
	view    presentation.PresentationID
	metrics chrome.ShellFrameMetrics
	active  bool
}

// syncShellFrameMetrics measures the footer this frame will draw and hands the
// active view what it costs beyond the two rows every view is sized against.
//
// It runs once per update rather than once per frame for the reason
// syncHostedPanelHost does: a budget delivered mid-render would make the paint
// depend on the order the frame was assembled in, and the measurement it carries
// is of a footer that has not been composed yet.
//
// Only the active view is told, and the one that stops being active is handed
// the zero value. A host drawn under a bounded panel is not the view paying for
// the footer — the panel above it is — so telling both would reserve the same
// rows twice and leave a band of blank content above the divider.
func (m *AppModel) syncShellFrameMetrics() {
	view := m.currentPresentationView()
	if m.shellFrame.active && m.shellFrame.view != view {
		m.releaseShellFrameMetrics()
	}
	aware, ok := m.activeView().(route.ShellFrameAware)
	if !ok {
		return
	}
	metrics := m.shellFrameMetrics()
	if m.shellFrame.active && m.shellFrame.view == view && m.shellFrame.metrics == metrics {
		return
	}
	m.shellFrame = shellFrameRecipient{view: view, metrics: metrics, active: true}
	aware.SetShellFrameMetrics(metrics)
}

func (m *AppModel) releaseShellFrameMetrics() {
	if !m.shellFrame.active {
		return
	}
	released := m.shellFrame.view
	m.shellFrame = shellFrameRecipient{}
	if aware, ok := m.presentationViewFor(released).(route.ShellFrameAware); ok {
		aware.SetShellFrameMetrics(chrome.ShellFrameMetrics{})
	}
}
