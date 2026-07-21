package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// =============================================================================
// LinkTypeQuickPicker — Step 3 of global linked task creation
// =============================================================================

// linkTypeEntry is a single entry in the LinkTypeQuickPicker.
type linkTypeEntry struct {
	linkType    LinkType
	key         string
	label       string
	description string
}

var linkedTaskFlowBreadcrumb = []string{"Select Project", "Select Task", "Link Type", "Link Mode"}

var linkTypeEntries = []linkTypeEntry{
	{LinkTypeBlocker, "b", "blocker", "new task blocks source"},
	{LinkTypeDependent, "d", "dependent", "new task depends on source"},
	{LinkTypeSubtask, "s", "subtask", "new task is a subtask of source"},
	{LinkTypeFollowUp, "f", "follow-up", "new task follows up on source"},
}

// ShowGlobalLinkedTaskTypePickerMsg opens the link type picker for the given task.
// Used as the backMsg for LinkModeQuickPicker so Esc returns to step 3.
type ShowGlobalLinkedTaskTypePickerMsg struct {
	Task    *models.Task
	Project *models.Project
}

// GlobalLinkedTaskTypeSelectedMsg is emitted when the user picks a link type.
type GlobalLinkedTaskTypeSelectedMsg struct {
	Task     *models.Task
	Project  *models.Project
	LinkType LinkType
}

// LinkTypeQuickPicker is an overlay for selecting the link type.
// Esc emits backMsg to return to the task picker.
type LinkTypeQuickPicker struct {
	task    *models.Task
	project *models.Project
	cursor  int
	backMsg tea.Msg
}

func newLinkTypeQuickPicker(task *models.Task, project *models.Project, backMsg tea.Msg) *LinkTypeQuickPicker {
	return &LinkTypeQuickPicker{
		task:    task,
		project: project,
		cursor:  0,
		backMsg: backMsg,
	}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg).
func (p *LinkTypeQuickPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, p.backMsg

	case "enter":
		return nil, true, GlobalLinkedTaskTypeSelectedMsg{
			Task:     p.task,
			Project:  p.project,
			LinkType: linkTypeEntries[p.cursor].linkType,
		}

	case "up", "ctrl+k", "ctrl+p", "k":
		p.moveCursor(-1)
		return nil, false, nil

	case "down", "ctrl+j", "ctrl+n", "j":
		p.moveCursor(1)
		return nil, false, nil

	default:
		key := msg.String()
		for _, e := range linkTypeEntries {
			if key == e.key {
				return nil, true, GlobalLinkedTaskTypeSelectedMsg{
					Task:     p.task,
					Project:  p.project,
					LinkType: e.linkType,
				}
			}
		}
		return nil, false, nil
	}
}

func (p *LinkTypeQuickPicker) moveCursor(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(linkTypeEntries) {
		p.cursor = len(linkTypeEntries) - 1
	}
}

func (p *LinkTypeQuickPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	taskTitle := p.task.Title
	const maxTitleLen = 40
	if len(taskTitle) > maxTitleLen {
		taskTitle = taskTitle[:maxTitleLen-1] + "…"
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(
		fmt.Sprintf("Link Type: %s", taskTitle),
	)
	footerText := HelpStyle.Render("Enter/key: select · j/k: navigate · Esc: back")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 40), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var content strings.Builder
	if bc := renderStepBreadcrumb(linkedTaskFlowBreadcrumb, 2); bc != "" {
		content.WriteString(bc)
		content.WriteString("\n")
	}

	for i, e := range linkTypeEntries {
		keyHint := MutedStyle.Render(fmt.Sprintf("[%s]", e.key))
		line := fmt.Sprintf(" %s %s %s", keyHint, e.label, MutedStyle.Render("— "+e.description))

		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}

		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}

		content.WriteString(line)
		if i < len(linkTypeEntries)-1 {
			content.WriteString("\n")
		}
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}

// =============================================================================
// LinkModeQuickPicker — Step 4 of global linked task creation
// =============================================================================

