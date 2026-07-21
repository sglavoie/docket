package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// sectionPickerEntry is a single entry in the SectionQuickPicker.
type sectionPickerEntry struct {
	displayName string // shown in the list
	section     string // value passed to SetSection ("" for Active)
}

// GlobalMoveSectionSelectedMsg is emitted when the user picks a destination section.
type GlobalMoveSectionSelectedMsg struct {
	Section string // "" for Active, otherwise section name
}

// GlobalSectionChangedMsg is emitted when the user picks a new section in the
// global section change workflow.
type GlobalSectionChangedMsg struct {
	Task    *models.Task
	Project *models.Project
	Section string // "" for Active, otherwise section name
}

var sectionPickerBreadcrumb = []string{"Source Project", "Select Task", "Destination", "Section"}

// SectionQuickPicker is an overlay for choosing the destination section
// when moving a task to another project or changing a task's section.
// Esc emits backMsg to return to the previous step.
type SectionQuickPicker struct {
	entries       []sectionPickerEntry
	cursor        int
	task          *models.Task
	project       *models.Project // destination or owning project
	backMsg       tea.Msg
	breadcrumb    []string
	breadcrumbIdx int
	resultFactory func(string) tea.Msg // called with selected section on enter
}

// newSectionQuickPicker creates a picker listing Active plus all sections
// from the destination project. Pre-selects Next if available, else Active.
// Used by the global task-move workflow.
func newSectionQuickPicker(destProject *models.Project, sourceTask *models.Task, backMsg tea.Msg) *SectionQuickPicker {
	entries := []sectionPickerEntry{
		{displayName: "Active", section: ""},
	}
	for _, s := range destProject.GetSections() {
		entries = append(entries, sectionPickerEntry{displayName: s, section: s})
	}

	// Pre-select Next if available, else Active.
	cursor := 0
	for i, e := range entries {
		if e.section == models.SectionNext {
			cursor = i
			break
		}
	}

	return &SectionQuickPicker{
		entries:       entries,
		cursor:        cursor,
		task:          sourceTask,
		project:       destProject,
		backMsg:       backMsg,
		breadcrumb:    sectionPickerBreadcrumb,
		breadcrumbIdx: 3,
		resultFactory: func(section string) tea.Msg {
			return GlobalMoveSectionSelectedMsg{Section: section}
		},
	}
}

// newSectionChangeQuickPicker creates a picker for changing a task's section
// within its own project. Pre-selects the task's current section.
// Used by the global section-change workflow.
func newSectionChangeQuickPicker(project *models.Project, task *models.Task, backMsg tea.Msg) *SectionQuickPicker {
	entries := []sectionPickerEntry{
		{displayName: "Active", section: ""},
	}
	for _, s := range project.GetSections() {
		entries = append(entries, sectionPickerEntry{displayName: s, section: s})
	}

	currentSection := taskSectionBucket(task)
	cursor := 0
	for i, e := range entries {
		if e.section == currentSection {
			cursor = i
			break
		}
	}

	bc := []string{"Select Project", "Select Task", "Choose Section"}
	return &SectionQuickPicker{
		entries:       entries,
		cursor:        cursor,
		task:          task,
		project:       project,
		backMsg:       backMsg,
		breadcrumb:    bc,
		breadcrumbIdx: 2,
		resultFactory: func(section string) tea.Msg {
			return GlobalSectionChangedMsg{Task: task, Project: project, Section: section}
		},
	}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg).
func (p *SectionQuickPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, p.backMsg

	case "enter":
		return nil, true, p.resultFactory(p.entries[p.cursor].section)

	case "up", "ctrl+k", "ctrl+p", "k":
		p.moveCursor(-1)
		return nil, false, nil

	case "down", "ctrl+j", "ctrl+n", "j":
		p.moveCursor(1)
		return nil, false, nil

	default:
		return nil, false, nil
	}
}

func (p *SectionQuickPicker) moveCursor(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.entries) {
		p.cursor = len(p.entries) - 1
	}
}

func (p *SectionQuickPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	taskTitle := p.task.Title
	const maxTitleLen = 40
	if len(taskTitle) > maxTitleLen {
		taskTitle = taskTitle[:maxTitleLen-1] + "…"
	}
	destName := p.project.Name
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(
		fmt.Sprintf("Move to %s: Select Section", destName),
	)
	footerText := HelpStyle.Render(fmt.Sprintf("Moving: %s · Enter: select · j/k: navigate · Esc: back", taskTitle))

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*3/5, 40), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var content strings.Builder
	if bc := renderStepBreadcrumb(p.breadcrumb, p.breadcrumbIdx); bc != "" {
		content.WriteString(bc)
		content.WriteString("\n")
	}

	for i, e := range p.entries {
		line := fmt.Sprintf("  %s", e.displayName)

		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}

		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}

		content.WriteString(line)
		if i < len(p.entries)-1 {
			content.WriteString("\n")
		}
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
