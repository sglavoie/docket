package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// View renders the global tag manager view.
func (m *GlobalTagManagerModel) View() tea.View {
	var b strings.Builder

	if m.showProjectPicker {
		return tea.NewView(m.renderProjectPicker())
	}

	b.WriteString(renderFullscreenTitle("Global Tag Manager", "across all projects"))
	b.WriteString("\n\n")

	if len(m.tags) == 0 {
		b.WriteString(MutedStyle.Render("No tags found in any project"))
		b.WriteString("\n")
	} else {
		list := m.renderTagRows()
		if list != "" {
			b.WriteString(list)
			b.WriteString("\n")
		}
	}

	if m.errorMsg != "" {
		b.WriteString("\n")
		b.WriteString(TagManagerErrorStyle.Render(m.errorMsg))
		b.WriteString("\n")
	}

	var footerHelp string
	if m.renameMode {
		footerHelp = "enter confirm · esc cancel"
	} else if len(m.tags) > 0 {
		footerHelp = "j/k nav · enter filter · space/p pin · e edit · Esc close"
	} else {
		footerHelp = "Esc close"
	}
	b.WriteString(renderStatusBarFooter(HelpStyle.Render(footerHelp), "", m.width))

	return tea.NewView(b.String())
}

func (m *GlobalTagManagerModel) renderTagRows() string {
	maxVisible := m.maxVisibleTagRows()
	start := calcScrollOffset(m.cursor, maxVisible)
	end := min(start+maxVisible, len(m.tags))

	rowWidth := m.rowWidth()
	contentWidth := rowWidth
	scrollbar := RenderScrollbar(len(m.tags), maxVisible, start)
	if scrollbar != "" {
		contentWidth = max(rowWidth-1, 1)
	}

	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rows = append(rows, m.renderTagRow(i, contentWidth))
	}
	content := strings.Join(rows, "\n")
	if scrollbar != "" {
		return attachScrollbarAtWidth(content, scrollbar, contentWidth)
	}
	return content
}

func (m *GlobalTagManagerModel) renderTagRow(i, width int) string {
	tag := m.tags[i]
	style := TagManagerItemStyle
	if i == m.cursor {
		style = TagManagerItemSelectedStyle
	}

	if m.renameMode && tag.Name == m.renameTarget {
		return fitStyledLineToWidth("  Rename to: "+m.renameInput.View(), width)
	}

	pinMarker := "  "
	if tag.Pinned {
		pinMarker = "* "
	}
	line := fmt.Sprintf("  %s%s ", pinMarker, tag.Name)
	countStr := fmt.Sprintf("(%d)", tag.TaskCount)
	projectStr := fmt.Sprintf(" [%d project", len(tag.Projects))
	if len(tag.Projects) != 1 {
		projectStr += "s"
	}
	projectStr += "]"

	return fitStyledLineToWidth(style.Render(line)+TagManagerCountStyle.Render(countStr)+MutedStyle.Render(projectStr), width)
}

func (m *GlobalTagManagerModel) maxVisibleTagRows() int {
	if m.height <= 0 {
		return max(len(m.tags), 1)
	}
	const baseChrome = 4 // title, blank line, own footer separator, own footer help.
	errorChrome := 0
	if m.errorMsg != "" {
		errorChrome = 2 // blank line plus error row.
	}
	return max(m.height-baseChrome-errorChrome, 1)
}

// renderProjectPicker renders the project picker overlay.
func (m *GlobalTagManagerModel) renderProjectPicker() string {
	var b strings.Builder

	var projects []TagProjectUsage
	for _, tag := range m.tags {
		if tag.Name == m.selectedTag {
			projects = tag.Projects
			break
		}
	}

	b.WriteString(renderFullscreenTitle("Select Project", fmt.Sprintf("for tag \"%s\"", m.selectedTag)))
	b.WriteString("\n\n")

	list := m.renderProjectRows(projects)
	if list != "" {
		b.WriteString(list)
		b.WriteString("\n")
	}

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("j/k nav · enter select · esc back"), "", m.width))

	return b.String()
}

func (m *GlobalTagManagerModel) renderProjectRows(projects []TagProjectUsage) string {
	if len(projects) == 0 {
		return ""
	}
	maxVisible := m.maxVisibleProjectRows(len(projects))
	start := calcScrollOffset(m.projectCursor, maxVisible)
	end := min(start+maxVisible, len(projects))

	rowWidth := m.rowWidth()
	contentWidth := rowWidth
	scrollbar := RenderScrollbar(len(projects), maxVisible, start)
	if scrollbar != "" {
		contentWidth = max(rowWidth-1, 1)
	}

	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rows = append(rows, m.renderProjectRow(projects[i], i == m.projectCursor, contentWidth))
	}
	content := strings.Join(rows, "\n")
	if scrollbar != "" {
		return attachScrollbarAtWidth(content, scrollbar, contentWidth)
	}
	return content
}

func (m *GlobalTagManagerModel) renderProjectRow(usage TagProjectUsage, selected bool, width int) string {
	style := TagManagerItemStyle
	if selected {
		style = TagManagerItemSelectedStyle
	}
	line := fmt.Sprintf("  %s ", usage.Project.Name)
	countStr := fmt.Sprintf("(%d tasks)", usage.TaskCount)
	return fitStyledLineToWidth(style.Render(line)+TagManagerCountStyle.Render(countStr), width)
}

func (m *GlobalTagManagerModel) maxVisibleProjectRows(projectCount int) int {
	if m.height <= 0 {
		return max(projectCount, 1)
	}
	const baseChrome = 4 // title, blank line, own footer separator, own footer help.
	return max(m.height-baseChrome, 1)
}

func (m *GlobalTagManagerModel) rowWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}
