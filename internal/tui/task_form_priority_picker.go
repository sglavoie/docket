package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TaskFormPriorityPickedMsg is emitted from the inline priority picker.
type TaskFormPriorityPickedMsg struct {
	Priority int
}

var formPriorityEntries = []struct {
	priority int
	key      string
	label    string
}{
	{1, "1", "critical"},
	{2, "2", "high"},
	{3, "3", "normal"},
	{4, "4", "low"},
	{5, "5", "someday"},
}

type formPriorityPicker struct {
	cursor int
}

func newFormPriorityPicker(current int) *formPriorityPicker {
	cursor := 2 // default: normal
	for i, e := range formPriorityEntries {
		if e.priority == current {
			cursor = i
			break
		}
	}
	return &formPriorityPicker{cursor: cursor}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg). dismiss=true closes the overlay.
func (p *formPriorityPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "enter":
		return nil, true, TaskFormPriorityPickedMsg{Priority: formPriorityEntries[p.cursor].priority}
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil, false, nil
	case "down", "j":
		if p.cursor < len(formPriorityEntries)-1 {
			p.cursor++
		}
		return nil, false, nil
	default:
		for _, e := range formPriorityEntries {
			if msg.String() == e.key {
				return nil, true, TaskFormPriorityPickedMsg{Priority: e.priority}
			}
		}
		return nil, false, nil
	}
}

func (p *formPriorityPicker) render(width, current int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Set Priority")
	footerText := HelpStyle.Render("j/k · 1-5: pick · Enter: select · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 36), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var content strings.Builder
	for i, e := range formPriorityEntries {
		icon := GetPriorityStyle(e.priority).Render(PriorityIcon(e.priority))
		isCurrent := e.priority == current
		currentMark := "  "
		if isCurrent {
			currentMark = "● "
		}
		keyHint := MutedStyle.Render(fmt.Sprintf("[%s]", e.key))
		line := fmt.Sprintf(" %s%s %s %s", currentMark, icon, e.label, keyHint)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}
		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(formPriorityEntries)-1 {
			content.WriteString("\n")
		}
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
