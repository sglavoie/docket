package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TaskFormStatePickedMsg is emitted from the inline canonical state picker.
type TaskFormStatePickedMsg struct {
	State models.WorkflowState
}

type formStatePicker struct {
	cursor int
}

func newFormStatePicker(current models.WorkflowState) *formStatePicker {
	cursor := 0
	for i, state := range models.AllWorkflowStates() {
		if state == current {
			cursor = i
			break
		}
	}
	return &formStatePicker{cursor: cursor}
}

func (p *formStatePicker) handleKey(msg tea.KeyPressMsg) (bool, tea.Msg) {
	states := models.AllWorkflowStates()
	switch msg.String() {
	case "esc":
		return true, nil
	case "enter":
		return true, TaskFormStatePickedMsg{State: states[p.cursor]}
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(states)-1 {
			p.cursor++
		}
	}
	return false, nil
}

func (p *formStatePicker) render(width int, current models.WorkflowState) string {
	states := models.AllWorkflowStates()
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Set State")
	footer := HelpStyle.Render("j/k: move · Enter: select · Esc: cancel")
	minInnerWidth := max(lipgloss.Width(title), lipgloss.Width(footer)) + 4
	innerWidth := min(max(max(width*2/5, 36), minInnerWidth), max(width-4, 1))

	var content strings.Builder
	for i, state := range states {
		currentMark := "  "
		if state == current {
			currentMark = "● "
		}
		line := fmt.Sprintf(" %s%s %s", currentMark, state.Icon(), state.String())
		if lineWidth := lipgloss.Width(line); lineWidth < innerWidth-4 {
			line += strings.Repeat(" ", innerWidth-4-lineWidth)
		}
		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(states)-1 {
			content.WriteString("\n")
		}
	}
	return renderOverlayBox(content.String(), innerWidth, title, footer, borderStyle)
}
