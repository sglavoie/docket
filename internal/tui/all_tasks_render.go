package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AllTasksViewModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("All Tasks", m.allTasksSubtitle(), m.AccentColor()))
	b.WriteString("\n")

	if len(m.allTasks) == 0 {
		b.WriteString("\n")
		b.WriteString(renderEmptyState("No tasks found across projects.", "Press Esc to return to dashboard."))
		return tea.NewView(b.String())
	}

	chrome, chromeLines := m.renderAllTasksChrome()
	b.WriteString(chrome)

	// Overflow picker overlay: render instead of task list when open.
	if m.overflowPickerOpen {
		b.WriteString(m.renderOverflowPicker())
		return tea.NewView(b.String())
	}

	contentHeight := fullscreenContentHeight(m.Height(), chromeLines)
	if m.totalNavigableItems() == 0 {
		b.WriteString(m.renderAllTasksEmptyState())
		return tea.NewView(b.String())
	}

	lineWidth := m.allTasksLineWidth(contentHeight)
	colLayout := m.allTasksColumnLayout(lineWidth)
	lines, cursorLineIdx := m.buildAllTasksLines(lineWidth, colLayout)
	lines, contentHeight = m.addAllTasksPreview(lines, cursorLineIdx, contentHeight, lineWidth)
	b.WriteString(m.renderAllTasksScrollableContent(lines, cursorLineIdx, contentHeight))
	b.WriteString(m.renderAllTasksFooter())

	return tea.NewView(b.String())
}

func (m *AllTasksViewModel) allTasksSubtitle() string {
	if m.totalVisibleTaskCount() != len(m.allTasks) {
		return fmt.Sprintf("%d/%d tasks across %d projects", m.totalVisibleTaskCount(), len(m.allTasks), m.projectCount())
	}
	return fmt.Sprintf("%d tasks across %d projects", len(m.allTasks), m.projectCount())
}

func (m *AllTasksViewModel) renderAllTasksChrome() (string, int) {
	tabAccent := m.AccentColor()
	if entry := m.activeTabEntry(); entry != nil && !entry.IsBuiltIn && !entry.IsOverflow {
		tabAccent = ColorAccent
	}

	chrome, chromeLines := renderFullscreenChrome(
		m.tabs.RenderBar(m.Width(), tabAccent),
		m.renderActiveFilterIndicator(),
		false,
	)

	var b strings.Builder
	b.WriteString(chrome)
	if m.groupBy != ATGroupByProject {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  Grouped by: %s", m.groupBy)))
		b.WriteString("\n")
		chromeLines++
	}
	b.WriteString("\n")
	chromeLines++

	return b.String(), chromeLines
}

func (m *AllTasksViewModel) renderAllTasksEmptyState() string {
	if m.HasActiveFilters() {
		return renderEmptyState(EmptyStateFiltered, "shift+←/→ tabs · Fx clear filters · Esc close")
	}
	if m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix() {
		return renderContextualEmptyState(EmptyStateNoTasks,
			[]string{"i add", "F filters", ": commands", "? help"})
	}
	return renderEmptyState(EmptyStateNoTasks, "shift+←/→ tabs · Esc close")
}

func (m *AllTasksViewModel) allTasksLineWidth(contentHeight int) int {
	lineWidth := m.Width()
	if m.totalNavigableItems() > contentHeight {
		lineWidth--
	}
	return lineWidth
}

func (m *AllTasksViewModel) allTasksColumnLayout(lineWidth int) *ColumnLayout {
	if lineWidth < MinColumnAlignWidth {
		return nil
	}

	var visibleTasks []*models.Task
	var visibleItems []CrossProjectTask
	for _, group := range m.groups {
		if m.collapsedGroups[group.Name] {
			continue
		}
		for _, item := range group.Tasks {
			visibleTasks = append(visibleTasks, item.Task)
			visibleItems = append(visibleItems, item)
		}
	}

	return ComputeBadgeWidths(visibleTasks, BadgeWidthOpts{
		IncludeStatus:  true,
		IncludeType:    true,
		IncludeStage:   true,
		ProjectNameFor: crossProjectNameLookup(visibleItems),
		MaxProjectLen:  15,
	})
}

func (m *AllTasksViewModel) buildAllTasksLines(lineWidth int, colLayout *ColumnLayout) ([]string, int) {
	var lines []string
	maxTitleLen := lineWidth - 50
	currentIndex := 0
	cursorLineIdx := 0
	separator := renderHorizontalDivider(lineWidth)

	for i, group := range m.groups {
		if i > 0 {
			lines = append(lines, separator)
		}

		collapsed := m.collapsedGroups[group.Name]
		headerSelected := currentIndex == m.cursor
		if headerSelected {
			cursorLineIdx = len(lines)
		}
		lines = append(lines, m.renderGroupHeader(group.Name, len(group.Tasks), collapsed, headerSelected, lineWidth))
		currentIndex++

		if collapsed {
			continue
		}
		for _, item := range group.Tasks {
			isSelected := currentIndex == m.cursor
			if isSelected {
				cursorLineIdx = len(lines)
			}
			isChecked := m.selectedTasks[item.Task.ID]
			line := m.renderTaskLine(item, isSelected, isChecked, maxTitleLen, currentIndex, lineWidth, colLayout)
			lines = append(lines, line)
			currentIndex++
		}
	}

	return lines, cursorLineIdx
}

