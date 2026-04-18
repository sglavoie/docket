package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TaskFormTypePickedMsg is emitted when the user selects a type in formTypePicker.
type TaskFormTypePickedMsg struct {
	Type models.TaskType
}

// formTypePicker is an inline overlay for picking a task type in TaskFormModel.
type formTypePicker struct {
	cursor  int
	entries []models.TaskType
}

func newFormTypePicker(current models.TaskType) *formTypePicker {
	entries := models.AllTaskTypes()
	cursor := 0
	for i, t := range entries {
		if t == current {
			cursor = i
			break
		}
	}
	return &formTypePicker{cursor: cursor, entries: entries}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg). dismiss=true closes the overlay.
func (p *formTypePicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "enter":
		return nil, true, TaskFormTypePickedMsg{Type: p.entries[p.cursor]}
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil, false, nil
	case "down", "j":
		if p.cursor < len(p.entries)-1 {
			p.cursor++
		}
		return nil, false, nil
	}
	return nil, false, nil
}

func (p *formTypePicker) render(width int, current models.TaskType) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Set Type")
	footerText := HelpStyle.Render("j/k: navigate · Enter: select · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 36), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var content strings.Builder
	for i, t := range p.entries {
		icon := GetTypeBadge(string(t), t.Icon())
		isCurrent := t == current
		currentMark := "  "
		if isCurrent {
			currentMark = "● "
		}
		line := fmt.Sprintf(" %s%s %s", currentMark, icon, t.HeadingPrefix())
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}
		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(p.entries)-1 {
			content.WriteString("\n")
		}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(innerWidth)

	rendered := boxStyle.Render(content.String())
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}
	actualWidth := lipgloss.Width(lines[0])
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	lines[len(lines)-1] = renderTitledBottomBorder(footerText, actualWidth, borderStyle)
	return strings.Join(lines, "\n")
}
