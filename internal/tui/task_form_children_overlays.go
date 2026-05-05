package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
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
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Edit Checklist Item")
	footerText := HelpStyle.Render("Enter: save · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	m.checklistItemInput.SetWidth(innerWidth - 4)
	applyThemedTextInput(&m.checklistItemInput)
	content := " " + m.checklistItemInput.View()

	return renderTitledBox(content, innerWidth, title, footerText, borderStyle)
}

// TaskFormSubtaskEditedMsg is emitted when a subtask draft edit is confirmed.
type TaskFormSubtaskEditedMsg struct {
	Idx         int
	Title       string
	TaskType    models.TaskType
	OpenDetail  bool
	OpenContent bool
}

// handleSubtaskEditKey handles key events for the subtask edit overlay.
func (m *TaskFormModel) handleSubtaskEditKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "enter", "ctrl+o", "ctrl+d":
		title := strings.TrimSpace(m.subtaskItemInput.Value())
		t := formSubtaskTypes[m.subtaskTypeCursor]
		return nil, true, TaskFormSubtaskEditedMsg{
			Idx:         m.childrenEditIdx,
			Title:       title,
			TaskType:    t,
			OpenDetail:  msg.String() == "ctrl+o",
			OpenContent: msg.String() == "ctrl+d",
		}
	case "esc":
		return nil, true, nil // cancel
	case "up", "k", "shift+tab":
		if m.subtaskTypeCursor > 0 {
			m.subtaskTypeCursor--
		}
		return nil, false, nil
	case "down", "j", "tab":
		if m.subtaskTypeCursor < len(formSubtaskTypes)-1 {
			m.subtaskTypeCursor++
		}
		return nil, false, nil
	default:
		var cmd tea.Cmd
		m.subtaskItemInput, cmd = m.subtaskItemInput.Update(msg)
		return cmd, false, nil
	}
}

// renderSubtaskEditOverlay renders the inline subtask edit overlay (title + type).
func (m *TaskFormModel) renderSubtaskEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Edit Subtask")
	footerText := HelpStyle.Render("j/k: type · Enter: save · Ctrl+O: save & open · Ctrl+D: save & edit desc · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	m.subtaskItemInput.SetWidth(innerWidth - 6)
	applyThemedTextInput(&m.subtaskItemInput)

	var content strings.Builder
	content.WriteString(" " + m.subtaskItemInput.View() + "\n")
	content.WriteString(" " + MutedStyle.Render("Type:") + "  ")
	for i, st := range formSubtaskTypes {
		badge := GetTypeBadge(string(st), st.Icon())
		if i == m.subtaskTypeCursor {
			content.WriteString(SelectorOptionSelectedStyle.Render(" " + badge + " " + string(st) + " "))
		} else {
			content.WriteString(MutedStyle.Render(" " + badge + " " + string(st) + " "))
		}
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
