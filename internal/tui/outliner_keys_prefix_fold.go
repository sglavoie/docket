package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleFoldPrefix handles the z prefix key for fold/zoom operations.
func (m *OutlinerModel) handleFoldPrefix(key string, cursorTask *models.Task) (tea.Model, tea.Cmd) {
	switch key {
	case "a":
		// Toggle fold
		if cursorTask != nil && len(cursorTask.Children) > 0 {
			m.ensureExpandedNodes()
			if m.isExpanded(cursorTask.ID) {
				m.expandedNodes[cursorTask.ID] = false
			} else {
				m.expandedNodes[cursorTask.ID] = true
			}
			m.rebuildRows()
		}
		return m, nil
	case "o":
		// Expand
		if cursorTask != nil && len(cursorTask.Children) > 0 {
			m.ensureExpandedNodes()
			m.expandedNodes[cursorTask.ID] = true
			m.rebuildRows()
		}
		return m, nil
	case "c":
		// Collapse
		if cursorTask != nil && len(cursorTask.Children) > 0 {
			m.ensureExpandedNodes()
			m.expandedNodes[cursorTask.ID] = false
			m.rebuildRows()
		}
		return m, nil
	case "R":
		// Expand all
		m.expandedNodes = make(map[string]bool)
		m.rebuildRows()
		return m, nil
	case "M":
		// Collapse all - mark all parent tasks as collapsed
		if m.project == nil {
			m.ensureExpandedNodes()
			for _, p := range m.loadedProjects {
				m.collapseAllParents(p.GetRootTasks())
			}
			m.rebuildRows()
			return m, nil
		}
		m.ensureExpandedNodes()
		m.collapseAllParents(m.project.GetRootTasks())
		m.rebuildRows()
		return m, nil
	case "v":
		// Zoom into current task.
		if cursorTask != nil && len(cursorTask.Children) > 0 {
			if m.project == nil {
				// Cross-project mode: push CrossProjectZoomEntry scoped to the
				// owning project so UUID collisions across projects cannot cause
				// incorrect zoom resolution.
				projectID := m.rows[m.cursor].ProjectID
				m.crossZoomStack = append(m.crossZoomStack, CrossProjectZoomEntry{
					TaskID:      cursorTask.ID,
					ProjectID:   projectID,
					CursorIndex: m.cursor,
				})
				m.cursor = 0
			} else {
				m.zoomStack = append(m.zoomStack, ZoomEntry{
					TaskID:      cursorTask.ID,
					CursorIndex: m.cursor,
				})
				m.cursor = 0
			}
		}
		return m, nil
	case "p":
		m.previewMode = !m.previewMode
		return m, nil
	}

	return m, nil
}
