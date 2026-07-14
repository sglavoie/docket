package tui

import (
	"fmt"
	"strings"
)

// detailPaneHeight returns the configured detail pane height (default 3).
func (m *KanbanModel) detailPaneHeight() int {
	if m.runtime.Settings().KanbanSettings.DetailPaneHeight != nil {
		h := *m.runtime.Settings().KanbanSettings.DetailPaneHeight
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

	sep := MutedStyle.Render(" │ ")

	// ── Line 1: title ─────────────────────────────────────────────────────
	titlePart := TaskTitleStyleFor(task, TaskTitleStyleOptions{}).
		Render(truncateToVisualWidth(task.Title, max(w-1, 0)))
	sb.WriteString(truncateToVisualWidth(" "+titlePart, w) + "\n")

	if paneH < 2 {
		return sb.String()
	}

	// ── Line 2: project │ priority │ due │ score │ recency ────────────────
	var metadataParts []string
	if project != nil {
		metadataParts = append(metadataParts, MutedStyle.Render(project.Name))
	}
	if task.Priority > 0 {
		metadataParts = append(metadataParts, renderPriorityBadge(task.Priority))
	}
	if task.DueDate != nil {
		dueStr := FormatRelativeDueTask(task, true, "Jan 2")
		metadataParts = append(metadataParts, "Due: "+dueStr)
	}
	if project != nil {
		if profile := m.runtime.Automation().GetProjectScoringProfile(project.ID); profile != nil {
			if scoreBadge := RenderScoreBadge(task, profile); scoreBadge != "" {
				scoreStr := scoreBadge
				if profile.Name != "" {
					scoreStr += " " + MutedStyle.Render("("+profile.Name+")")
				}
				metadataParts = append(metadataParts, scoreStr)
			}
		}
	}
	if !task.UpdatedAt.IsZero() {
		metadataParts = append(metadataParts, MutedStyle.Render(formatRelativeTime(task.UpdatedAt)))
	}
	// Indicators dropped from compact cards (dense-row hierarchy) surface here.
	if ind := renderChecklistIndicator(task); ind != "" {
		metadataParts = append(metadataParts, ind)
	}
	if ind := renderDescriptionIndicator(task); ind != "" {
		metadataParts = append(metadataParts, ind)
	}
	if ind := renderGitHubIssueIndicator(task); ind != "" {
		metadataParts = append(metadataParts, ind)
	}

	var line2 string
	if len(metadataParts) > 0 {
		line2 = " " + strings.Join(metadataParts, sep)
	}
	sb.WriteString(truncateToVisualWidth(line2, w) + "\n")

	if paneH < 3 {
		return sb.String()
	}

	// ── Line 3: tags │ blockers ───────────────────────────────────────────
	var detailParts []string

	if len(task.Tags) > 0 {
		detailParts = append(detailParts, "Tags: "+renderTruncatedTags(task.Tags, 4, 12))
	}

	if len(task.DependsOn) > 0 {
		titles := m.lookupTaskTitles(task.DependsOn, 2)
		if len(titles) > 0 {
			blocker := "Blocked by: " + strings.Join(titles, ", ")
			if len(task.DependsOn) > 2 {
				blocker += fmt.Sprintf(" +%d", len(task.DependsOn)-2)
			}
			detailParts = append(detailParts, blocker)
		} else {
			detailParts = append(detailParts, fmt.Sprintf("Blocked by %d %s", len(task.DependsOn),
				pluralize(len(task.DependsOn), "task", "tasks")))
		}
	}

	var line3 string
	if len(detailParts) > 0 {
		line3 = " " + strings.Join(detailParts, sep)
	}
	sb.WriteString(truncateToVisualWidth(line3, w) + "\n")

	// Remaining lines
	for i := 3; i < paneH; i++ {
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
