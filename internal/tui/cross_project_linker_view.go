package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *CrossProjectLinkerModel) View() tea.View {
	title := m.getStepTitle()
	subtitlePrefix := fmt.Sprintf("#%d ", m.sourceTask.ShortID)
	availableTitleWidth := max(m.Width()-lipgloss.Width(title)-lipgloss.Width(subtitlePrefix)-4, 1)
	sourceTitle := truncateToVisualWidth(m.sourceTask.Title, availableTitleWidth)
	titleContent := fullscreenTitleStyle(m.AccentColor()).Render(title) + "  " +
		MutedStyle.Render(subtitlePrefix) +
		TaskTitleStyleFor(m.sourceTask, TaskTitleStyleOptions{}).Render(sourceTitle)

	var out strings.Builder
	out.WriteString(renderFullscreenTitleContent(titleContent, ""))
	out.WriteString("\n\n")

	switch m.step {
	case StepSelectProject:
		out.WriteString(m.viewProjectSelection())
	case StepSelectTask:
		out.WriteString(m.viewTaskSelection())
	case StepSelectLinkType:
		out.WriteString(m.viewLinkTypeSelection())
	}

	return tea.NewView(out.String())
}

func (m *CrossProjectLinkerModel) getStepTitle() string {
	switch m.step {
	case StepSelectProject:
		return "Select Project (1/3)"
	case StepSelectTask:
		return "Select Task (2/3)"
	case StepSelectLinkType:
		return "Select Link Type (3/3)"
	}
	return "Cross-Project Link"
}

func (m *CrossProjectLinkerModel) viewProjectSelection() string {
	var b strings.Builder

	if m.IsSearchMode() {
		b.WriteString("Search: " + m.SearchInputView())
		b.WriteString("\n\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d matches)", m.SearchQuery(), len(m.filteredProjects))))
		b.WriteString("\n\n")
	}

	if len(m.filteredProjects) == 0 {
		if len(m.allProjects) == 0 {
			b.WriteString(MutedStyle.Render("No other projects available."))
		} else {
			b.WriteString(MutedStyle.Render("No matching projects."))
		}
		b.WriteString("\n\n")
		if m.SearchQuery() != "" {
			b.WriteString(HelpStyle.Render("Esc close"))
		} else {
			b.WriteString(HelpStyle.Render("Esc cancel"))
		}
		return b.String()
	}

	maxVisible := 8
	scrollOffset := calcScrollOffset(m.projectCursor, maxVisible)

	var rowBuilder strings.Builder
	for i, p := range m.filteredProjects {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}
		contextBadge := MutedStyle.Render(fmt.Sprintf("[%s]", p.Context))
		availableNameWidth := max(m.Width()-lipgloss.Width(contextBadge)-3, 1)
		name := truncateToVisualWidth(p.Name, availableNameWidth)
		line := fmt.Sprintf("  %s %s", name, contextBadge)
		if i == m.projectCursor {
			line = BoldStyle.Render(line)
		}
		rowBuilder.WriteString(line)
		rowBuilder.WriteString("\n")
	}

	scrollbar := RenderScrollbar(len(m.filteredProjects), maxVisible, scrollOffset)
	b.WriteString(AttachScrollbar(rowBuilder.String(), scrollbar))

	b.WriteString("\n")
	if m.IsSearchMode() {
		b.WriteString(HelpStyle.Render(renderWrappingHelpLine("Type to filter · Enter confirm · Esc cancel", m.Width())))
	} else {
		b.WriteString(HelpStyle.Render(renderWrappingHelpLine("/ search · j/k navigate · Enter select · Esc cancel", m.Width())))
	}

	return b.String()
}

