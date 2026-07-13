package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *RecentActivityModel) View() tea.View {
	var b strings.Builder

	title := "Latest Activity"
	if m.projectScope != nil {
		title = fmt.Sprintf("Project Activity: %s", m.projectScope.Name)
	}
	subtitle := fmt.Sprintf("%d tasks", len(m.filtered))
	if m.SearchQuery() != "" || m.IsSearchMode() {
		subtitle = fmt.Sprintf("%d/%d tasks", len(m.filtered), len(m.tasks))
	}
	b.WriteString(renderFullscreenTitle(title, subtitle, m.AccentColor()))
	b.WriteString("\n")

	if len(m.filtered) == 0 && !m.IsSearchMode() && m.SearchQuery() == "" {
		b.WriteString("\n")
		if m.HasActiveFilters() {
			b.WriteString(m.renderActiveFilterIndicator())
			b.WriteString("\n\n")
		}
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			"No recent activity found.", "Press Esc to close.",
			"Press Esc to clear filters."))
		return tea.NewView(b.String())
	}

	// Tab bar
	b.WriteString(m.tabs.RenderBar(m.Width(), m.AccentColor()))
	b.WriteString("\n")

	// Filter badge
	filterBadge := m.renderActiveFilterIndicator()
	if filterBadge != "" {
		b.WriteString(filterBadge)
		b.WriteString("\n")
	}

	// Search input
	if m.IsSearchMode() {
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Filter: %s (%d/%d)", m.SearchQuery(), len(m.filtered), len(m.tasks))))
		b.WriteString("\n")
	}

	contentHeight := fullscreenContentHeight(m.Height(), 2) // tab bar(2)
	if filterBadge != "" {
		contentHeight--
	}
	if m.IsSearchMode() || m.SearchQuery() != "" {
		contentHeight--
	}

	// Reserve one row for the selected-task metadata strip when there is a
	// task to describe.
	var stripTask *CrossProjectTask
	if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
		stripTask = &m.filtered[m.cursor]
		contentHeight = max(contentHeight-1, 1)
	}

	// Compute column layout for badge alignment.
	var colLayout *ColumnLayout
	if m.Width() >= MinColumnAlignWidth {
		tasks := make([]*models.Task, 0, len(m.filtered))
		for _, dt := range m.filtered {
			tasks = append(tasks, dt.Task)
		}
		colLayout = ComputeTaskLineColumnLayout(tasks, false)
	}

	// Build lines
	var lines []string
	for i, dt := range m.filtered {
		selected := i == m.cursor
		line := m.renderTaskLine(dt, selected, colLayout, i)
		lines = append(lines, line)
	}

	if len(m.filtered) == 0 {
		lines = append(lines, MutedStyle.Render(EmptyStateFiltered))
	}

	visibleLines, _, _ := persistentLineWindow(lines, m.cursor, contentHeight, &m.viewScrollOffset)
	b.WriteString(strings.Join(visibleLines, "\n"))
	if stripTask != nil {
		strip := renderTaskMetadataStrip(TaskMetadataStripOpts{
			Task:    stripTask.Task,
			Project: stripTask.Project,
			Config:  m.config,
			Width:   m.Width(),
			AlreadyShown: TaskMetadataAlreadyShown{
				Project:  m.projectScope != nil,
				Priority: true,
				Updated:  true,
			},
		})
		b.WriteString("\n" + strip)
	}
	b.WriteString("\n\n")

	left := HelpStyle.Render("j/k nav · Tab tabs · Enter open · p preview · , status · / filter · r refresh · ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(left, right, m.Width(), m.AccentColor()))

	return tea.NewView(b.String())
}

func (m *RecentActivityModel) renderTaskLine(dt CrossProjectTask, selected bool, layout *ColumnLayout, rowIndex int) string {
	task := dt.Task
	var preBadges string
	maxTitleLen := m.Width() - 50
	if m.projectScope == nil {
		projectName := truncateToVisualWidth(dt.Project.Name, 12)
		preBadges = MutedStyle.Render(fmt.Sprintf("[%s]", projectName)) + " " +
			MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)) + " "
	} else {
		preBadges = MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)) + " "
		maxTitleLen += 14
	}
	relTime := MutedStyle.Render("  " + storage.FormatRelativeTime(task.UpdatedAt))
	return TaskLine{
		Task:         task,
		Project:      dt.Project,
		IsSelected:   selected,
		Queued:       m.queueIDs[task.ID],
		DailyPlanned: m.dailyPlanIDs[task.ID],
		Active:       m.queueIDs[task.ID],
		Pinned:       task.Pinned,
		MaxTitle:     maxTitleLen,
		PreBadges:    preBadges,
		PostTitle:    relTime,
		ViewWidth:    m.Width(),
		Flash:        m.cursorFlash,
		SearchQuery:  m.SearchQuery(),
		Layout:       layout,
		RowIndex:     rowIndex,
	}.Render()
}
