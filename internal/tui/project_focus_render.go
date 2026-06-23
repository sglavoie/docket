package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"

	tea "charm.land/bubbletea/v2"
)

// GetHelpItems returns help items for this view.
func (m *ProjectFocusViewModel) GetHelpItems() []HelpItem {
	return mergeHelpItems(
		GetKeybindingRegistry().GetAppLevelForView(ViewProjectFocusView),
		helpItemsFromKeyMap(m.buildKeyMap()),
	)
}

func (m *ProjectFocusViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *ProjectFocusViewModel) View() tea.View {
	tasks := m.filteredTasks()

	var b strings.Builder

	subtitle := fmt.Sprintf("%d active tasks", len(m.tasks))
	if len(m.tasks) == 0 {
		subtitle = "no active tasks"
	}
	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Focus: %s", m.project.Name), subtitle, m.AccentColor()))
	b.WriteString("\n\n")

	if len(m.tasks) == 0 {
		b.WriteString(renderEmptyState("No tasks in focus.", "Mark tasks as active with * in the project tree."))
		b.WriteString("\n")
		left := HelpStyle.Render("q return")
		b.WriteString(renderStatusBarFooter(left, "", m.Width(), m.AccentColor()))
		return tea.NewView(b.String())
	}

	contentHeight := max(m.Height()-OverlayListChrome, 1)

	// Sub-tab bar
	subTabBar := m.renderSubTabBar()
	if subTabBar != "" {
		b.WriteString(subTabBar)
		b.WriteString("\n")
		contentHeight -= 2
	}

	// Reserve one row for the selected-task metadata strip whenever a task is
	// selectable. Keeps layout stable as the selection moves.
	var stripTask *models.Task
	if m.cursor >= 0 && m.cursor < len(tasks) {
		stripTask = tasks[m.cursor]
		contentHeight = max(contentHeight-1, 1)
	}

	// Compute column layout for badge alignment.
	var colLayout *ColumnLayout
	if m.Width() >= MinColumnAlignWidth {
		colLayout = ComputeTaskLineColumnLayout(tasks, false)
	}

	// Render task lines
	var lines []string
	maxTitleLen := m.Width() - 35

	for i, task := range tasks {
		isSelected := i == m.cursor
		line := m.renderTaskLine(task, isSelected, maxTitleLen, colLayout, i)
		lines = append(lines, line)
	}

	if len(tasks) == 0 {
		b.WriteString(MutedStyle.Render(EmptyStateFiltered))
		b.WriteString("\n\n")
	} else {
		visibleLines, _, _ := persistentLineWindow(lines, m.cursor, contentHeight, &m.viewScrollOffset)
		b.WriteString(strings.Join(visibleLines, "\n"))
		if stripTask != nil {
			strip := renderTaskMetadataStrip(TaskMetadataStripOpts{
				Task:    stripTask,
				Project: m.project,
				Config:  m.getConfig(),
				Width:   m.Width(),
				AlreadyShown: TaskMetadataAlreadyShown{
					Project:  true,
					Priority: true,
					Due:      stripTask.DueDate != nil,
				},
			})
			b.WriteString("\n")
			b.WriteString(strip)
		}
		b.WriteString("\n\n")
	}

	b.WriteString(renderStatusMessages(m.statusMessage, m.Error()))

	// Footer
	footerParts := "j/k nav · * unfocus · ,s/d/b/x status · 1-5 priority · dt/dm due"
	if len(m.subTabNames) > 1 {
		footerParts += " · Tab sections"
	}
	footerParts += " · ? help"
	left := HelpStyle.Render(footerParts)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(tasks)))
	b.WriteString(renderStatusBarFooter(left, right, m.Width(), m.AccentColor()))

	return tea.NewView(b.String())
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *ProjectFocusViewModel) renderSubTabBar() string {
	if len(m.subTabNames) <= 1 {
		return ""
	}

	tabs := make([]string, 0, len(m.subTabNames)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, m.subTabNames...)

	bar := RenderTabBar(tabs, m.subTab, min(m.Width()-6, 54)-2, 1, m.AccentColor())

	if m.subTabRowFocused {
		bar = FocusIndicator + bar
	} else {
		bar = " " + bar
	}
	return bar
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *ProjectFocusViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
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

// renderTaskLine renders a single task line.
func (m *ProjectFocusViewModel) renderTaskLine(task *models.Task, isSelected bool, maxTitleLen int, layout *ColumnLayout, rowIndex int) string {
	csStr := ""

	dueStr := ""
	if task.DueDate != nil {
		dueStr = FormatRelativeDueTask(task, true, m.getConfig().GetTimeFormat())
	}

	// Build indicator groups
	orgGroup := csStr
	structureGroup := renderProgressFraction(task)
	contentGroup := renderTruncatedTags(task.Tags, 2, 8)
	postTitle := JoinIndicatorGroups(orgGroup, structureGroup, contentGroup)
	if postTitle != "" {
		postTitle = " " + postTitle
	}

	return TaskLine{
		Task:       task,
		Project:    m.project,
		IsSelected: isSelected,
		Active:     task.Active,
		Pinned:     task.Pinned,
		MaxTitle:   maxTitleLen,
		PostTitle:  postTitle,
		DueStr:     dueStr,
		ViewWidth:  m.Width(),
		Flash:      m.cursorFlash,
		Layout:     layout,
		RowIndex:   rowIndex,
	}.Render()
}

func (m *ProjectFocusViewModel) StatusBarLabel() string { return "Active Tasks" }
