package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/util"
)

// TaskFormDueDatePickedMsg is emitted from the inline due-date preset picker.
// Date is nil when the user picks "clear".
type TaskFormDueDatePickedMsg struct {
	Date *time.Time
}

type formDueDatePicker struct {
	cursor int
}

func newFormDueDatePicker(current *time.Time) *formDueDatePicker {
	cursor := 0
	if current == nil {
		// Pre-select "clear"
		for i, e := range dueDatePickerEntries {
			if e.isClear {
				cursor = i
				break
			}
		}
	} else {
		today := util.StartOfDay(time.Now())
		for i, e := range dueDatePickerEntries {
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
func (p *formDueDatePicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	toMsg := func(e dueDatePickerEntry) tea.Msg {
		if e.isClear {
			return TaskFormDueDatePickedMsg{Date: nil}
		}
		t := util.StartOfDay(time.Now()).AddDate(0, 0, e.daysFromNow)
		return TaskFormDueDatePickedMsg{Date: &t}
	}
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "enter":
		return nil, true, toMsg(dueDatePickerEntries[p.cursor])
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil, false, nil
	case "down", "j":
		if p.cursor < len(dueDatePickerEntries)-1 {
			p.cursor++
		}
		return nil, false, nil
	default:
		key := msg.String()
		for _, e := range dueDatePickerEntries {
			if key == e.key {
				return nil, true, toMsg(e)
			}
		}
		return nil, false, nil
	}
}

func (p *formDueDatePicker) render(width int, current *time.Time) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	var titleText string
	if current != nil {
		titleText = fmt.Sprintf("Set Due Date: %s", current.Format("Mon Jan 2, 2006"))
	} else {
		titleText = "Set Due Date"
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(titleText)
	footerText := HelpStyle.Render("j/k · key: pick · Enter: select · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 36), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	today := util.StartOfDay(time.Now())
	currentEntryIdx := -1
	if current == nil {
		for i, e := range dueDatePickerEntries {
			if e.isClear {
				currentEntryIdx = i
				break
			}
		}
	} else {
		for i, e := range dueDatePickerEntries {
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
	for i, e := range dueDatePickerEntries {
		isCurrent := i == currentEntryIdx
		keyHint := MutedStyle.Render(fmt.Sprintf("[%s]", e.key))
		currentMark := "  "
		if isCurrent {
			currentMark = "● "
		}
		line := fmt.Sprintf(" %s%s %s", currentMark, e.label, keyHint)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}
		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(dueDatePickerEntries)-1 {
			content.WriteString("\n")
		}
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
