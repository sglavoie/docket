package taskform

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
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
	{0, "0", "Unset"},
	{1, "1", "High"},
	{2, "2", "Medium"},
	{3, "3", "Low"},
}

type formPriorityPicker struct {
	cursor int
}

func newFormPriorityPicker(current int) *formPriorityPicker {
	cursor := 0 // default: unset
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
func (p *formPriorityPicker) handleKey(msg tea.KeyPressMsg) (bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return true, nil
	case "enter":
		return true, TaskFormPriorityPickedMsg{Priority: formPriorityEntries[p.cursor].priority}
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return false, nil
	case "down", "j":
		if p.cursor < len(formPriorityEntries)-1 {
			p.cursor++
		}
		return false, nil
	default:
		for _, e := range formPriorityEntries {
			if msg.String() == e.key {
				return true, TaskFormPriorityPickedMsg{Priority: e.priority}
			}
		}
		return false, nil
	}
}

func (p *formPriorityPicker) render(width, current int) string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Set Priority")

	innerWidth := formOverlayInnerWidth(width, max(width*2/5, 36), formOverlayTitleMinInner(title))
	footerText := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec(
		"Enter select", "j/k navigate", "0-3 pick",
	), max(innerWidth-4, 1))

	var content strings.Builder
	for i, e := range formPriorityEntries {
		icon := theme.GetPriorityStyle(e.priority).Render(theme.PriorityIcon(e.priority))
		isCurrent := e.priority == current
		currentMark := "  "
		if isCurrent {
			currentMark = "● "
		}
		keyHint := theme.MutedStyle.Render(fmt.Sprintf("[%s]", e.key))
		line := fmt.Sprintf(" %s%s %s %s", currentMark, icon, e.label, keyHint)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-4 {
			line += strings.Repeat(" ", innerWidth-4-lineWidth)
		}
		if i == p.cursor {
			line = theme.SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(formPriorityEntries)-1 {
			content.WriteString("\n")
		}
	}

	return chrome.RenderOverlayBox(content.String(), innerWidth, title, footerText, borderStyle)
}
