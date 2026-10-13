package taskform

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// TaskFormDueDatePickedMsg is emitted from the inline due-date preset picker.
// Date is nil when the user picks "clear".
type TaskFormDueDatePickedMsg struct {
	Date *time.Time
}

// The form's rows are named by the shared preset identity rather than by a key
// of their own, so the key this picker prints and answers to is the one the
// Outliner's due section prints and answers to. The order, the labels, and the
// day arithmetic below stay the form's: this list is a draft editor, and it
// borrows the keyboard table without borrowing the other surface's vocabulary.
type formDueDatePreset struct {
	presetID    string
	label       string
	daysFromNow int
	isClear     bool
}

var formDueDatePresets = []formDueDatePreset{
	{presetID: chrome.DatePresetToday, label: "today", daysFromNow: 0},
	{presetID: chrome.DatePresetTomorrow, label: "tomorrow", daysFromNow: 1},
	{presetID: chrome.DatePresetWeek, label: "+week", daysFromNow: 7},
	{presetID: chrome.DatePresetMonth, label: "+30 days", daysFromNow: 30},
	{presetID: chrome.DatePresetClear, label: "clear", isClear: true},
}

// key is the shortcut the shared table binds to this preset in the due field. An
// unkeyed preset would be a row nothing reaches by key, which is why
// TestFormDueDatePresetsAreAllKeyed refuses to let one exist.
func (e formDueDatePreset) key() string {
	key, _ := chrome.DatePresetKey(false, e.presetID)
	return key
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
			if key == e.key() {
				return true, toMsg(e)
			}
		}
		return false, nil
	}
}

func (p *formDueDatePicker) render(width int, current *time.Time) string {
	return p.renderAt(width, current, time.Now())
}

func (p *formDueDatePicker) renderAt(width int, current *time.Time, now time.Time) string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)

	var titleText string
	if current != nil {
		titleText = fmt.Sprintf("Set Due Date: %s", chrome.FormatWeekdayDate(*current, now, false))
	} else {
		titleText = "Set Due Date"
	}
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(titleText)

	innerWidth := formOverlayInnerWidth(width, max(width*2/5, 48), formOverlayTitleMinInner(title))
	footerText := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec(
		"Enter select", "j/k navigate", "preset key pick",
	), max(innerWidth-4, 1))

	today := util.StartOfDay(now)
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
		keyHint := theme.MutedStyle.Render(fmt.Sprintf("[%s]", e.key()))
		currentMark := "  "
		if isCurrent {
			currentMark = "● "
		}
		line := fmt.Sprintf(" %s%s %s", currentMark, e.label, keyHint)
		if !e.isClear {
			date := today.AddDate(0, 0, e.daysFromNow)
			line += theme.MutedStyle.Render(" · " + chrome.FormatWeekdayDate(date, now, false))
		}
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

	return chrome.RenderOverlayBox(content.String(), innerWidth, title, footerText, borderStyle)
}