func (m *CrossProjectLinkerModel) viewTaskSelection() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Project: %s", m.targetProject.Name)))
	b.WriteString("\n\n")

	if m.IsSearchMode() {
		b.WriteString("Search: " + m.SearchInputView())
		b.WriteString("\n\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d matches)", m.SearchQuery(), len(m.filteredTasks))))
		b.WriteString("\n\n")
	}

	if len(m.filteredTasks) == 0 {
		if len(m.allTasks) == 0 {
			b.WriteString(MutedStyle.Render("No tasks in this project. Add tasks first, then link them here."))
		} else {
			b.WriteString(MutedStyle.Render("No matching tasks. Try a different search term."))
		}
		b.WriteString("\n\n")
		if m.SearchQuery() != "" {
			b.WriteString(HelpStyle.Render("Esc close"))
		} else {
			b.WriteString(HelpStyle.Render("Esc go back"))
		}
		return b.String()
	}

	maxVisible := 8
	scrollOffset := calcScrollOffset(m.taskCursor, maxVisible)

	var rowBuilder strings.Builder
	for i, t := range m.filteredTasks {
		if i < scrollOffset || i >= scrollOffset+maxVisible {
			continue
		}
		statusIcon := GetStatusBadge(string(t.Status), t.Status.Icon())
		typeIcon := GetTypeBadge(string(t.Type), t.Type.Icon())
		stageBadge := GetStageBadge(t.Stage)
		stageSegment := ""
		if stageBadge != "" {
			stageSegment = stageBadge + " "
		}
		prefix := fmt.Sprintf("  %s %s %s#%d ", statusIcon, typeIcon, stageSegment, t.ShortID)
		maxTitleLen := max(m.Width()-lipgloss.Width(prefix), 1)
		title := truncateToVisualWidth(t.Title, maxTitleLen)
		titleRendered := TaskTitleStyleFor(t, TaskTitleStyleOptions{
			CursorSelected: i == m.taskCursor,
		}).Render(title)
		line := prefix + titleRendered
		if lipgloss.Width(line) > m.Width() {
			excess := lipgloss.Width(line) - m.Width()
			title = truncateToVisualWidth(t.Title, max(maxTitleLen-excess, 1))
			titleRendered = TaskTitleStyleFor(t, TaskTitleStyleOptions{
				CursorSelected: i == m.taskCursor,
			}).Render(title)
			line = prefix + titleRendered
		}
		if i == m.taskCursor {
			line = BoldStyle.Render(line)
		}
		rowBuilder.WriteString(line)
		rowBuilder.WriteString("\n")
	}

	scrollbar := RenderScrollbar(len(m.filteredTasks), maxVisible, scrollOffset)
	b.WriteString(AttachScrollbar(rowBuilder.String(), scrollbar))

	b.WriteString("\n")
	if m.IsSearchMode() {
		b.WriteString(HelpStyle.Render(renderWrappingHelpLine("Type to filter · Enter confirm · Esc cancel", m.Width())))
	} else {
		b.WriteString(HelpStyle.Render(renderWrappingHelpLine("/ search · j/k navigate · Enter select · Esc back", m.Width())))
	}

	return b.String()
}

func (m *CrossProjectLinkerModel) viewLinkTypeSelection() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Project: %s", m.targetProject.Name)))
	b.WriteString("\n")
	targetPrefix := fmt.Sprintf("Task: #%d ", m.targetTask.ShortID)
	targetTitle := truncateToVisualWidth(m.targetTask.Title, max(m.Width()-lipgloss.Width(targetPrefix), 1))
	b.WriteString(MutedStyle.Render(targetPrefix))
	b.WriteString(TaskTitleStyleFor(m.targetTask, TaskTitleStyleOptions{}).Render(targetTitle))
	b.WriteString("\n\n")

	linkTypes := []struct {
		name string
		desc string
	}{
		{name: "Blocks", desc: fmt.Sprintf("#%d blocks #%d (source depends on target)", m.targetTask.ShortID, m.sourceTask.ShortID)},
		{name: "Blocked By", desc: fmt.Sprintf("#%d depends on #%d (target depends on source)", m.targetTask.ShortID, m.sourceTask.ShortID)},
		{name: "Related", desc: fmt.Sprintf("#%d ↔ #%d (bidirectional)", m.sourceTask.ShortID, m.targetTask.ShortID)},
	}

	for i, lt := range linkTypes {
		line := "  " + lt.name
		if i == m.linkTypeCursor {
			line = BoldStyle.Render(line)
			b.WriteString(line)
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render("    " + lt.desc))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine("↑/↓ navigate · Enter create link · Esc back", m.Width())))

	return b.String()
}
