package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleFoldPrefix handles the f prefix key for fold/zoom operations.
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
			m.resetViewScrollOffset()
			m.rebuildRows()
		}
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
				m.resetViewScrollOffset()
			} else {
				m.zoomStack = append(m.zoomStack, ZoomEntry{
					TaskID:      cursorTask.ID,
					CursorIndex: m.cursor,
				})
				m.cursor = 0
				m.resetViewScrollOffset()
			}
		}
		return m, nil
	case "p":
		m.previewMode = !m.previewMode
		m.resetViewScrollOffset()
		return m, nil
	case "S":
		m.toggleAllSectionFolds()
		return m, nil
	}

	return m, nil
}

func (m *OutlinerModel) toggleAllSectionFolds() bool {
	keys := make([]string, 0)
	seen := make(map[string]bool)
	anyExpanded := false

	for _, row := range m.rows {
		if !row.IsSectionHeader {
			continue
		}
		key := row.effectiveCollapseKey()
		if !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
		if m.collapsedSections == nil || !m.collapsedSections[key] {
			anyExpanded = true
		}
	}

	if len(keys) == 0 {
		return false
	}

	if m.collapsedSections == nil {
		m.collapsedSections = make(map[string]bool)
	}
	targetCollapsed := anyExpanded
	for _, key := range keys {
		m.collapsedSections[key] = targetCollapsed
	}
	m.resetViewScrollOffset()
	m.rebuildRows()
	return true
}

func (m *OutlinerModel) toggleSectionFoldAtCursor() bool {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}

	headerIndex := -1
	row := m.rows[m.cursor]
	switch {
	case row.IsSectionHeader:
		headerIndex = m.cursor
	case row.IsSeparator:
		return false
	case row.task != nil:
		for i := m.cursor - 1; i >= 0; i-- {
			candidate := m.rows[i]
			if candidate.IsSeparator {
				return false
			}
			if candidate.IsSectionHeader {
				headerIndex = i
				break
			}
		}
	default:
		return false
	}

	if headerIndex < 0 {
		return false
	}

	if m.collapsedSections == nil {
		m.collapsedSections = make(map[string]bool)
	}
	headerRow := m.rows[headerIndex]
	key := headerRow.effectiveCollapseKey()
	m.collapsedSections[key] = !m.collapsedSections[key]
	m.cursor = headerIndex
	m.resetViewScrollOffset()
	m.rebuildRows()
	return true
}
