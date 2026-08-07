package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func taskToolGuidanceSpec(confirmActive bool, activeInteraction bool, km *keymap.KeyMap) (GuidanceSpec, bool) {
	if confirmActive {
		return confirmationGuidanceSpec(), true
	}
	rows := 1
	if activeInteraction {
		rows = 2
	}
	return guidanceSpecFromKeyMap(km, rows)
}

func (m *TaskChecklistModel) guidanceSpec() (GuidanceSpec, bool) {
	return taskToolGuidanceSpec(m.confirm.IsActive(), m.addMode || m.editMode, m.buildKeyMap())
}

func (m *TaskCommentsModel) guidanceSpec() (GuidanceSpec, bool) {
	return taskToolGuidanceSpec(m.confirm.IsActive(), m.addMode || m.editMode || m.expandedView, m.buildKeyMap())
}

func (m *TaskAttachmentsModel) guidanceSpec() (GuidanceSpec, bool) {
	return taskToolGuidanceSpec(m.confirm.IsActive(), m.addMode, m.buildKeyMap())
}

func renderTaskToolGuidance(spec GuidanceSpec, ok bool, width int, accent ...color.Color) string {
	if !ok {
		return ""
	}
	if len(accent) > 0 {
		return renderGuidance(spec, width, accent[0])
	}
	return renderGuidance(spec, width)
}

// taskToolAvailableRows measures the already-rendered prefix and structured
// footer together. Content inserted between them may consume the remainder.
func taskToolAvailableRows(prefix, footer string, height int) int {
	if height <= 0 {
		return 1 << 20
	}
	return max(height-lipgloss.Height(prefix+footer), 0)
}

func taskToolMaxVisible(total, availableRows, maxRowsPerItem int) int {
	if total <= 0 || availableRows <= 0 {
		return 0
	}
	maxRowsPerItem = max(maxRowsPerItem, 1)
	visible := min(total, availableRows/maxRowsPerItem)
	if visible < total {
		visible = max((availableRows-1)/maxRowsPerItem, 0)
	}
	return visible
}
