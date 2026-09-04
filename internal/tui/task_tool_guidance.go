package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func taskToolGuidanceSpec(confirmActive bool, activeInteraction bool, km *keymap.KeyMap) (chrome.GuidanceSpec, bool) {
	if confirmActive {
		return chrome.ConfirmationGuidanceSpec(), true
	}
	rows := 1
	if activeInteraction {
		rows = 2
	}
	return chrome.GuidanceSpecFromKeyMap(km, rows)
}

func (m *TaskCommentsModel) guidanceSpec() (chrome.GuidanceSpec, bool) {
	return taskToolGuidanceSpec(m.Confirm.IsActive(), m.addMode || m.editMode || m.expandedView, m.buildKeyMap())
}

func (m *TaskAttachmentsModel) guidanceSpec() (chrome.GuidanceSpec, bool) {
	return taskToolGuidanceSpec(m.Confirm.IsActive(), m.addMode, m.buildKeyMap())
}

func renderTaskToolGuidance(spec chrome.GuidanceSpec, ok bool, width int, accent ...color.Color) string {
	if !ok {
		return ""
	}
	if len(accent) > 0 {
		return chrome.RenderGuidance(spec, width, accent[0])
	}
	return chrome.RenderGuidance(spec, width)
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
