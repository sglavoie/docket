package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// renderPeekOverlay renders the peek task detail overlay when peekActive is true.
//
// Layout: near-fullscreen two-pane split. Left pane shows the task description;
// right pane stacks the complementary sections (Checklist, Decisions, Comments,
// Attachments) — each hidden when empty after filtering resolved entries.
// Both panes are independently scrollable; the focused pane uses an accent
// border. As a side effect, this method caches the per-pane line counts and
// visible pane height on the AppModel so the key handler can clamp scroll
// without re-rendering.
func (m *AppModel) renderPeekOverlay() string {
	if !m.peekActive || m.peekTask == nil {
		return ""
	}
	task := m.peekTask

	focusMarker := "◀"
	if m.peekFocusRight {
		focusMarker = "▶"
	}
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	title := titleStyle.Render("Peek") + " " + MutedStyle.Render(focusMarker)
	footerText := HelpStyle.Render("↑/↓ scroll · tab switch pane · esc/space close")

	outerWidth := min(max(m.width*92/100, 40), m.width-2)
	outerHeight := min(max(m.height*88/100, 10), m.height-2)

	innerWidth := max(outerWidth-2, 20)
	// renderTitledBox uses one-cell borders plus one-cell horizontal padding,
	// so the content itself must be four cells narrower than the styled width.
	contentWidth := max(innerWidth-4, 10)

	sepWidth := 1
	leftWidth := max((contentWidth-sepWidth)/2, 8)
	rightWidth := max(contentWidth-sepWidth-leftWidth, 8)

	paneHeight := max(outerHeight-2, 3)

	leftLines := renderPeekDescription(task, leftWidth)
	rightLines := renderPeekRightPane(task, rightWidth)

	m.peekLeftLines = len(leftLines)
	m.peekRightLines = len(rightLines)
	m.peekPaneHeight = paneHeight

	m.peekLeftOffset = clampPeekOffset(m.peekLeftOffset, m.peekLeftLines, paneHeight)
	m.peekRightOffset = clampPeekOffset(m.peekRightOffset, m.peekRightLines, paneHeight)

	leftView := buildPeekPane(leftLines, m.peekLeftOffset, leftWidth, paneHeight)
	rightView := buildPeekPane(rightLines, m.peekRightOffset, rightWidth, paneHeight)

	sepLine := lipgloss.NewStyle().Foreground(ColorPrimary).Render("│")
	sepCol := strings.Repeat(sepLine+"\n", paneHeight-1) + sepLine
	pane := lipgloss.JoinHorizontal(lipgloss.Top, leftView, sepCol, rightView)

	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	return renderTitledBox(pane, innerWidth, title, footerText, borderStyle)
}

// activatePeek opens the peek overlay for the given task, zero-initializing
// scroll/focus state so the overlay always starts at the top of both panes
// with description (left) pane focused.
func (m *AppModel) activatePeek(task *models.Task, project *models.Project) {
	m.peekTask = task
	m.peekProject = project
	m.peekActive = true
	m.peekLeftOffset = 0
	m.peekRightOffset = 0
	m.peekFocusRight = false
}

// dismissPeek clears all peek state.
func (m *AppModel) dismissPeek() {
	m.peekTask = nil
	m.peekProject = nil
	m.peekActive = false
	m.peekLeftOffset = 0
	m.peekRightOffset = 0
	m.peekFocusRight = false
	m.peekLeftLines = 0
	m.peekRightLines = 0
	m.peekPaneHeight = 0
}

// clampPeekOffset bounds a scroll offset to [0, max(0, total-visible)].
func clampPeekOffset(offset, total, visible int) int {
	if offset < 0 {
		return 0
	}
	maxOffset := max(total-visible, 0)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

// buildPeekPane slices lines into a fixed-height pane window, padding each line
// (and the bottom of the pane) to width/height so JoinHorizontal aligns cleanly.
func buildPeekPane(lines []string, offset, width, height int) string {
	out := make([]string, 0, height)
	for i := range height {
		idx := offset + i
		var line string
		if idx < len(lines) {
			line = lines[idx]
		}
		out = append(out, padPeekLine(line, width))
	}
	return strings.Join(out, "\n")
}

// padPeekLine fits a (possibly styled) line to width by visual width: right-pads
// if short, ansi-truncates if long. Truncation is a safety net so the surrounding
// box style's Width() can't hardwrap an over-wide line into the neighboring pane.
func padPeekLine(line string, width int) string {
	w := lipgloss.Width(line)
	if w == width {
		return line
	}
	if w > width {
		return truncateIntentional(line, width, "")
	}
	return line + strings.Repeat(" ", width-w)
}

func appendPeekWrappedItem(lines []string, firstPrefix, nextPrefix, body string, width int) []string {
	if width <= 0 {
		return lines
	}

	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	parts := strings.Split(body, "\n")
	if len(parts) == 0 {
		parts = []string{""}
	}

	for partIndex, part := range parts {
		prefix := firstPrefix
		if partIndex > 0 {
			prefix = nextPrefix
		}
		wrapped := wrapStyled(part, peekBodyWrapWidth(width, prefix, nextPrefix))
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		for lineIndex, line := range wrapped {
			linePrefix := prefix
			if lineIndex > 0 {
				linePrefix = nextPrefix
			}
			lines = append(lines, truncatePeekLine(linePrefix+line, width))
		}
	}
	return lines
}

func peekBodyWrapWidth(width int, prefixes ...string) int {
	bodyWidth := width
	for _, prefix := range prefixes {
		bodyWidth = min(bodyWidth, width-lipgloss.Width(prefix))
	}
	return max(bodyWidth, 1)
}

func truncatePeekLine(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}
	return truncateIntentional(line, width, "")
}

