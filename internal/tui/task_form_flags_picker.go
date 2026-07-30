package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
)

// TaskFormFlagsPickedMsg is emitted when the user confirms flag selection.
type TaskFormFlagsPickedMsg struct {
	Pinned bool
}

// formFlagsPicker is an inline overlay for toggling task flags.
type formFlagsPicker struct {
	cursor int
	pinned bool
}

func newFormFlagsPicker(pinned bool) *formFlagsPicker {
	return &formFlagsPicker{cursor: 0, pinned: pinned}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg). dismiss=true closes the overlay.
func (p *formFlagsPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "enter":
		return nil, true, TaskFormFlagsPickedMsg{Pinned: p.pinned}
	case "space", "x":
		p.pinned = !p.pinned
		return nil, false, nil
	}
	return nil, false, nil
}

func (p *formFlagsPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Set Flags")
	footerText := HelpStyle.Render("j/k: navigate · Space/x: toggle · Enter: confirm · Esc: cancel")

	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 40), width-4)
	innerWidth := min(max(maxWidth, footerMinInner), max(width-4, 1))

	type row struct {
		label string
		val   bool
	}
	rows := []row{
		{"Pinned", p.pinned},
	}

	var content strings.Builder
	for i, r := range rows {
		checked := icons.Get("checkbox.unchecked")
		if r.val {
			checked = icons.Get("checkbox.checked")
		}
		line := fmt.Sprintf(" %s %s", checked, r.label)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-4 {
			line += strings.Repeat(" ", innerWidth-4-lineWidth)
		}
		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(rows)-1 {
			content.WriteString("\n")
		}
	}

	return renderOverlayBox(content.String(), innerWidth, title, footerText, borderStyle)
}
