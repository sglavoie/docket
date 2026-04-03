package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// breadcrumb returns the rendered breadcrumb string showing the ancestor chain
// from the first zoom level to the current zoom target. Returns empty string
// when not zoomed. If the breadcrumb exceeds maxWidth, leftmost segments are
// dropped and replaced with "..." until it fits.
func (m *OutlinerModel) breadcrumb(maxWidth int) string {
	if len(m.zoomStack) == 0 || m.project == nil {
		return ""
	}

	separator := " > "
	var segments []string
	for _, entry := range m.zoomStack {
		if t := m.project.GetTaskByID(entry.TaskID); t != nil {
			segments = append(segments, t.Title)
		}
	}
	if len(segments) == 0 {
		return ""
	}

	full := strings.Join(segments, separator)
	rendered := MutedStyle.Render(full)
	if lipgloss.Width(rendered) <= maxWidth {
		return rendered
	}

	// Truncate from the left, replacing dropped segments with "...".
	ellipsis := "..."
	for len(segments) > 1 {
		segments = segments[1:]
		candidate := ellipsis + separator + strings.Join(segments, separator)
		rendered = MutedStyle.Render(candidate)
		if lipgloss.Width(rendered) <= maxWidth {
			return rendered
		}
	}

	// Even the last segment alone with ellipsis doesn't fit; render it anyway.
	return MutedStyle.Render(ellipsis + separator + segments[0])
}
