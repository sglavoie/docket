package projecthub

import "github.com/sglavoie/tt/internal/tui/listsurface"

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
// the list and widens the preview, a positive one the reverse. It is inert while
// no preview pane is drawn, so a press the frame cannot show moves nothing.
func (m *ProjectHubModel) ShiftPreviewSplit(direction int) {
	if direction == 0 || !m.previewPaneVisible() {
		return
	}
	split := m.previewSplit()
	current := split.ListPercent
	if current <= 0 {
		current = listsurface.ListSurfaceDefaultListWidthPercent
	}
	step := listsurface.ListSurfaceListWidthPercentStep
	if direction < 0 {
		step = -step
	}
	split.ListPercent = min(
		max(current+step, listsurface.ListSurfaceMinListWidthPercent),
		listsurface.ListSurfaceMaxListWidthPercent,
	)
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