func (m *AllTasksViewModel) addAllTasksPreview(lines []string, cursorLineIdx, contentHeight, lineWidth int) ([]string, int) {
	if !m.previewMode || cursorLineIdx < 0 || cursorLineIdx >= len(lines) {
		return lines, contentHeight
	}

	task, _ := m.getTaskRef()
	if task == nil {
		return lines, contentHeight
	}

	pLines := inlinePreviewLines(task, "        ", lineWidth)
	if len(pLines) == 0 {
		return lines, contentHeight
	}

	insertAt := cursorLineIdx + 1
	if insertAt > len(lines) {
		return lines, contentHeight
	}

	tail := append([]string{}, lines[insertAt:]...)
	lines = append(lines[:insertAt], pLines...)
	lines = append(lines, tail...)
	contentHeight -= len(pLines)
	if contentHeight < 1 {
		contentHeight = 1
	}
	return lines, contentHeight
}

func (m *AllTasksViewModel) renderAllTasksScrollableContent(lines []string, cursorLineIdx, contentHeight int) string {
	scrolledLines := applyPersistentScroll(lines, cursorLineIdx, contentHeight, &m.scrollOffset)
	scrollbar := RenderScrollbar(len(lines), contentHeight, m.scrollOffset)
	tintedContent := TabContentBgStyle.Width(m.Width()).Render(strings.Join(scrolledLines, "\n"))
	return AttachScrollbar(tintedContent, scrollbar)
}

func (m *AllTasksViewModel) renderAllTasksFooter() string {
	footerLeft := "j/k nav · shift+←/→ tabs · Ctrl+G group · ,s/d/b/x status · 1-5 sort · v multi"
	if m.multiSelectMode {
		footerLeft = "MULTI-SELECT: space toggle · a select all · A deselect · ,s/d/b status · D delete · v exit"
	}
	left := HelpStyle.Render(footerLeft)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalVisibleTaskCount()))
	return renderFullscreenFooterBlock(m.statusMessage, m.err, left, right, m.Width(), m.AccentColor())
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *AllTasksViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix() {
		return nil
	}
	prefix := m.Dispatcher.PendingPrefix()
	var hints []PrefixHint
	for _, h := range m.Dispatcher.PrefixHints() {
		hints = append(hints, PrefixHint{Key: h.Key, Description: h.Description})
	}
	return &PrefixPopupInfo{
		Prefix: prefix,
		Hints:  PrefixHints{prefix: hints},
	}
}

// AllTasksNavigationPrefixHints contains "g" prefix hints for all tasks view.
var AllTasksNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project", Detail: "Navigate to task in project tree"},
}

// renderGroupHeader renders a group header with task count and collapse indicator.
func (m *AllTasksViewModel) renderGroupHeader(name string, count int, collapsed bool, isSelected bool, width int) string {
	return renderCollapsibleHeader(name, count, collapsed, isSelected, m.cursorFlash, SectionHeaderStyle, width)
}

// renderTaskLine renders a single task line.
func (m *AllTasksViewModel) renderTaskLine(item CrossProjectTask, isSelected bool, isChecked bool, maxTitleLen int, rowIndex int, lineWidth int, layout *ColumnLayout) string {
	task := item.Task

	// Multi-select checkbox
	checkbox := ""
	if m.multiSelectMode {
		if isChecked {
			checkbox = "[x] "
		} else {
			checkbox = "[ ] "
		}
	}

	dueStr := ""
	if task.DueDate != nil {
		dueStr = FormatRelativeDueTask(task, true, m.config.GetTimeFormat())
	}

	line := RenderCrossProjectTaskLine(task, item.Project, m.config, isSelected, lineWidth, maxTitleLen, CrossProjectTaskLineOpts{
		Queued:          m.queueIDs[task.ID],
		DailyPlanned:    m.dailyPlanIDs[task.ID],
		ShowTags:        true,
		ShowScore:       true,
		ShowDescription: m.config.IsDescriptionColumnVisible(),
		MaxProjectLen:   15,
		Prefix:          checkbox,
		DueStr:          dueStr,
		Flash:           m.cursorFlash,
		RowIndex:        rowIndex,
		Layout:          layout,
	})
	if isSelected && m.mutationFlash {
		return MutationFlashLine(line, lineWidth, m.mutationFlashBg)
	}
	return line
}

// renderOverflowPicker renders the overflow tab picker when there are more tabs
// than the visible cap. Shown in place of the task list.
func (m *AllTasksViewModel) renderOverflowPicker() string {
	overflowEntries := m.allTabEntries[overflowTabCap-1:]
	var b strings.Builder

	b.WriteString(MutedStyle.Render("  Select tab  ·  ↑/↓ navigate  ·  Enter select  ·  Esc cancel"))
	b.WriteString("\n\n")

	for i, e := range overflowEntries {
		name := e.Name
		if !e.IsBuiltIn {
			name = "● " + name
		}
		if i == m.overflowPickerCursor {
			b.WriteString(SelectedStyle.Render("  " + name))
		} else {
			b.WriteString("  " + name)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *AllTasksViewModel) renderActiveFilterIndicator() string {
	common := renderCommonFilterIndicator(&m.filter)
	if len(m.projectFilter) > 0 {
		projBadge := fmt.Sprintf(" [%d projects]", len(m.projectFilter))
		if common == "" {
			return FilterBadgeStyle.Render("Filters:" + projBadge)
		}
		return common + FilterBadgeStyle.Render(projBadge)
	}
	return common
}
