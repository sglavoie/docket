package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// renderFullWidth renders the outliner tree using the full terminal width.
//
// The content is wrapped and then defensively fitted to m.width. Lip Gloss'
// width wrapper handles ordinary long headers, but some styled/wide-glyph
// boundary cases can still produce a line one cell over budget; the final pass
// keeps the full-width surface inside the terminal.
func (m *OutlinerModel) renderFullWidth() string {
	if m.width >= inspectorWideBreakpoint {
		return m.renderWideInspectorLayout()
	}
	rendered := lipgloss.NewStyle().Width(m.width).Render(m.renderOutlinerTreeContent(m.width))
	return fitRenderedBlockToWidth(rendered, m.width)
}

// renderWideInspectorLayout keeps the Outliner as the only primary surface
// while reserving a persistent right pane for the task under the cursor.
func (m *OutlinerModel) renderWideInspectorLayout() string {
	const separatorWidth = 1
	inspectorWidth := min(max(m.width*36/100, 36), 48)
	outlinerWidth := max(m.width-inspectorWidth-separatorWidth, 1)
	height := max(m.height, 1)

	outliner := lipgloss.NewStyle().
		Width(outlinerWidth).
		Render(m.renderOutlinerTreeContent(outlinerWidth))
	outliner = fitRenderedBlockToDimensions(outliner, outlinerWidth, height)

	task, project := m.getTaskRef()
	inspector := renderTaskInspectorSummary(task, project, inspectorWidth, height)
	inspector = fitRenderedBlockToDimensions(inspector, inspectorWidth, height)

	separatorLine := lipgloss.NewStyle().Foreground(ColorBorder).Render("│")
	separator := strings.TrimSuffix(strings.Repeat(separatorLine+"\n", height), "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, outliner, separator, inspector)
}

func fitRenderedBlockToWidth(content string, width int) string {
	if width <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			line = truncateIntentional(line, width, "")
		}
		if pad := width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func fitRenderedBlockToDimensions(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			line = truncateIntentional(line, width, "")
		}
		if pad := width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
