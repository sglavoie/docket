package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// TaskFormDueDatePickedMsg is emitted from the inline due-date preset picker.
// Date is nil when the user picks "clear".
type TaskFormDueDatePickedMsg struct {
	Date *time.Time
}

type formDueDatePreset struct {
	key         string
	label       string
	daysFromNow int
	isClear     bool
}

var formDueDatePresets = []formDueDatePreset{
	{key: "t", label: "today", daysFromNow: 0},
	{key: "m", label: "tomorrow", daysFromNow: 1},
	{key: "w", label: "+week", daysFromNow: 7},
	{key: "n", label: "+month", daysFromNow: 30},
	{key: "0", label: "clear", isClear: true},
}

type formDueDatePicker struct {
	cursor int
}

func newFormDueDatePicker(current *time.Time) *formDueDatePicker {
	cursor := 0
	if current == nil {
		// Pre-select "clear"
		for i, e := range formDueDatePresets {
			if e.isClear {
				cursor = i
				break
			}
		}
	} else {
		today := util.StartOfDay(time.Now())
		for i, e := range formDueDatePresets {
			if e.isClear {
				continue
			}
			if current.Equal(today.AddDate(0, 0, e.daysFromNow)) {
				cursor = i
				break
			}
		}
	}
	return &formDueDatePicker{cursor: cursor}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg). dismiss=true closes the overlay.
func (p *formDueDatePicker) handleKey(msg tea.KeyPressMsg) (bool, tea.Msg) {
	toMsg := func(e formDueDatePreset) tea.Msg {
		if e.isClear {
			return TaskFormDueDatePickedMsg{Date: nil}
		}
		t := util.StartOfDay(time.Now()).AddDate(0, 0, e.daysFromNow)
		return TaskFormDueDatePickedMsg{Date: &t}
	}
	switch msg.String() {
	case "esc":
		return true, nil
	case "enter":
		return true, toMsg(formDueDatePresets[p.cursor])
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return false, nil
	case "down", "j":
		if p.cursor < len(formDueDatePresets)-1 {
			p.cursor++
		}
		return false, nil
	default:
		key := msg.String()
		for _, e := range formDueDatePresets {
			if key == e.key {
				return true, toMsg(e)
			}
		}
		return false, nil
	}
}

func (p *formDueDatePicker) render(width int, current *time.Time) string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)

	var titleText string
	if current != nil {
		titleText = fmt.Sprintf("Set Due Date: %s", current.Format("Mon Jan 2, 2006"))
	} else {
		titleText = "Set Due Date"
	}
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(titleText)

	titleMinInner := lipgloss.Width(title) + 6 - 2
	maxWidth := min(max(width*2/5, 36), width-4)
	innerWidth := min(max(maxWidth, titleMinInner), max(width-4, 1))
	footerText := renderGuidanceText(requiredActionGuidanceSpec(
		"Enter select", "j/k navigate", "preset key pick",
	), max(innerWidth-4, 1))

	today := util.StartOfDay(time.Now())
	currentEntryIdx := -1
	if current == nil {
		for i, e := range formDueDatePresets {
			if e.isClear {
				currentEntryIdx = i
				break
			}
		}
	} else {
		for i, e := range formDueDatePresets {
			if e.isClear {
				continue
			}
			if current.Equal(today.AddDate(0, 0, e.daysFromNow)) {
				currentEntryIdx = i
				break
			}
		}
	}

	var content strings.Builder
	for i, e := range formDueDatePresets {
		isCurrent := i == currentEntryIdx
		keyHint := theme.MutedStyle.Render(fmt.Sprintf("[%s]", e.key))
		currentMark := "  "
		if isCurrent {
			currentMark = "● "
		}
		line := fmt.Sprintf(" %s%s %s", currentMark, e.label, keyHint)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-4 {
			line += strings.Repeat(" ", innerWidth-4-lineWidth)
		}
		if i == p.cursor {
			line = theme.SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(formDueDatePresets)-1 {
			content.WriteString("\n")
		}
	}

	return renderOverlayBox(content.String(), innerWidth, title, footerText, borderStyle)
}
