package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// statusPickerEntry is a single entry in the StatusQuickPicker.
type statusPickerEntry struct {
	status models.TaskStatus
	key    string // direct key shortcut
	label  string // human-readable label
}

// statusPickerEntries lists all statuses in display order with their direct key shortcuts.
var statusPickerEntries = []statusPickerEntry{
	{models.StatusTodo, "o", "todo"},
	{models.StatusInProgress, "s", "in progress"},
	{models.StatusReview, "r", "review"},
	{models.StatusStandby, "S", "standby"},
	{models.StatusDone, "d", "done"},
	{models.StatusBlocked, "b", "blocked"},
	{models.StatusCanceled, "x", "canceled"},
}

// GlobalStatusAppliedMsg is emitted when the user picks a new status in StatusQuickPicker.
type GlobalStatusAppliedMsg struct {
	Task      *models.Task
	Project   *models.Project
	NewStatus models.TaskStatus
}

// StatusQuickPicker is an overlay that lets the user pick a new status for a task.
// Esc emits backMsg to return to the previous step (GlobalTaskPicker).
type StatusQuickPicker struct {
	task    *models.Task
	project *models.Project
	cursor  int
	backMsg tea.Msg
}

var statusPickerBreadcrumb = []string{"Select Project", "Select Task", "Choose Status"}

// newStatusQuickPicker creates a picker pre-selected on the task's current status.
func newStatusQuickPicker(task *models.Task, project *models.Project, backMsg tea.Msg) *StatusQuickPicker {
	cursor := 0
	for i, e := range statusPickerEntries {
		if e.status == task.Status {
			cursor = i
			break
		}
	}
	return &StatusQuickPicker{
		task:    task,
		project: project,
		cursor:  cursor,
		backMsg: backMsg,
	}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg).
func (p *StatusQuickPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, p.backMsg

	case "enter":
		return nil, true, GlobalStatusAppliedMsg{
			Task:      p.task,
			Project:   p.project,
			NewStatus: statusPickerEntries[p.cursor].status,
		}

	case "up", "ctrl+k", "ctrl+p", "k":
		p.moveCursor(-1)
		return nil, false, nil

	case "down", "ctrl+j", "ctrl+n", "j":
		p.moveCursor(1)
		return nil, false, nil

	default:
		key := msg.String()
		for _, e := range statusPickerEntries {
			if key == e.key {
				return nil, true, GlobalStatusAppliedMsg{
					Task:      p.task,
					Project:   p.project,
					NewStatus: e.status,
				}
			}
		}
		return nil, false, nil
	}
}

func (p *StatusQuickPicker) moveCursor(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(statusPickerEntries) {
		p.cursor = len(statusPickerEntries) - 1
	}
}

func (p *StatusQuickPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	// Title shows the task's short title
	taskTitle := p.task.Title
	const maxTitleLen = 40
	if len(taskTitle) > maxTitleLen {
		taskTitle = taskTitle[:maxTitleLen-1] + "…"
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(
		fmt.Sprintf("Set Status: %s", taskTitle),
	)
	footerText := HelpStyle.Render("Enter/key: select · j/k: navigate · Esc: back")

	// Compute inner width
	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 40), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var content strings.Builder
	if bc := renderStepBreadcrumb(statusPickerBreadcrumb, 2); bc != "" {
		content.WriteString(bc)
		content.WriteString("\n")
	}

	for i, e := range statusPickerEntries {
		icon := e.status.Icon()
		isCurrent := e.status == p.task.Status
		keyHint := MutedStyle.Render(fmt.Sprintf("[%s]", e.key))

		currentMark := "  "
		if isCurrent {
			currentMark = "● "
		}

		line := fmt.Sprintf(" %s%s %s %s", currentMark, icon, e.label, keyHint)

		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}

		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}

		content.WriteString(line)
		if i < len(statusPickerEntries)-1 {
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
