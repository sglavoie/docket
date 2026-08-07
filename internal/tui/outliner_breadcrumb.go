package tui

import (
	"strings"
)

// projectZoomContext returns the full single-project zoom path. Width fitting
// belongs to the operational-title renderer so pulse components can yield
// before this active interaction context is shortened.
func (m *OutlinerModel) projectZoomContext() string {
	if len(m.zoomStack) == 0 || m.project == nil {
		return ""
	}

	segments := make([]string, 0, len(m.zoomStack))
	for _, entry := range m.zoomStack {
		if t := m.project.GetTaskByID(entry.TaskID); t != nil {
			segments = append(segments, t.Title)
		}
	}
	if len(segments) == 0 {
		return ""
	}
	return "Zoom: " + strings.Join(segments, " > ")
}
