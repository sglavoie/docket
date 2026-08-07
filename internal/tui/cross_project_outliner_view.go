package tui

import (
	"strings"
)

// scopeChainSeparator joins nested project-scope names inside the breadcrumb
// anchor (e.g. "[Alpha › Beta]").
const scopeChainSeparator = " › "

// scopeChromeSeparator joins the qualifiers appended to a scope or project name
// (e.g. "[Client Migration · 12 tasks]").
const scopeChromeSeparator = " · "

// activeProjectsScopeLabel names the unscoped global Outliner population in
// every piece of scope chrome.
const activeProjectsScopeLabel = "Active Projects"

// crossProjectZoomContext returns the full global-Outliner zoom path. The
// project scope is already part of the operational identity, so it is not
// repeated here. Width fitting belongs to the shared title renderer.
func (m *OutlinerModel) crossProjectZoomContext() string {
	if len(m.crossZoomStack) == 0 {
		return ""
	}

	// Resolve each entry to a task title, scoped to the owning
	// project. Using ProjectID (not a global search) prevents UUID collisions
	// across projects from resolving to the wrong task title.
	taskSegments := make([]string, 0, len(m.crossZoomStack))
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
	if len(taskSegments) == 0 {
		return ""
	}
	return "Zoom: " + strings.Join(taskSegments, " > ")
}
