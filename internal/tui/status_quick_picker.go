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
	state models.WorkflowState
	key   string // direct key shortcut
	label string // human-readable label
}

// statusPickerEntries lists all ten canonical workflow states in display order
// with their direct key shortcuts. The keys mirror the Fw workflow-state filter
// sub-prefix (i/r/n/s/w/p/v/m/d/x) so the same physical key means the same state
// in the picker and the filter.
var statusPickerEntries = []statusPickerEntry{
	{models.StateInbox, "i", "inbox"},
	{models.StateReady, "r", "ready"},
	{models.StateNext, "n", "next"},
	{models.StateInProgress, "s", "in progress"},
	{models.StateWaiting, "w", "waiting"},
	{models.StatePaused, "p", "paused"},
	{models.StateReview, "v", "review"},
	{models.StateSomeday, "m", "someday"},
	{models.StateDone, "d", "done"},
	{models.StateCanceled, "x", "canceled"},
}

// GlobalStatusAppliedMsg is emitted when the user picks a new workflow state in
// StatusQuickPicker.
type GlobalStatusAppliedMsg struct {
	Task     *models.Task
	Project  *models.Project
	NewState models.WorkflowState
}

// StatusQuickPicker is an overlay that lets the user pick a new status for a task.
// Esc emits backMsg to return to the previous step (GlobalTaskPicker).
type StatusQuickPicker struct {
	task    *models.Task
	project *models.Project
	cursor  int
	backMsg tea.Msg
}

var statusPickerBreadcrumb = []string{"Select Project", "Select Task", "Choose State"}

// newStatusQuickPicker creates a picker pre-selected on the task's current status.
func newStatusQuickPicker(task *models.Task, project *models.Project, backMsg tea.Msg) *StatusQuickPicker {
	cursor := 0
	for i, e := range statusPickerEntries {
		if task.IsInState(e.state) {
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
			Task:     p.task,
			Project:  p.project,
			NewState: statusPickerEntries[p.cursor].state,
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
					Task:     p.task,
					Project:  p.project,
					NewState: e.state,
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
		fmt.Sprintf("Set State: %s", taskTitle),
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
		icon := e.state.Icon()
		isCurrent := p.task.IsInState(e.state)
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

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
