package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// applyOverlays composites all popup/overlay layers onto the base content.
// h is the overlay positioning height (m.height for full-height views,
// targetContentHeight for views with an external status bar).
//
// Overlay visual contract:
//   - View-local prefix popups: no dim (user is mid-chord, context stays visible)
//   - Global prefix popups (";"): dim (full-screen mode change)
//   - Quick-switch popup: no dim (navigational, lightweight)
//   - Project cycle picker: dim (selection overlay)
//   - Pickers/selection overlays (dimmedOverlayRenderers): dim
//   - Peek/preview: dim (replaces context with detail)
//   - Confirm dialogs: dim (destructive action requires focus)
func (m *AppModel) applyOverlays(content string, h int) string {
	// Overlay prefix popup if the current view provides one
	if popup := m.renderPrefixPopupOverlay(); popup != "" {
		content = OverlayPopup(content, popup, m.width, h)
	}

	// Overlay semicolon prefix popup if active (global, not view-specific)
	if popup := m.renderSemicolonPrefixPopup(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay quick-switch popup if active
	if popup := m.renderQuickSwitchPopup(); popup != "" {
		content = OverlayPopup(content, popup, m.width, h)
		// Overlay prefix hints on top of quick-switch popup when prefix is pending
		if m.quickSwitchPendingPrefix != "" {
			var qsPriority int
			if m.quickSwitchCursor >= 0 && m.quickSwitchCursor < m.quickSwitchFilteredLen() {
				realIdx := m.quickSwitchFiltered[m.quickSwitchCursor]
				qsPriority = m.recentTaskVisits[realIdx].Task.Priority
			}
			prefixPopup := RenderPrefixPopup(m.quickSwitchPendingPrefix, PrefixHints{",": StatusPrefixHints, "!": BuildPriorityPrefixHints(qsPriority)}, m.width, h)
			content = OverlayPopup(content, prefixPopup, m.width, h)
		}
	}

	// Overlay project cycle popup if active (bottom-left to avoid covering sidebar)
	if popup := m.renderProjectCyclePopup(); popup != "" {
		content = OverlayPopupBottomLeftDimmed(content, popup, m.width, h)
	}

	// Overlay project cycle picker popup if active
	if m.showProjectCyclePicker {
		popup := RenderPrefixPopupWithCategory("`", "project", m.buildProjectCyclePickerHints(), m.width, h)
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Replace base content with task preview when pinned popup is active
	if m.showPinnedPopup && len(m.pinnedPopupItems) > 0 {
		if preview := m.renderPinnedPreviewBackground(); preview != "" {
			content = preview
		}
	}

	// Overlay pinned tasks popup if active
	if popup := m.renderPinnedPopup(); popup != "" {
		content = OverlayPopupBottomLeft(content, popup, m.width, h)
		// Overlay prefix hints on top of pinned popup when prefix is pending
		if m.pinnedPendingPrefix != "" {
			var pinnedPriority int
			if m.pinnedPopupCursor >= 0 && m.pinnedPopupCursor < m.pinnedPopupFilteredLen() {
				realIdx := m.pinnedPopupFiltered[m.pinnedPopupCursor]
				pinnedPriority = m.pinnedPopupItems[realIdx].Task.Priority
			}
			prefixPopup := RenderPrefixPopup(m.pinnedPendingPrefix, PrefixHints{",": StatusPrefixHints, "!": BuildPriorityPrefixHints(pinnedPriority)}, m.width, h)
			content = OverlayPopupBottomLeft(content, prefixPopup, m.width, h)
		}
	}

	// Overlay dimmed picker/note popups (all share the same nil-check + render pattern)
	for _, renderer := range m.dimmedOverlayRenderers() {
		if popup := renderer.render(m.width); popup != "" {
			content = OverlayPopupDimmed(content, popup, m.width, h)
		}
	}

	// Overlay peek task detail if active
	if popup := m.renderPeekOverlay(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay floating quick capture if active
	if popup := m.renderQuickCaptureOverlay(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	// Overlay confirm dialog last so it renders on top of all other popups
	if popup := m.renderConfirmDialogOverlay(); popup != "" {
		content = OverlayPopupDimmed(content, popup, m.width, h)
	}

	return content
}

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

	// Outer overlay dimensions: ~92% width, ~88% height.
	outerWidth := max(m.width*92/100, 40)
	if outerWidth > m.width-2 {
		outerWidth = m.width - 2
	}
	outerHeight := max(m.height*88/100, 10)
	if outerHeight > m.height-2 {
		outerHeight = m.height - 2
	}

	// renderTitledBox uses a rounded border (1 col each side) + padding (1 col
	// each side) = 4 cols of horizontal chrome. The Width passed to lipgloss is
	// the inner content width (border excluded). We pre-compute that here.
	innerWidth := max(outerWidth-2, 20) // subtract border (2)
	// Padding inside the box (1 + 1) further trims content width.
	contentWidth := max(innerWidth-2, 10)

	// Reserve 1 col for the vertical separator between panes.
	sepWidth := 1
	leftWidth := max((contentWidth-sepWidth)/2, 8)
	rightWidth := max(contentWidth-sepWidth-leftWidth, 8)

	// Pane height: outer height minus top/bottom border (2) minus footer line (1).
	// Footer is part of the bottom border in renderTitledBox, but the box content
	// is just our pane area, so subtract 2 for border lines.
	paneHeight := max(outerHeight-2, 3)

	leftLines := renderPeekDescription(task, leftWidth)
	rightLines := renderPeekRightPane(task, rightWidth)

	// Cache for the key handler.
	m.peekLeftLines = len(leftLines)
	m.peekRightLines = len(rightLines)
	m.peekPaneHeight = paneHeight

	// Clamp offsets in case content shrunk (e.g., on resize).
	m.peekLeftOffset = clampPeekOffset(m.peekLeftOffset, m.peekLeftLines, paneHeight)
	m.peekRightOffset = clampPeekOffset(m.peekRightOffset, m.peekRightLines, paneHeight)

	leftView := buildPeekPane(leftLines, m.peekLeftOffset, leftWidth, paneHeight)
	rightView := buildPeekPane(rightLines, m.peekRightOffset, rightWidth, paneHeight)

	// Separator: accent-colored on the focused side to reinforce the title's
	// focus marker without needing per-pane content dimming (which would mangle
	// inner ANSI styling).
	sepColor := ColorPrimary
	sepLine := lipgloss.NewStyle().Foreground(sepColor).Render("│")
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
	maxOffset := total - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

// buildPeekPane slices lines into a fixed-height pane window, padding each line
// (and the bottom of the pane) to width/height so JoinHorizontal aligns cleanly.
func buildPeekPane(lines []string, offset, width, height int) string {
	out := make([]string, 0, height)
	for i := 0; i < height; i++ {
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
		return ansi.Truncate(line, width, "")
	}
	return line + strings.Repeat(" ", width-w)
}

// renderPeekDescription wraps the task description to width and returns its
// lines. Returns a single-line "No description" placeholder when empty.
func renderPeekDescription(task *models.Task, width int) []string {
	if strings.TrimSpace(task.Description) == "" {
		return []string{MutedStyle.Render("No description")}
	}
	var out []string
	for _, raw := range strings.Split(strings.TrimRight(task.Description, "\n"), "\n") {
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
	// Prefix is "[x] " / "[ ] " (4 visual cells) to match the project's checkbox
	// convention (see task_checklist_view.go). Ambiguous-width glyphs like ✔/✗
	// were unsafe — lipgloss measures them as 1 cell but many terminals render
	// them as 2, which threw off the wrap-width math and visibly broke wrapping
	// on multi-line items.
	bodyWidth := max(width-4, 4)
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
		for i, w := range wrapStyled(body, bodyWidth) {
			if i == 0 {
				lines = append(lines, prefix+w)
			} else {
				lines = append(lines, "    "+w)
			}
		}
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
	bodyWidth := max(width-2, 4)
	for _, d := range active {
		body := d.Text
		if d.Category != "" {
			body = MutedStyle.Render(fmt.Sprintf("[%s]", d.Category)) + " " + d.Text
		}
		for i, w := range wrapStyled(body, bodyWidth) {
			if i == 0 {
				lines = append(lines, "• "+w)
			} else {
				lines = append(lines, "  "+w)
			}
		}
		if strings.TrimSpace(d.Rationale) != "" {
			for _, w := range wrapStyled(d.Rationale, bodyWidth) {
				lines = append(lines, "  "+MutedStyle.Render(w))
			}
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
	bodyWidth := max(width-2, 4)
	for _, c := range active {
		date := c.CreatedAt.Format("2006-01-02")
		body := MutedStyle.Render(date+":") + " " + c.Text
		for i, w := range wrapStyled(body, bodyWidth) {
			if i == 0 {
				lines = append(lines, "• "+w)
			} else {
				lines = append(lines, "  "+w)
			}
		}
	}
	return lines
}

func peekAttachmentsBlock(task *models.Task, width int) []string {
	if len(task.Attachments) == 0 {
		return nil
	}
	lines := []string{BoldStyle.Render(fmt.Sprintf("Attachments (%d)", len(task.Attachments)))}
	bodyWidth := max(width-2, 4)
	for _, a := range task.Attachments {
		body := a.Filename
		if a.IsCodeRef && a.LineNumber > 0 {
			body = fmt.Sprintf("%s:%d", a.Filename, a.LineNumber)
		} else if a.IsURL && a.OriginalPath != "" && a.OriginalPath != a.Filename {
			body = a.Filename + " " + MutedStyle.Render("("+a.OriginalPath+")")
		}
		for i, w := range wrapStyled(body, bodyWidth) {
			if i == 0 {
				lines = append(lines, "• "+w)
			} else {
				lines = append(lines, "  "+w)
			}
		}
	}
	return lines
}

// widthRenderer is satisfied by all overlay pickers/popups that render given a width.
type widthRenderer interface {
	render(width int) string
}

// dimmedOverlayRenderers returns the non-nil overlay renderers that are composited
// with OverlayPopupDimmed. Order matters: later entries render on top.
func (m *AppModel) dimmedOverlayRenderers() []widthRenderer {
	var out []widthRenderer
	if m.globalProjectPicker != nil {
		out = append(out, m.globalProjectPicker)
	}
	if m.globalTaskPicker != nil {
		out = append(out, m.globalTaskPicker)
	}
	if m.statusQuickPicker != nil {
		out = append(out, m.statusQuickPicker)
	}
	if m.sectionQuickPicker != nil {
		out = append(out, m.sectionQuickPicker)
	}
	if m.linkTypeQuickPicker != nil {
		out = append(out, m.linkTypeQuickPicker)
	}
	if m.linkModeQuickPicker != nil {
		out = append(out, m.linkModeQuickPicker)
	}
	if m.priorityQuickPicker != nil {
		out = append(out, m.priorityQuickPicker)
	}
	if m.dueDateQuickPicker != nil {
		out = append(out, m.dueDateQuickPicker)
	}
	if m.sectionChangeQuickPicker != nil {
		out = append(out, m.sectionChangeQuickPicker)
	}
	if m.quickNoteOverlay != nil {
		out = append(out, m.quickNoteOverlay)
	}
	return out
}

// renderPrefixPopupOverlay checks if the current view has a pending prefix and
// renders the popup box for overlay compositing.
func (m *AppModel) renderPrefixPopupOverlay() string {
	view := m.views.Get(m.currentView())
	if view == nil {
		return ""
	}
	provider, ok := view.(PrefixPopupProvider)
	if !ok {
		return ""
	}
	info := provider.GetPrefixPopupInfo()
	if info == nil {
		return ""
	}
	return RenderPrefixPopup(info.Prefix, info.Hints, m.width, m.height)
}

// renderSemicolonPrefixPopup renders the ";" or ";t" prefix popup.
func (m *AppModel) renderSemicolonPrefixPopup() string {
	if m.pendingSemicolonTPrefix {
		hints := PrefixHints{";t": SemicolonTPrefixHints}
		return RenderPrefixPopup(";t", hints, m.width, m.height)
	}
	if !m.pendingSemicolonPrefix {
		return ""
	}
	hints := PrefixHints{";": m.buildSemicolonHints()}
	return RenderPrefixPopup(";", hints, m.width, m.height)
}

// buildSemicolonHints returns a copy of SemicolonPrefixHints with context-dependent disabling.
// Shortcuts pointing to empty/unavailable data are dimmed so users understand at a glance
// which destinations are currently active.
func (m *AppModel) buildSemicolonHints() []PrefixHint {
	hints := make([]PrefixHint, len(SemicolonPrefixHints))
	copy(hints, SemicolonPrefixHints)

	hasAlerts := m.alertCount() > 0
	hasWorkflowRules := m.config != nil && len(m.config.WorkflowRules) > 0

	for i := range hints {
		switch hints[i].Key {
		case "n":
			hints[i].Disabled = !hasAlerts
		case "W":
			hints[i].Disabled = !hasWorkflowRules
		}
	}
	return hints
}

// renderConfirmDialogOverlay checks if the current view has an active confirm
// dialog and renders it as a floating overlay box.
func (m *AppModel) renderConfirmDialogOverlay() string {
	view := m.views.Get(m.currentView())
	if view == nil {
		return ""
	}
	provider, ok := view.(ConfirmDialogProvider)
	if !ok {
		return ""
	}
	cd := provider.GetConfirmDialog()
	if cd == nil || !cd.IsActive() {
		return ""
	}
	return cd.RenderOverlay(m.width)
}

// renderQuickCaptureOverlay renders the floating quick capture box when the
// overlay is active. The box is centered and dimmed over the current view.
func (m *AppModel) renderQuickCaptureOverlay() string {
	if m.quickCaptureOverlay == nil {
		return ""
	}
	qc := m.quickCaptureOverlay

	innerWidth := max(min(m.width*3/5, m.width-4), 60)
	qc.SetSize(innerWidth, 0)

	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Quick Capture")
	footer := HelpStyle.Render(qc.overlayHelpText())
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	return renderTitledBox(qc.renderOverlayContent(), innerWidth, title, footer, borderStyle)
}
