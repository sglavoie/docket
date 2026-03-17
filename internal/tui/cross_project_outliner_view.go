package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// crossProjectBreadcrumb returns the rendered breadcrumb string for the
// cross-project outliner when the view is zoomed. Returns empty string when
// not zoomed (crossZoomStack is empty).
//
// Format: "[ProjectName] > Task1 > Task2..."
//
// The project name is always kept as the first segment. When the full breadcrumb
// exceeds maxWidth, task segments are dropped from the left and replaced with
// "…" until it fits. The project prefix is never dropped.
func (m *OutlinerModel) crossProjectBreadcrumb(maxWidth int) string {
	if len(m.crossZoomStack) == 0 {
		return ""
	}

	separator := " > "
	top := m.crossZoomStack[0]
	projName := m.projectNameByID(top.ProjectID)
	projectPrefix := "[" + projName + "]"

	// Resolve each zoom stack entry to a task title, scoped to the owning
	// project. Using ProjectID (not a global search) prevents UUID collisions
	// across projects from resolving to the wrong task title.
	var taskSegments []string
	for _, entry := range m.crossZoomStack {
		for _, p := range m.loadedProjects {
			if p.ID == entry.ProjectID {
				if t := p.GetTaskByID(entry.TaskID); t != nil {
					taskSegments = append(taskSegments, t.Title)
				}
				break
			}
		}
	}

	segments := make([]string, 0, len(taskSegments)+1)
	segments = append(segments, projectPrefix)
	segments = append(segments, taskSegments...)

	full := strings.Join(segments, separator)
	rendered := MutedStyle.Render(full)
	if lipgloss.Width(rendered) <= maxWidth {
		return rendered
	}

	// Truncate task segments from the left; the project prefix is always kept.
	// Each iteration drops the oldest task segment and replaces it with "…".
	ellipsis := "…"
	for len(taskSegments) > 1 {
		taskSegments = taskSegments[1:]
		candidate := projectPrefix + separator + ellipsis + separator + strings.Join(taskSegments, separator)
		rendered = MutedStyle.Render(candidate)
		if lipgloss.Width(rendered) <= maxWidth {
			return rendered
		}
	}

	// Even the single remaining task segment doesn't fit; render it anyway.
	if len(taskSegments) > 0 {
		return MutedStyle.Render(projectPrefix + separator + ellipsis + separator + taskSegments[0])
	}
	return MutedStyle.Render(projectPrefix)
}