// linkModeEntry is a single entry in the LinkModeQuickPicker.
type linkModeEntry struct {
	key   string
	label string
	desc  string
}

var linkModeEntries = []linkModeEntry{
	{"n", "Create new task", "open quick capture with link pre-set"},
	{"e", "Link existing task", "open cross-project linker"},
}

// GlobalLinkedTaskModeCreateMsg is emitted when the user chooses "create new".
type GlobalLinkedTaskModeCreateMsg struct {
	Task     *models.Task
	Project  *models.Project
	LinkType LinkType
}

// GlobalLinkedTaskModeLinkMsg is emitted when the user chooses "link existing".
type GlobalLinkedTaskModeLinkMsg struct {
	Task    *models.Task
	Project *models.Project
}

// LinkModeQuickPicker is an overlay for choosing between creating a new linked
// task or linking an existing one.
// Esc emits backMsg to return to the link type picker.
type LinkModeQuickPicker struct {
	task     *models.Task
	project  *models.Project
	linkType LinkType
	cursor   int
	backMsg  tea.Msg
}

func newLinkModeQuickPicker(task *models.Task, project *models.Project, linkType LinkType, backMsg tea.Msg) *LinkModeQuickPicker {
	return &LinkModeQuickPicker{
		task:     task,
		project:  project,
		linkType: linkType,
		cursor:   0,
		backMsg:  backMsg,
	}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg).
func (p *LinkModeQuickPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, p.backMsg

	case "enter":
		return p.selectCurrent()

	case "up", "ctrl+k", "ctrl+p", "k":
		p.moveCursor(-1)
		return nil, false, nil

	case "down", "ctrl+j", "ctrl+n", "j":
		p.moveCursor(1)
		return nil, false, nil

	default:
		key := msg.String()
		for i, e := range linkModeEntries {
			if key == e.key {
				p.cursor = i
				return p.selectCurrent()
			}
		}
		return nil, false, nil
	}
}

func (p *LinkModeQuickPicker) selectCurrent() (tea.Cmd, bool, tea.Msg) {
	switch p.cursor {
	case 0: // Create new
		return nil, true, GlobalLinkedTaskModeCreateMsg{
			Task:     p.task,
			Project:  p.project,
			LinkType: p.linkType,
		}
	case 1: // Link existing
		return nil, true, GlobalLinkedTaskModeLinkMsg{
			Task:    p.task,
			Project: p.project,
		}
	}
	return nil, false, nil
}

func (p *LinkModeQuickPicker) moveCursor(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(linkModeEntries) {
		p.cursor = len(linkModeEntries) - 1
	}
}

func (p *LinkModeQuickPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	linkTypeNames := map[LinkType]string{
		LinkTypeBlocker:   "blocker",
		LinkTypeDependent: "dependent",
		LinkTypeSubtask:   "subtask",
		LinkTypeFollowUp:  "follow-up",
	}
	linkName := linkTypeNames[p.linkType]

	taskTitle := p.task.Title
	const maxTitleLen = 40
	if len(taskTitle) > maxTitleLen {
		taskTitle = taskTitle[:maxTitleLen-1] + "…"
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(
		fmt.Sprintf("Add %s: %s", linkName, taskTitle),
	)
	footerText := HelpStyle.Render("Enter/key: select · j/k: navigate · Esc: back")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 40), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var content strings.Builder
	if bc := renderStepBreadcrumb(linkedTaskFlowBreadcrumb, 3); bc != "" {
		content.WriteString(bc)
		content.WriteString("\n")
	}

	for i, e := range linkModeEntries {
		keyHint := MutedStyle.Render(fmt.Sprintf("[%s]", e.key))
		line := fmt.Sprintf(" %s %s %s", keyHint, e.label, MutedStyle.Render("— "+e.desc))

		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}

		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}

		content.WriteString(line)
		if i < len(linkModeEntries)-1 {
			content.WriteString("\n")
		}
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
