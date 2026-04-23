package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TaskFormFlagsPickedMsg is emitted when the user confirms flag selection.
type TaskFormFlagsPickedMsg struct {
	Pinned bool
	Active bool
	Dimmed bool
}

// formFlagsPicker is an inline overlay for toggling task flags (Pinned/Active/Dimmed).
type formFlagsPicker struct {
	cursor int
	pinned bool
	active bool
	dimmed bool
}

func newFormFlagsPicker(pinned, active, dimmed bool) *formFlagsPicker {
	return &formFlagsPicker{cursor: 0, pinned: pinned, active: active, dimmed: dimmed}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg). dismiss=true closes the overlay.
func (p *formFlagsPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "enter":
		return nil, true, TaskFormFlagsPickedMsg{Pinned: p.pinned, Active: p.active, Dimmed: p.dimmed}
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil, false, nil
	case "down", "j":
		if p.cursor < 2 {
			p.cursor++
		}
		return nil, false, nil
	case "space", "x":
		switch p.cursor {
		case 0:
			p.pinned = !p.pinned
		case 1:
			p.active = !p.active
			// Active and Dimmed are mutually exclusive (see task_lifecycle.go).
			if p.active {
				p.dimmed = false
			}
		case 2:
			p.dimmed = !p.dimmed
			// Active and Dimmed are mutually exclusive (see task_lifecycle.go).
			if p.dimmed {
				p.active = false
			}
		}
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
	innerWidth := max(maxWidth, footerMinInner)

	type row struct {
		label string
		val   bool
	}
	rows := []row{
		{"Pinned", p.pinned},
		{"Active", p.active},
		{"Dimmed", p.dimmed},
	}

	var content strings.Builder
	for i, r := range rows {
		checked := "☐"
		if r.val {
			checked = "☑"
		}
		line := fmt.Sprintf(" %s %s", checked, r.label)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}
		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(rows)-1 {
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
