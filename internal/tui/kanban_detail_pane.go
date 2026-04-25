package tui

import (
	"fmt"
	"strings"
)

// detailPaneHeight returns the configured detail pane height (default 3).
func (m *KanbanModel) detailPaneHeight() int {
	if m.config.KanbanSettings.DetailPaneHeight != nil {
		h := *m.config.KanbanSettings.DetailPaneHeight
		if h >= 1 && h <= 10 {
			return h
		}
	}
	return 3
}

// renderKanbanDetailPane renders the bottom detail pane for the focused card.
// Returns an empty string when the pane is not visible.
func (m *KanbanModel) renderKanbanDetailPane() string {
	if !m.detailPaneVisible {
		return ""
	}

	card := m.getCursorCard()
	paneH := m.detailPaneHeight()
	w := m.width

	var sb strings.Builder

	// Horizontal separator
	sb.WriteString(strings.Repeat("─", w) + "\n")

	if card == nil {
		for range paneH {
			sb.WriteString("\n")
		}
		return sb.String()
	}

	task := card.Task
	project := card.Project

	// ── Line 1: title │ priority │ due │ project │ recency ────────────────
	sep := MutedStyle.Render(" │ ")

	var parts []string
	if m.crossProject && project != nil {
		parts = append(parts, MutedStyle.Render(project.Name))
	}
	if task.Priority > 0 {
		parts = append(parts, renderPriorityBadge(task.Priority))
	}
	if task.DueDate != nil {
		dueStr := FormatRelativeDueTask(task, true, "Jan 2")
		parts = append(parts, "Due: "+dueStr)
	}
	if project != nil {
		if profile := m.config.GetProjectScoringProfile(project.ID); profile != nil {
			if scoreBadge := RenderScoreBadge(task, profile); scoreBadge != "" {
				scoreStr := scoreBadge
				if profile.Name != "" {
					scoreStr += " " + MutedStyle.Render("("+profile.Name+")")
				}
				parts = append(parts, scoreStr)
			}
		}
	}
	if !task.UpdatedAt.IsZero() {
		parts = append(parts, MutedStyle.Render(formatRelativeTime(task.UpdatedAt)))
	}

	titlePart := TitleStyle.Render(truncateToVisualWidth(task.Title, 28))
	var line1 string
	if len(parts) > 0 {
		line1 = " " + titlePart + sep + strings.Join(parts, sep)
	} else {
		line1 = " " + titlePart
	}
	sb.WriteString(truncateToVisualWidth(line1, w) + "\n")

	if paneH < 2 {
		return sb.String()
	}

	// ── Line 2: blockers │ tags ───────────────────────────────────────────
	var parts2 []string

	if len(task.DependsOn) > 0 {
		titles := m.lookupTaskTitles(task.DependsOn, 2)
		if len(titles) > 0 {
			blocker := "Blocked by: " + strings.Join(titles, ", ")
			if len(task.DependsOn) > 2 {
				blocker += fmt.Sprintf(" +%d", len(task.DependsOn)-2)
			}
			parts2 = append(parts2, blocker)
		} else {
			parts2 = append(parts2, fmt.Sprintf("Blocked by %d %s", len(task.DependsOn),
				pluralize(len(task.DependsOn), "task", "tasks")))
		}
	}
	if len(task.Tags) > 0 {
		parts2 = append(parts2, "Tags: "+renderTruncatedTags(task.Tags, 4, 12))
	}

	var line2 string
	if len(parts2) > 0 {
		line2 = " " + strings.Join(parts2, sep)
	}
	sb.WriteString(truncateToVisualWidth(line2, w) + "\n")

	// Remaining lines
	for i := 2; i < paneH; i++ {
		sb.WriteString("\n")
	}

	return sb.String()
}

// lookupTaskTitles returns the titles of tasks by ID from allCards, up to max results.
func (m *KanbanModel) lookupTaskTitles(ids []string, maxResults int) []string {
	idSet := make(map[string]string, len(ids))
	for _, id := range ids {
		idSet[id] = ""
	}
	for _, card := range m.allCards {
		if _, ok := idSet[card.Task.ID]; ok {
			idSet[card.Task.ID] = card.Task.Title
		}
	}
	var titles []string
	for _, id := range ids {
		if t, ok := idSet[id]; ok && t != "" {
			titles = append(titles, truncateToVisualWidth(t, 20))
			if len(titles) >= maxResults {
				break
			}
		}
	}
	return titles
}
