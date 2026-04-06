package tui

// fieldHeight returns the rendered height of a field in lines.
func (m *BulkEditModel) fieldHeight(fieldIndex int) int {
	switch fieldIndex {
	case fieldStatus:
		return 8
	case fieldType:
		return 9
	case fieldPriority:
		return 8
	case fieldTags:
		height := 2
		if len(m.commonTags) > 0 {
			height++
		}
		if m.tagMode == 1 {
			height++
		} else {
			if len(m.addTags) > 0 {
				height++
			}
			if len(m.removeTags) > 0 {
				height++
			}
		}
		if m.tagMode == 2 {
			height += 1 + len(m.allTags)
		}
		if m.focusIndex == fieldTags && m.tagMode == 0 {
			height++
		}
		height++
		return height
	case fieldSection:
		return 3 + len(m.sectionOptions)
	case fieldDueDate:
		height := 5
		if m.dueDateInputMode {
			height++
			if m.dueDateError != nil {
				height++
			}
		}
		return height
	case fieldRecurrence:
		return 9
	case fieldParent:
		height := 5
		if m.parentListMode {
			visible := min(len(m.parentCandidates), 8)
			if visible == 0 {
				visible = 1
			}
			height += visible
		}
		return height
	case fieldActions:
		return 3
	default:
		return 0
	}
}

// fieldStartLine returns the starting line number of a field (0-indexed).
func (m *BulkEditModel) fieldStartLine(fieldIndex int) int {
	line := 0
	for i := range fieldIndex {
		line += m.fieldHeight(i)
	}
	return line
}

// viewportHeight returns the available height for content inside the box.
func (m *BulkEditModel) viewportHeight() int {
	return max(m.Height()-8, 15)
}

// ensureFocusedFieldVisible adjusts scrollOffset to keep the focused field visible.
func (m *BulkEditModel) ensureFocusedFieldVisible() {
	viewportHeight := m.viewportHeight()
	fieldStart := m.fieldStartLine(m.focusIndex)
	fieldEnd := fieldStart + m.fieldHeight(m.focusIndex)

	if fieldStart < m.scrollOffset {
		m.scrollOffset = fieldStart
	}

	if fieldEnd > m.scrollOffset+viewportHeight {
		m.scrollOffset = min(fieldEnd-viewportHeight, fieldStart)
	}

	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}
