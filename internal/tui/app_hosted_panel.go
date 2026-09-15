package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

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
	if view := m.hostedPanelViewFor(hostID); view != nil {
		return hostID, view, true
	}
	return 0, nil, false
}

func (m *AppModel) hostedPanelViewFor(hostID presentation.PresentationID) route.View {
	if m.manageHub != nil {
		if view, ok := m.manageHub.PresentationView(); ok && view == hostID {
			return m.manageHub
		}
	}
	if m.viewNav.views == nil {
		return nil
	}
	return m.viewNav.views.Get(hostID)
}

// composeHostedPanelContent stacks the panel under the presentation it was
// opened from. The host is re-rendered inside the rows the panel left it rather
// than covered by it, which is the whole reason the shell resizes it: a panel
// drawn on top of a full-height host would hide the row the host's cursor is on.
func (m *AppModel) composeHostedPanelContent(panel string, contentHeight int) string {
	layout, ok := m.hostedPanelLayout()
	if !ok {
		return panel
	}
	hostRows := contentHeight - lipgloss.Height(panel)
	if hostRows < 1 {
		return panel
	}
	frame, hosted := m.hostedPanelFrame()
	if !hosted || !layout.HostDrawn {
		// The panel asked for its share of the area whatever the shell found
		// above it, so the rows it declined are blank rather than its own: a
		// bottom panel that floated to the top row would read as a full view.
		return chrome.ComposeHostedPanel("", panel, hostRows)
	}
	return chrome.ComposeHostedPanel(frame.view.View().Content, panel, hostRows)
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
	frame.view.SetSize(width, frame.layout.HostRows)
}

func (m *AppModel) releaseHostedPanelHost() {
	if !m.hostedPanelHost.active {
		return
	}
	released := m.hostedPanelHost.view
	m.hostedPanelHost = hostedPanelHostRecord{}
	if view := m.hostedPanelViewFor(released); view != nil {
		view.SetSize(m.viewNav.viewSize())
	}
}