// renderPeekDescription wraps the task description to width and returns its
// lines. Returns a single-line "No description" placeholder when empty.
func renderPeekDescription(task *models.Task, width int) []string {
	if strings.TrimSpace(task.Description) == "" {
		return []string{MutedStyle.Render("No description")}
	}
	var out []string
	for raw := range strings.SplitSeq(strings.TrimRight(task.Description, "\n"), "\n") {
		if raw == "" {
			out = append(out, "")
			continue
		}
		wrapped := wrapText(raw, width)
		if len(wrapped) == 0 {
			out = append(out, "")
		} else {
			out = append(out, wrapped...)
		}
	}
	return out
}

// renderPeekRightPane stacks Checklist → Decisions → Comments → Attachments
// blocks (each omitted when empty after filtering resolved entries). Returns
// a single-line "No metadata" placeholder when every block is empty.
func renderPeekRightPane(task *models.Task, width int) []string {
	var out []string
	appendBlock := func(block []string) {
		if len(block) == 0 {
			return
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, block...)
	}

	appendBlock(peekChecklistBlock(task, width))
	appendBlock(peekDecisionsBlock(task, width))
	appendBlock(peekCommentsBlock(task, width))
	appendBlock(peekAttachmentsBlock(task, width))
	if len(out) == 0 {
		return []string{MutedStyle.Render("No metadata")}
	}
	return out
}

func peekChecklistBlock(task *models.Task, width int) []string {
	if len(task.Checklist) == 0 {
		return nil
	}
	done, total := 0, len(task.Checklist)
	for _, item := range task.Checklist {
		if item.Done {
			done++
		}
	}
	header := BoldStyle.Render(fmt.Sprintf("Checklist (%d/%d)", done, total))
	bar := renderChecklistBar(task, width)
	lines := []string{header}
	if bar != "" {
		lines = append(lines, bar)
	}

	for _, item := range task.Checklist {
		text := strings.ReplaceAll(item.Text, "\n", " ")
		var prefix, body string
		if item.Done {
			prefix = SuccessStyle.Render("[x]") + " "
			body = MutedStyle.Render(text)
		} else {
			prefix = MutedStyle.Render("[ ]") + " "
			body = text
		}
		lines = appendPeekWrappedItem(lines, prefix, "    ", body, width)
	}
	return lines
}

func peekDecisionsBlock(task *models.Task, width int) []string {
	var active []*models.Decision
	for _, d := range task.Decisions {
		if d != nil && !d.Resolved {
			active = append(active, d)
		}
	}
	if len(active) == 0 {
		return nil
	}
	lines := []string{BoldStyle.Render(fmt.Sprintf("Decisions (%d)", len(active)))}
	for _, d := range active {
		body := d.Text
		if d.Category != "" {
			body = MutedStyle.Render(fmt.Sprintf("[%s]", d.Category)) + " " + d.Text
		}
		lines = appendPeekWrappedItem(lines, "• ", "  ", body, width)
		if strings.TrimSpace(d.Rationale) != "" {
			lines = appendPeekWrappedItem(lines, "  ", "  ", MutedStyle.Render(d.Rationale), width)
		}
	}
	return lines
}

func peekCommentsBlock(task *models.Task, width int) []string {
	var active []*models.Comment
	for _, c := range task.Comments {
		if c != nil && !c.Resolved {
			active = append(active, c)
		}
	}
	if len(active) == 0 {
		return nil
	}
	lines := []string{BoldStyle.Render(fmt.Sprintf("Comments (%d)", len(active)))}
	for _, c := range active {
		date := c.CreatedAt.Format("2006-01-02")
		body := MutedStyle.Render(date+":") + " " + c.Text
		lines = appendPeekWrappedItem(lines, "• ", "  ", body, width)
	}
	return lines
}

func peekAttachmentsBlock(task *models.Task, width int) []string {
	if len(task.Attachments) == 0 {
		return nil
	}
	lines := []string{BoldStyle.Render(fmt.Sprintf("Attachments (%d)", len(task.Attachments)))}
	for _, a := range task.Attachments {
		body := a.Filename
		if ref, ok := gitHubIssueAttachmentRef(a); ok {
			body = "GitHub issue " + formatGitHubIssueRef(ref)
		} else if a.IsCodeRef && a.LineNumber > 0 {
			body = fmt.Sprintf("%s:%d", a.Filename, a.LineNumber)
		} else if a.IsURL && a.OriginalPath != "" && a.OriginalPath != a.Filename {
			body = a.Filename + " " + MutedStyle.Render("("+a.OriginalPath+")")
		}
		lines = appendPeekWrappedItem(lines, "• ", "  ", body, width)
	}
	return lines
}
