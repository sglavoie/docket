package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *LinkedTaskSelectorModel) View() tea.View { //nolint:gocyclo
	var b strings.Builder
	if len(m.relations) == 0 {
		b.WriteString(m.renderLinkedTaskSelectorTitle("Linked Tasks"))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render("No linked tasks."))
		b.WriteString("\n")
		left := HelpStyle.Render("Esc close")
		b.WriteString(renderStatusBarFooter(left, "", m.width, m.AccentColor()))
		return tea.NewView(b.String())
	}

	maxVisibleItems := max(m.height-10, 8)
	if m.cursor >= m.scrollOffset+maxVisibleItems {
		m.scrollOffset = m.cursor - maxVisibleItems + 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}

	visibleStart := m.scrollOffset
	visibleEnd := min(m.scrollOffset+maxVisibleItems, len(m.filtered))

	b.WriteString(m.renderLinkedTaskSelectorTitle("Navigate Linked Tasks"))
	b.WriteString("\n\n")
	b.WriteString(m.renderSearchState())

	if len(m.filtered) == 0 && len(m.relations) > 0 {
		b.WriteString(MutedStyle.Render("No matching linked tasks."))
		b.WriteString("\n")
		left := HelpStyle.Render("Esc close")
		b.WriteString(renderStatusBarFooter(left, "", m.width, m.AccentColor()))
		return tea.NewView(b.String())
	}

	scrollbar := RenderScrollbar(len(m.filtered), visibleEnd-visibleStart, visibleStart)
	contentWidth := m.width
	if scrollbar != "" {
		contentWidth = max(contentWidth-1, 1)
	}
	sectionContent := m.renderVisibleSections(visibleStart, visibleEnd, contentWidth)
	b.WriteString(attachScrollbarAtWidth(sectionContent, scrollbar, contentWidth))

	footerLeft := HelpStyle.Render("/ search · j/k navigate · Enter select · x unlink · Esc cancel")
	if m.IsSearchMode() {
		footerLeft = HelpStyle.Render("Type to filter · Enter confirm · Esc cancel")
	}
	right := MutedStyle.Render(fmt.Sprintf("%d links", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(footerLeft, right, m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

func (m *LinkedTaskSelectorModel) renderLinkedTaskSelectorTitle(title string) string {
	subtitlePrefix := fmt.Sprintf("#%d ", m.sourceTask.ShortID)
	availableTitleWidth := max(m.width-lipgloss.Width(title)-lipgloss.Width(subtitlePrefix)-4, 1)
	taskTitle := truncateToVisualWidth(m.sourceTask.Title, min(availableTitleWidth, 30))
	titleContent := fullscreenTitleStyle(m.AccentColor()).Render(title) + "  " +
		MutedStyle.Render(subtitlePrefix) +
		TaskTitleStyleFor(m.sourceTask, TaskTitleStyleOptions{}).Render(taskTitle)
	return renderFullscreenTitleContent(titleContent, "")
}

func (m *LinkedTaskSelectorModel) renderSearchState() string {
	if m.IsSearchMode() {
		return "Search: " + m.SearchInputView() + "\n\n"
	}
	if m.SearchQuery() != "" {
		return MutedStyle.Render(fmt.Sprintf("Filter: %s (%d matches)", m.SearchQuery(), len(m.filtered))) + "\n\n"
	}
	return ""
}

func (m *LinkedTaskSelectorModel) renderVisibleSections(visibleStart, visibleEnd, contentWidth int) string {
	var b strings.Builder
	itemIndex := 0
	needSpacer := false

	for _, section := range m.groupedRelationSections() {
		if len(section.items) == 0 {
			continue
		}

		sectionStart := itemIndex
		sectionEnd := itemIndex + len(section.items)
		if sectionEnd > visibleStart && sectionStart < visibleEnd {
			if needSpacer {
				b.WriteString("\n")
			}
			b.WriteString(SubtitleStyle.Render(section.title))
			b.WriteString("\n")
			needSpacer = true

			for itemOffset, rel := range section.items {
				idx := sectionStart + itemOffset
				if idx >= visibleStart && idx < visibleEnd {
					b.WriteString("  " + m.renderRelationLineAtWidth(rel, false, contentWidth) + "\n")
				}
			}
		}

		itemIndex = sectionEnd
	}

	return b.String()
}

type linkedTaskRelationSection struct {
	title string
	items []LinkedTaskRelation
}

func (m *LinkedTaskSelectorModel) groupedRelationSections() []linkedTaskRelationSection {
	var blocksItems []LinkedTaskRelation
	var blockedByItems []LinkedTaskRelation
	var relatedItems []LinkedTaskRelation

	for _, rel := range m.filtered {
		switch rel.RelationType {
		case "blocks":
			blocksItems = append(blocksItems, rel)
		case "blocked by":
			blockedByItems = append(blockedByItems, rel)
		case "related":
			relatedItems = append(relatedItems, rel)
		}
	}

	return []linkedTaskRelationSection{
		{title: "→ Blocks", items: blocksItems},
		{title: "← Blocked By", items: blockedByItems},
		{title: "↔ Related", items: relatedItems},
	}
}

func (m *LinkedTaskSelectorModel) renderRelationLine(rel LinkedTaskRelation, isSelected bool) string {
	return m.renderRelationLineAtWidth(rel, isSelected, m.width)
}

func (m *LinkedTaskSelectorModel) renderRelationLineAtWidth(rel LinkedTaskRelation, isSelected bool, width int) string {
	if rel.IsMissing {
		ref := models.ParseTaskRef(rel.TaskRef)
		label := "(missing)"
		if ref.IsCrossProject() {
			label = fmt.Sprintf("[%s] (missing)", ref.ProjectID)
		}

		line := ValidationErrorStyle.Render(label)
		if isSelected {
			line = SelectedStyle.Render(line)
		}
		return line
	}

	statusIcon := GetStatusBadge(string(rel.Task.Status), rel.Task.Status.Icon())
	typeIcon := GetTypeBadge(string(rel.Task.Type), rel.Task.Type.Icon())

	availableWidth := max(width-4, 20)
	projectLabel := ""
	if rel.Project != nil {
		projectName := truncateToVisualWidth(rel.Project.Name, 12)
		projectLabel = MutedStyle.Render(fmt.Sprintf("[%s] ", projectName))
	}

	prefix := fmt.Sprintf("%s %s %s#%d ", statusIcon, typeIcon, projectLabel, rel.Task.ShortID)
	maxTitleLen := max(availableWidth-lipgloss.Width(prefix), 1)
	title := truncateToVisualWidth(rel.Task.Title, maxTitleLen)
	titleRendered := TaskTitleStyleFor(rel.Task, TaskTitleStyleOptions{
		CursorSelected: isSelected,
	}).Render(title)
	line := prefix + titleRendered
	if lipgloss.Width(line) > availableWidth {
		excess := lipgloss.Width(line) - availableWidth
		title = truncateToVisualWidth(rel.Task.Title, max(maxTitleLen-excess, 1))
		titleRendered = TaskTitleStyleFor(rel.Task, TaskTitleStyleOptions{
			CursorSelected: isSelected,
		}).Render(title)
		line = prefix + titleRendered
	}
	if isSelected {
		line = SelectedStyle.Render(line)
	}
	return line
}
