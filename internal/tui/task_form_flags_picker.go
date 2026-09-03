package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TaskFormFlagsPickedMsg is emitted when the user confirms flag selection.
type TaskFormFlagsPickedMsg struct {
	Pinned bool
	Review bool
}

// formFlagsPicker is an inline overlay for toggling task flags. Both flags are
// stored booleans the task carries beside its lifecycle state, so they share one
// overlay rather than each growing a form row of its own.
type formFlagsPicker struct {
	cursor int
	pinned bool
	review bool
}

func newFormFlagsPicker(pinned, review bool) *formFlagsPicker {
	return &formFlagsPicker{cursor: 0, pinned: pinned, review: review}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg). dismiss=true closes the overlay.
func (p *formFlagsPicker) handleKey(msg tea.KeyPressMsg) (bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return true, nil
	case "enter":
		return true, TaskFormFlagsPickedMsg{Pinned: p.pinned, Review: p.review}
	case "j", "down":
		p.cursor = min(p.cursor+1, formFlagsPickerRowCount-1)
		return false, nil
	case "k", "up":
		p.cursor = max(p.cursor-1, 0)
		return false, nil
	case "space", "x":
		if p.cursor == 0 {
			p.pinned = !p.pinned
			return false, nil
		}
		p.review = !p.review
		return false, nil
	}
	return false, nil
}

// formFlagsPickerRowCount is the number of toggles the overlay lists, which is
// what bounds its cursor.
const formFlagsPickerRowCount = 2

func (p *formFlagsPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Set Flags")

	maxWidth := min(max(width*2/5, 40), width-4)
	innerWidth := min(maxWidth, max(width-4, 1))
	footerText := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec(
		"Enter confirm", "Space/x toggle", "j/k navigate",
	), max(innerWidth-4, 1))

	type row struct {
		label string
		val   bool
	}
	rows := []row{
		{"Pinned", p.pinned},
		{"Review", p.review},
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
			line = theme.SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(rows)-1 {
			content.WriteString("\n")
		}
	}

	return chrome.RenderOverlayBox(content.String(), innerWidth, title, footerText, borderStyle)
}
