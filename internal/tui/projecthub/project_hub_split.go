package projecthub

import "github.com/sglavoie/docket/internal/tui/listsurface"

// ProjectHubPreviewSplit is where the Hub divides its list from its preview,
// as the list's share of the width. It is session-only: the Manage hub owns one
// and lends it to every Hub it mounts, so reopening Select Project keeps the
// split while a restart returns to the default. Zero is the default split.
type ProjectHubPreviewSplit struct {
	ListPercent int
}

// SetPreviewSplit lends the Hub the session's split. A Hub without one keeps a
// private split of its own, which is what a construction outside the app gets.
func (m *ProjectHubModel) SetPreviewSplit(split *ProjectHubPreviewSplit) {
	if split != nil {
		m.split = split
	}
}

// previewSplit returns the split the Hub draws with, minting a private one on
// first use so every caller reads and writes one value.
func (m *ProjectHubModel) previewSplit() *ProjectHubPreviewSplit {
	if m.split == nil {
		m.split = &ProjectHubPreviewSplit{}
	}
	return m.split
}

// ShiftPreviewSplit moves the divider one step: a negative direction narrows
// the list and widens the preview, a positive one the reverse. The first move
// turns the content-fitted default into an explicit share, which then holds. It is inert while
// no preview pane is drawn, so a press the frame cannot show moves nothing.
func (m *ProjectHubModel) ShiftPreviewSplit(direction int) {
	if direction == 0 || !m.previewPaneVisible() {
		return
	}
	split := m.previewSplit()
	current := split.ListPercent
	if current <= 0 {
		// The default split may already be narrowed to the list's content, so
		// the first press moves from the divider the frame drew rather than
		// jumping back to the default share first.
		config := m.projectHubRenderConfig(0)
		listWidth, _ := listsurface.ListSurfacePreviewGeometry(config)
		current = listWidth * 100 / max(config.Width, 1)
	}
	step := listsurface.ListSurfaceListWidthPercentStep
	if direction < 0 {
		step = -step
	}
	next := min(
		max(current+step, listsurface.ListSurfaceMinListWidthPercent),
		listsurface.ListSurfaceMaxListWidthPercent,
	)
	// A content-fitted list can already sit outside the bounds a moved split
	// keeps, so clamping could carry the divider against the press. A move the
	// divider cannot make in the pressed direction is no move at all.
	if (direction < 0 && next >= current) || (direction > 0 && next <= current) {
		return
	}
	split.ListPercent = next
}

// previewPaneVisible answers whether the frame draws a preview pane at all:
// the list is showing, not an action sheet, a row has a preview to show, and
// the terminal fits both panes.
func (m *ProjectHubModel) previewPaneVisible() bool {
	if m.surface == nil || m.actions != nil {
		return false
	}
	if _, ok := m.surface.CurrentPreview(); !ok {
		return false
	}
	_, previewWidth := listsurface.ListSurfacePreviewGeometry(m.projectHubRenderConfig(0))
	return previewWidth > 0
}
