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

// breadcrumb returns the rendered breadcrumb string showing the ancestor chain
// from the first zoom level to the current zoom target. Returns empty string
// when not zoomed. If the breadcrumb exceeds maxWidth, leftmost segments are
// dropped and replaced with "..." until it fits.
func (m *OutlinerModel) breadcrumb(maxWidth int) string {
	if len(m.zoomStack) == 0 || m.project == nil {
		return ""
	}

	separator := " > "
	var segments []string
	for _, entry := range m.zoomStack {
		if t := m.project.GetTaskByID(entry.TaskID); t != nil {
			segments = append(segments, t.Title)
		}
	}
	if len(segments) == 0 {
		return ""
	}

	full := strings.Join(segments, separator)
	rendered := MutedStyle.Render(full)
	if lipgloss.Width(rendered) <= maxWidth {
		return rendered
	}

	// Truncate from the left, replacing dropped segments with "...".
	ellipsis := "..."
	for len(segments) > 1 {
		segments = segments[1:]
		candidate := ellipsis + separator + strings.Join(segments, separator)
		rendered = MutedStyle.Render(candidate)
		if lipgloss.Width(rendered) <= maxWidth {
			return rendered
		}
	}

	// Even the last segment alone with ellipsis doesn't fit; render it anyway.
	return MutedStyle.Render(ellipsis + separator + segments[0])
}

// View implements tea.Model. It renders the full outliner tree with connector
// prefixes computed on each render pass from the project's task hierarchy.
func (m *OutlinerModel) View() string {
	if m.project == nil {
		return "No project loaded"
	}

	// Rebuild flat row list with perspective filter and clamp cursor.
	m.rebuildRows()

	// Capture and clear the transient zoom notice (shown for one render).
	notice := m.zoomNotice
	m.zoomNotice = ""

	perspective := m.currentPerspective()

	// Show perspective label only for non-All perspectives.
	perspectiveLabel := ""
	if perspective.Filter != nil {
		perspectiveLabel = "  " + MutedStyle.Render("["+perspective.Name+"]")
	}

	bc := m.breadcrumb(m.width)

	if len(m.rows) == 0 {
		header := TitleStyle.Render(m.project.Name) + perspectiveLabel
		emptyMsg := "No tasks"
		if perspective.Filter != nil && len(m.zoomStack) > 0 {
			topID := m.zoomStack[len(m.zoomStack)-1].TaskID
			if t := m.project.GetTaskByID(topID); t != nil {
				emptyMsg = fmt.Sprintf("No tasks match the %s perspective within %q", perspective.Name, t.Title)
			} else {
				emptyMsg = fmt.Sprintf("No tasks match the %s perspective", perspective.Name)
			}
		} else if perspective.Filter != nil {
			emptyMsg = fmt.Sprintf("No tasks match the %s perspective", perspective.Name)
		}
		result := header + "\n"
		if bc != "" {
			result += bc + "\n"
		}
		result += "\n" + MutedStyle.Render(emptyMsg)
		if notice != "" {
			result += "\n\n" + PreviewWarningStyle.Render(notice)
		}
		return result
	}

	var b strings.Builder
	b.WriteString(TitleStyle.Render(m.project.Name))
	b.WriteString(perspectiveLabel)
	b.WriteString("\n")
	if bc != "" {
		b.WriteString(bc)
		b.WriteString("\n")
	}
	b.WriteString("\n")

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

	if notice != "" {
		b.WriteString("\n")
		b.WriteString(PreviewWarningStyle.Render(notice))
		b.WriteString("\n")
	}

	return b.String()
}
