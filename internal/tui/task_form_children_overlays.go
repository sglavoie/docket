package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TaskFormChecklistItemEditedMsg is emitted when a checklist item edit is confirmed.
type TaskFormChecklistItemEditedMsg struct {
	Idx  int
	Text string
}

// handleChecklistEditKey handles key events for the checklist item edit overlay.
func (m *TaskFormModel) handleChecklistEditKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.checklistItemInput.Value())
		return nil, true, TaskFormChecklistItemEditedMsg{Idx: m.childrenEditIdx, Text: text}
	case "esc":
		return nil, true, nil // cancel
	default:
		var cmd tea.Cmd
		m.checklistItemInput, cmd = m.checklistItemInput.Update(msg)
		return cmd, false, nil
	}
}

// renderChecklistEditOverlay renders the inline checklist item edit overlay.
func (m *TaskFormModel) renderChecklistEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Edit Checklist Item")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := min(max(maxWidth, titleMinInner), max(m.width-4, 1))
	footerText := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec(
		"Enter save",
	), max(innerWidth-4, 1))

	m.checklistItemInput.SetWidth(max(innerWidth-5, 1))
	theme.ApplyThemedTextInput(&m.checklistItemInput)
	content := " " + m.checklistItemInput.View()

	return chrome.RenderOverlayBox(content, innerWidth, title, footerText, borderStyle)
}

// TaskFormSubtaskEditedMsg is emitted when a subtask draft edit is confirmed.
type TaskFormSubtaskEditedMsg struct {
	Idx         int
	Title       string
	OpenContent bool
}

// handleSubtaskEditKey handles key events for the subtask edit overlay.
func (m *TaskFormModel) handleSubtaskEditKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "enter", "ctrl+e":
		title := strings.TrimSpace(m.subtaskItemInput.Value())
		return nil, true, TaskFormSubtaskEditedMsg{
			Idx:         m.childrenEditIdx,
			Title:       title,
			OpenContent: msg.String() == "ctrl+e",
		}
	case "esc":
		return nil, true, nil // cancel
	default:
		var cmd tea.Cmd
		m.subtaskItemInput, cmd = m.subtaskItemInput.Update(msg)
		return cmd, false, nil
	}
}

// renderSubtaskEditOverlay renders the inline subtask title editor.
func (m *TaskFormModel) renderSubtaskEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Edit Subtask")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := min(max(maxWidth, titleMinInner), max(m.width-4, 1))
	footerText := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec(
		"Enter save", "Ctrl+E save & edit description",
	), max(innerWidth-4, 1))

	m.subtaskItemInput.SetWidth(max(innerWidth-6, 1))
	theme.ApplyThemedTextInput(&m.subtaskItemInput)

	return chrome.RenderOverlayBox(" "+m.subtaskItemInput.View(), innerWidth, title, footerText, borderStyle)
}
