package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// scopeChainSeparator joins nested project-scope names inside the breadcrumb
// anchor (e.g. "[Alpha › Beta]").
const scopeChainSeparator = " › "

// crossProjectBreadcrumb returns the rendered scope/zoom breadcrumb for the
// cross-project outliner. It renders whenever a project scope is active or the
// view is zoomed, and returns "" only at the base all-projects, unzoomed scope.
//
// Format: "[ScopeAnchor] > Task1 > Task2..."
//
// The bracketed anchor is the first segment and always identifies the view's
// project context: the full project-scope chain when scoped (e.g.
// "[Alpha › Beta]"), otherwise the zoomed project's name. Zoom task titles
// follow. When the breadcrumb exceeds maxWidth, task segments are dropped from
// the left and replaced with "…"; the anchor is never dropped, and as a last
// resort it is hard-truncated so the chrome can never overflow onto a second
// wrapped line.
func (m *OutlinerModel) crossProjectBreadcrumb(maxWidth int) string {
	scopeNames := m.scopeChainNames()
	if len(scopeNames) == 0 && len(m.crossZoomStack) == 0 {
		return ""
	}

	separator := " > "

	// Bracketed anchor: the scope chain when scoped, else the zoomed project.
	var anchor string
	if len(scopeNames) > 0 {
		anchor = "[" + strings.Join(scopeNames, scopeChainSeparator) + "]"
	} else {
		top := m.crossZoomStack[0]
		anchor = "[" + m.projectNameByID(top.ProjectID) + "]"
	}

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

	fits := func(candidate string) (string, bool) {
		rendered := MutedStyle.Render(candidate)
		return rendered, lipgloss.Width(rendered) <= maxWidth
	}

	full := anchor
	if len(taskSegments) > 0 {
		full = anchor + separator + strings.Join(taskSegments, separator)
	}
	if rendered, ok := fits(full); ok {
		return rendered
	}

	// Truncate task segments from the left; the anchor is always kept. Each
	// iteration drops the oldest task segment and replaces it with "…".
	ellipsis := "…"
	for len(taskSegments) > 1 {
		taskSegments = taskSegments[1:]
		if rendered, ok := fits(anchor + separator + ellipsis + separator + strings.Join(taskSegments, separator)); ok {
			return rendered
		}
	}
	if len(taskSegments) == 1 {
		if rendered, ok := fits(anchor + separator + ellipsis + separator + taskSegments[0]); ok {
			return rendered
		}
	}

	// Even the anchor alone overflows the width; hard-truncate it so the scope
	// chrome always stays on a single line.
	return MutedStyle.Render(truncateToVisualWidth(anchor, maxWidth))
}
