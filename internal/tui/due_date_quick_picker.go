package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// dueDatePickerEntry is a single entry in the DueDateQuickPicker.
type dueDatePickerEntry struct {
	key         string // direct key shortcut
	label       string // human-readable label
	daysFromNow int
	isClear     bool
}

// dueDatePickerEntries lists all options in display order with their direct key shortcuts.
var dueDatePickerEntries = []dueDatePickerEntry{
	{key: "t", label: "today", daysFromNow: 0},
	{key: "m", label: "tomorrow", daysFromNow: 1},
	{key: "w", label: "+week", daysFromNow: 7},
	{key: "n", label: "+month", daysFromNow: 30},
	{key: "0", label: "clear", isClear: true},
}

// GlobalDueDateAppliedMsg is emitted when the user picks a due date option in DueDateQuickPicker.
type GlobalDueDateAppliedMsg struct {
	Task        *models.Task
	Project     *models.Project
	DaysFromNow int
	Clear       bool
}

// DueDateQuickPicker is an overlay that lets the user set a quick due date for a task.
// Esc emits backMsg to return to the previous step (GlobalTaskPicker).
type DueDateQuickPicker struct {
	task    *models.Task
	project *models.Project
	cursor  int
	backMsg tea.Msg
}

var dueDatePickerBreadcrumb = []string{"Select Project", "Select Task", "Set Due Date"}

// newDueDateQuickPicker creates a picker with the cursor on the entry matching
// the task's current due date, or on "clear" if no due date is set.
func newDueDateQuickPicker(task *models.Task, project *models.Project, backMsg tea.Msg) *DueDateQuickPicker {
	cursor := 0
	if task.DueDate == nil {
		// Pre-select "clear" entry
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
			target := today.AddDate(0, 0, e.daysFromNow)
			if task.DueDate.Equal(target) {
				cursor = i
				break
			}
		}
	}
	return &DueDateQuickPicker{
		task:    task,
		project: project,
		cursor:  cursor,
		backMsg: backMsg,
	}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg).
func (p *DueDateQuickPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, p.backMsg

	case "enter":
		e := dueDatePickerEntries[p.cursor]
		return nil, true, GlobalDueDateAppliedMsg{
			Task:        p.task,
			Project:     p.project,
			DaysFromNow: e.daysFromNow,
			Clear:       e.isClear,
		}

	case "up", "ctrl+k", "ctrl+p", "k":
		p.moveCursor(-1)
		return nil, false, nil

	case "down", "ctrl+j", "ctrl+n", "j":
		p.moveCursor(1)
		return nil, false, nil

	default:
		key := msg.String()
		for _, e := range dueDatePickerEntries {
			if key == e.key {
				return nil, true, GlobalDueDateAppliedMsg{
					Task:        p.task,
					Project:     p.project,
					DaysFromNow: e.daysFromNow,
					Clear:       e.isClear,
				}
			}
		}
		return nil, false, nil
	}
}

func (p *DueDateQuickPicker) moveCursor(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(dueDatePickerEntries) {
		p.cursor = len(dueDatePickerEntries) - 1
	}
}

func (p *DueDateQuickPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	taskTitle := p.task.Title
	const maxTitleLen = 40
	if len(taskTitle) > maxTitleLen {
		taskTitle = taskTitle[:maxTitleLen-1] + "…"
	}

	titleText := "Set Due Date"
	if p.task.DueDate != nil {
		dateStr := p.task.DueDate.Format("Mon Jan 2, 2006")
		dateStr = AppendTimeIfHasTime(p.task, dateStr, "15:04")
		titleText = fmt.Sprintf("Set Due Date: %s", dateStr)
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(
		fmt.Sprintf("%s — %s", titleText, taskTitle),
	)
	footerText := HelpStyle.Render("Enter/key: select · j/k: navigate · Esc: back")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 40), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	// Determine the "current" entry based on the task's existing due date.
	today := util.StartOfDay(time.Now())
	currentEntryIdx := -1
	if p.task.DueDate == nil {
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
			if p.task.DueDate.Equal(today.AddDate(0, 0, e.daysFromNow)) {
				currentEntryIdx = i
				break
			}
		}
	}

	var content strings.Builder
	if bc := renderStepBreadcrumb(dueDatePickerBreadcrumb, 2); bc != "" {
		content.WriteString(bc)
		content.WriteString("\n")
	}

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
