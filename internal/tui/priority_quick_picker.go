package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// priorityPickerEntry is a single entry in the PriorityQuickPicker.
type priorityPickerEntry struct {
	priority int
	key      string // direct key shortcut
	label    string // human-readable label
}

// priorityPickerEntries lists all priorities in display order with their direct key shortcuts.
var priorityPickerEntries = []priorityPickerEntry{
	{1, "1", "critical"},
	{2, "2", "high"},
	{3, "3", "normal"},
	{4, "4", "low"},
	{5, "5", "someday"},
}

// GlobalPriorityAppliedMsg is emitted when the user picks a new priority in PriorityQuickPicker.
type GlobalPriorityAppliedMsg struct {
	Task        *models.Task
	Project     *models.Project
	NewPriority int
}

// PriorityQuickPicker is an overlay that lets the user pick a new priority for a task.
// Esc emits backMsg to return to the previous step (GlobalTaskPicker).
type PriorityQuickPicker struct {
	task    *models.Task
	project *models.Project
	cursor  int
	backMsg tea.Msg
}

var priorityPickerBreadcrumb = []string{"Select Project", "Select Task", "Set Priority"}

// newPriorityQuickPicker creates a picker pre-selected on the task's current priority.
func newPriorityQuickPicker(task *models.Task, project *models.Project, backMsg tea.Msg) *PriorityQuickPicker {
	cursor := 0
	for i, e := range priorityPickerEntries {
		if e.priority == task.Priority {
			cursor = i
			break
		}
	}
	return &PriorityQuickPicker{
		task:    task,
		project: project,
		cursor:  cursor,
		backMsg: backMsg,
	}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg).
func (p *PriorityQuickPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, p.backMsg

	case "enter":
		return nil, true, GlobalPriorityAppliedMsg{
			Task:        p.task,
			Project:     p.project,
			NewPriority: priorityPickerEntries[p.cursor].priority,
		}

	case "up", "ctrl+k", "ctrl+p", "k":
		p.moveCursor(-1)
		return nil, false, nil

	case "down", "ctrl+j", "ctrl+n", "j":
		p.moveCursor(1)
		return nil, false, nil

	default:
		key := msg.String()
		for _, e := range priorityPickerEntries {
			if key == e.key {
				return nil, true, GlobalPriorityAppliedMsg{
					Task:        p.task,
					Project:     p.project,
					NewPriority: e.priority,
				}
			}
		}
		return nil, false, nil
	}
}

func (p *PriorityQuickPicker) moveCursor(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(priorityPickerEntries) {
		p.cursor = len(priorityPickerEntries) - 1
	}
}

func (p *PriorityQuickPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	taskTitle := p.task.Title
	const maxTitleLen = 40
	if len(taskTitle) > maxTitleLen {
		taskTitle = taskTitle[:maxTitleLen-1] + "…"
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(
		fmt.Sprintf("Set Priority: %s", taskTitle),
	)
	footerText := HelpStyle.Render("Enter/key: select · j/k: navigate · Esc: back")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 40), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var content strings.Builder
	if bc := renderStepBreadcrumb(priorityPickerBreadcrumb, 2); bc != "" {
		content.WriteString(bc)
		content.WriteString("\n")
	}

	for i, e := range priorityPickerEntries {
		icon := GetPriorityStyle(e.priority).Render(PriorityIcon(e.priority))
		isCurrent := e.priority == p.task.Priority
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
		if i < len(priorityPickerEntries)-1 {
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
