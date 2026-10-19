package conflict

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// conflictBrowserInnerWidth is the overlay box width the browser claims, matching
// the lens picker's so the two overlays sit in the same frame at every terminal
// size.
func conflictBrowserInnerWidth(width int) int {
	return min(max(width*4/5, 46), max(width-6, 24))
}

func (model *ConflictBrowserModel) Render(width int) string {
	return model.RenderWithin(width, 0)
}

// RenderWithin budgets the body after reserving rows for the modal chrome, so a
// minimum-supported terminal drops list rows or detail lines rather than losing
// the title, the header block, or the footer.
func (model *ConflictBrowserModel) RenderWithin(width, height int) string {
	if model == nil || model.surface == nil {
		return ""
	}
	innerWidth := conflictBrowserInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	header := make([]string, 0, 2)
	if model.notice != "" && (!model.inspecting() || model.editorActive() && model.confirmation == nil) {
		noticeLines := textfit.WrapText(model.notice, contentWidth)
		for _, line := range noticeLines[:min(len(noticeLines), 4)] {
			header = append(header, theme.ErrorStatusStyle.Render(line))
		}
	}
	if !model.inspecting() {
		line := conflictBrowserIntro
		for _, wrapped := range textfit.WrapText(line, contentWidth) {
			header = append(header, theme.MutedStyle.Render(wrapped))
		}
	}

	footer := chrome.RenderGuidanceText(model.guidanceSpec(), contentWidth)
	// Halo/frame, title and footer, plus the header's separating blank row.
	overhead := 5 + lipgloss.Height(footer)
	if len(header) > 0 {
		overhead += len(header) + 1
	}
	bodyHeight := 0
	if height > 0 {
		bodyHeight = max(height-overhead, 1)
	}

	content := model.renderList(contentWidth, bodyHeight)
	if model.inspecting() {
		content = model.renderDetail(contentWidth, bodyHeight)
		if model.editorActive() && model.confirmation == nil {
			editorHeight := 24
			if bodyHeight > 0 {
				editorHeight = min(editorHeight, bodyHeight)
			}
			content = model.editor.render(contentWidth, editorHeight)
		}
	}
	if len(header) > 0 {
		content = strings.Join(header, "\n") + "\n\n" + content
	}

	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(conflictBrowserTitle)
	return chrome.RenderOverlayBox(
		content,
		innerWidth,
		title,
		footer,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}

func (model *ConflictBrowserModel) renderList(width, bodyHeight int) string {
	listHeight := min(max(len(model.surface.Rows()), conflictBrowserMinBodyRows), conflictBrowserMaxListRows)
	if bodyHeight > 0 {
		listHeight = min(listHeight, bodyHeight)
	}
	return listsurface.RenderCompactListSurface(model.surface, width, listHeight)
}

// renderDetail wraps the pane at the box's width and paints one window of it.
// The wrapped count and the window height are recorded here because this is the
// only place either is known, and the scroll keys clamp against them.
func (model *ConflictBrowserModel) renderDetail(width, bodyHeight int) string {
	lines := model.detail
	if model.notice != "" {
		lines = append([]string{model.notice, ""}, lines...)
	}
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			wrapped = append(wrapped, "")
			continue
		}
		for _, part := range textfit.WrapText(line, width) {
			if model.notice != "" && line == model.notice {
				wrapped = append(wrapped, theme.ErrorStatusStyle.Render(part))
			} else {
				wrapped = append(wrapped, paintDiffLine(line, part))
			}
		}
	}
	paneHeight := min(max(len(wrapped), conflictBrowserMinBodyRows), conflictBrowserMaxDetailRows)
	if bodyHeight > 0 {
		paneHeight = min(paneHeight, bodyHeight)
	}
	model.detailLines = len(wrapped)
	model.detailHeight = paneHeight
	model.detailOffset = min(model.detailOffset, max(len(wrapped)-paneHeight, 0))

	window := wrapped[model.detailOffset:min(model.detailOffset+paneHeight, len(wrapped))]
	return strings.Join(window, "\n")
}

// guidanceSpec names the controls of the active resolution step.
func (model *ConflictBrowserModel) guidanceSpec() chrome.GuidanceSpec {
	if model == nil || model.surface == nil {
		return chrome.GuidanceSpec{}
	}
	if model.confirmation != nil {
		return chrome.RequiredExitGuidanceSpec("Enter confirm", "Esc cancel")
	}
	if model.discard {
		return chrome.RequiredExitGuidanceSpec("y discard", "n/Esc keep draft")
	}
	if model.editorActive() {
		return chrome.RequiredExitGuidanceSpec("Ctrl+S review merge", "Esc back (keep draft)")
	}
	if model.inspecting() {
		return chrome.RequiredExitGuidanceSpec("d diff/details", "Esc back", "b current/base diff", model.resolutionKeys(), model.scrollKeys())
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("j", "Move through the conflicts", "Navigation",
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey("j/k or ↑/↓")),
		keymap.Bind("enter", "Inspect the conflict", "Actions",
			keymap.WithFooter(1, "inspect", true), keymap.WithFooterKey("Enter")),
		keymap.Bind("esc", "Unwind the conflict browser", "Navigation",
			keymap.WithFooter(2, "close", true), keymap.WithFooterKey("Esc")),
	}}
	spec, _ := listsurface.ListSurfaceGuidanceSpec(model.surface.GuidanceState(false), km, nil)
	return spec
}
