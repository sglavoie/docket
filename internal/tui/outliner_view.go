package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Tree connector characters for outliner rendering.
const (
	connectorBranch   = "├── "
	connectorLastItem = "└── "
	connectorPipe     = "│   "
	connectorSpace    = "    "
)

// outlinerRow represents a single flattened row for rendering.
type outlinerRow struct {
	task   *models.Task
	prefix string
}

// buildTreePrefix computes the connector prefix string from the ancestor
// is-last-child boolean stack. Each true entry means that ancestor was the
// last child of its parent (so we render spaces), while false means we need
// a continuation pipe.
func buildTreePrefix(ancestorIsLast []bool, isLast bool) string {
	var b strings.Builder
	for _, last := range ancestorIsLast {
		if last {
			b.WriteString(connectorSpace)
		} else {
			b.WriteString(connectorPipe)
		}
	}
	if isLast {
		b.WriteString(connectorLastItem)
	} else {
		b.WriteString(connectorBranch)
	}
	return b.String()
}

// flattenTaskTree recursively walks the task tree and produces a flat list of
// outlinerRows with pre-computed connector prefixes. The ancestorIsLast stack
// tracks whether each ancestor is its parent's last child, which determines
// whether continuation pipes or spaces are drawn.
func flattenTaskTree(tasks []*models.Task, ancestorIsLast []bool, rows *[]outlinerRow) {
	for i, task := range tasks {
		isLast := i == len(tasks)-1
		prefix := ""
		if len(ancestorIsLast) > 0 || len(tasks) > 0 {
			prefix = buildTreePrefix(ancestorIsLast, isLast)
		}
		*rows = append(*rows, outlinerRow{task: task, prefix: prefix})

		if len(task.Children) > 0 {
			flattenTaskTree(task.Children, append(ancestorIsLast, isLast), rows)
		}
	}
}

// View implements tea.Model. It renders the full outliner tree with connector
// prefixes computed on each render pass from the project's task hierarchy.
func (m *OutlinerModel) View() string {
	if m.project == nil {
		return "No project loaded"
	}

	roots := m.project.GetRootTasks()
	if len(roots) == 0 {
		return TitleStyle.Render(m.project.Name) + "\n\n" + MutedStyle.Render("No tasks")
	}

	// Rebuild flat row list and clamp cursor.
	m.rebuildRows()

	var b strings.Builder
	b.WriteString(TitleStyle.Render(m.project.Name))
	b.WriteString("\n\n")

	// Compute scroll window.
	visibleRows := m.contentHeight()
	startIdx := calcScrollOffset(m.cursor, visibleRows)
	endIdx := min(startIdx+visibleRows, len(m.rows))

	if startIdx > 0 {
		b.WriteString(ScrollUpIndicator(startIdx))
		b.WriteString("\n")
	}

	for i := startIdx; i < endIdx; i++ {
		row := m.rows[i]
		task := row.task
		selected := i == m.cursor

		typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
		idStr := MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID))

		// Title with priority coloring
		titleStyle := NormalStyle
		if task.Status == models.StatusCanceled {
			titleStyle = CanceledStyle
		} else if task.Active {
			titleStyle = ActiveStyle
		} else if task.Pinned {
			titleStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
		}

		// Build inline indicators reusing existing helpers
		var indicators strings.Builder

		// Priority badge (only for high/medium)
		if task.Priority <= 2 {
			indicators.WriteString(" ")
			indicators.WriteString(renderPriorityBadge(task.Priority))
		}

		// Due date badge
		if dueStr := formatDueDate(task); dueStr != "" {
			indicators.WriteString(" ")
			indicators.WriteString(dueStr)
		}

		// Tag badges (first 2 tags, truncated)
		if len(task.Tags) > 0 {
			maxTags := 2
			maxTagLen := 8
			var tagParts []string
			for j, tag := range task.Tags {
				if j >= maxTags {
					break
				}
				displayTag := tag
				if len(displayTag) > maxTagLen {
					displayTag = displayTag[:maxTagLen-1] + "…"
				}
				tagParts = append(tagParts, "#"+displayTag)
			}
			indicators.WriteString(" ")
			indicators.WriteString(TagInlineStyle.Render(strings.Join(tagParts, " ")))
			if len(task.Tags) > maxTags {
				indicators.WriteString(MutedStyle.Render(fmt.Sprintf(" +%d", len(task.Tags)-maxTags)))
			}
		}

		// Progress for parent tasks
		if task.HasProgressChildren() {
			pct := task.GetProgress()
			indicators.WriteString(MutedStyle.Render(fmt.Sprintf(" [%.0f%%]", pct)))
		}

		// Cursor prefix
		cursorPrefix := "  "
		if selected {
			cursorPrefix = SelectionCursorStyle.Render(">") + " "
		}

		// Truncate title to fit available width
		availableWidth := max(m.width-lipgloss.Width(row.prefix)-lipgloss.Width(cursorPrefix)-12-lipgloss.Width(indicators.String()), 10)
		title := titleStyle.Render(util.Truncate(task.Title, availableWidth))

		line := fmt.Sprintf("%s%s%s %s %s %s%s",
			cursorPrefix,
			MutedStyle.Render(row.prefix),
			typeIcon, statusIcon, idStr, title, indicators.String())

		if selected {
			line = SelectionRowStyle.Width(m.width).Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	if endIdx < len(m.rows) {
		b.WriteString(ScrollDownIndicator(len(m.rows) - endIdx))
		b.WriteString("\n")
	}

	return b.String()
}
