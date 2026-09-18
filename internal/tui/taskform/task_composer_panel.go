package taskform

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

const (
	// composerPanelChromeRows is the labeled divider, the route row, and the
	// three-row input: the part of the panel that is there at every size,
	// whatever the draft says.
	composerPanelChromeRows = 5
	// composerPanelMinRows is that chrome plus three rows for whatever the panel
	// has to say about the line.
	composerPanelMinRows = composerPanelChromeRows + 3
	// composerPanelMaxRows caps the panel on a tall terminal: capture is a
	// bounded act, and the rows past this one are worth more to the outline
	// behind it.
	composerPanelMaxRows = 14
	// composerHostMinRows is what the panel leaves the presentation underneath
	// even in the smallest frame the app will render at all.
	composerHostMinRows = 4
)

// composerPanelRows is the panel's whole height in a content area of the given
// height. It reads the frame and nothing else — not the draft, not the preview,
// not the errors — because the host is resized to the remainder and a list
// surface drops its scroll window every time it is resized.
func composerPanelRows(height int) int {
	if height <= composerPanelMinRows+composerHostMinRows {
		return max(height-composerHostMinRows, 1)
	}
	return min(max(height*2/5, composerPanelMinRows), composerPanelMaxRows)
}

// HostedPanelLayout answers route.HostedPanelView. Compact capture is a panel
// over the presentation that raised it; the expanded form owns the content area
// outright and says so with the zero layout.
//
// Expanding the syntax guide hides the host for those frames rather than
// shrinking it further: reference text is read instead of the outline, not
// beside it, and the host keeps the size it already had so closing the guide
// costs it no scroll window.
func (m *TaskComposerModel) HostedPanelLayout(_, height int) chrome.HostedPanelLayout {
	if m.Mode == ComposerExpanded {
		return chrome.HostedPanelLayout{}
	}
	return chrome.HostedPanelLayout{
		HostRows:  max(height-composerPanelRows(height), 1),
		HostDrawn: !m.HelpExpanded,
	}
}

// compactPanelRows is the height this panel renders to: the share the layout
// left it, less whatever the shell's footer spends beyond the two rows every
// view is sized against. A footer carrying feedback beside its controls, or
// controls wrapped at minimum width, is taller than that, and the panel is what
// pays — the host below it keeps the rows the layout promised it, so opening
// capture and then saving costs the outline no scroll window.
//
// The excess is taken from the shell's own measurement rather than re-derived
// from this view's guidance, which is the only way the panel and the footer can
// be reserving and drawing the same rows.
func (m *TaskComposerModel) compactPanelRows() int {
	rows := m.HostedPanelLayout(m.width, m.height).PanelRows(m.height)
	return max(rows-m.shellFrame.ExcessRows(), 1)
}

// SetShellFrameMetrics answers route.ShellFrameAware. It is not a resize: the
// panel is recomputed from the frame on every render, so the budget only has to
// be current, and routing it through SetSize would reset the input and the
// preview every time a message arrived.
func (m *TaskComposerModel) SetShellFrameMetrics(metrics chrome.ShellFrameMetrics) {
	m.shellFrame = metrics
}

// renderCompactPanel draws the bounded capture panel: what the line will do,
// the line itself, and the one thing the panel has room to say about it.
func (m *TaskComposerModel) renderCompactPanel(rows int) string {
	width := max(m.width, 1)
	var b strings.Builder
	b.WriteString(chrome.RenderLabeledDivider(m.Preset.label(), width))
	b.WriteString("\n")
	b.WriteString(m.renderComposerRouteRow(width))
	b.WriteString("\n")
	b.WriteString(m.Input.View())

	if tail := m.renderComposerPanelTail(rows-lipgloss.Height(b.String()), width); tail != "" {
		b.WriteString("\n")
		b.WriteString(tail)
	}
	return chrome.FitBlockHeight(strings.TrimRight(b.String(), "\n"), rows)
}

// renderComposerRouteRow states the effective route on one row. Every axis is
// named, because a project without a landing place and a landing place without a
// project each describe a different destination, and because the plan and the
// status a capture lands in are two answers one sigil gives at once: `$today`
// plans for Today and lands in Backlog, so a row saying only `$today` left the
// second half for the user to know rather than to read.
//
// The plan half appears only when there is a membership to name. The project
// spends whatever the named axes leave, since it is the one value that can be
// arbitrarily long and the one a truncation still leaves recognizable.
func (m *TaskComposerModel) renderComposerRouteRow(width int) string {
	project, plan, status := m.effectiveRouteHeader()
	// FieldLabelStyle rather than SubtitleStyle: the latter carries a bottom
	// margin, which on a one-row summary breaks the row in two.
	const separator = " · "
	route := theme.FieldLabelStyle.Render(chrome.StatusFieldLabel+": ") + status
	if plan != "" {
		route = theme.FieldLabelStyle.Render(chrome.PlanFieldLabel+": ") + plan + separator + route
	}
	projectLabel := theme.FieldLabelStyle.Render("Project: ")
	budget := width - lipgloss.Width(projectLabel) - lipgloss.Width(route) - lipgloss.Width(separator)
	row := projectLabel + textfit.TruncateToVisualWidth(project, max(budget, 1)) + separator + route
	return textfit.TruncateToVisualWidth(row, width)
}

// renderComposerPanelTail fills what the panel has left after its chrome.
// Refusals lead, because they are the rows a bounded panel must never drop;
// below them the panel shows the one pane the current interaction is about. The
// discard question is not among them: it is the shell's own confirm dialog,
// drawn over this panel, and a second copy underneath would cost the preview a
// row to repeat what the overlay already asks.
func (m *TaskComposerModel) renderComposerPanelTail(rows, width int) string {
	if rows <= 0 {
		return ""
	}

	var block string
	for _, lineErr := range m.Errors {
		block = joinPanelBlocks(block, theme.ValidationErrorStyle.Width(width).
			Render(fmt.Sprintf("Line %d: %s", lineErr.Line, lineErr.Message)))
	}

	remaining := rows
	if block != "" {
		remaining -= lipgloss.Height(block)
	}
	if remaining > 0 {
		block = joinPanelBlocks(block, m.renderComposerPanelPane(remaining))
	}
	return chrome.FitBlockHeight(block, rows)
}

// renderComposerPanelPane picks the one pane the panel's remaining rows go to.
// Autocomplete and the syntax guide are live interactions the keyboard is
// already inside, so each owns the space while it is open; otherwise the rows
// belong to the preview of what Enter would create.
func (m *TaskComposerModel) renderComposerPanelPane(rows int) string {
	switch {
	case m.IsAutocompleteActive():
		return strings.TrimRight(m.AC.RenderDropdownWithLimit("", rows), "\n")
	case m.HelpExpanded:
		m.SetSyntaxViewportHeight(rows)
		return m.SyntaxVP.View()
	default:
		return m.renderComposerPanelPreview(rows)
	}
}

func (m *TaskComposerModel) renderComposerPanelPreview(rows int) string {
	if count := m.nonblankLineCount(); count > 1 {
		return theme.MutedStyle.Render(fmt.Sprintf("%d tasks · shared attributes apply to every line", count))
	}
	if m.PreviewErr == "" && m.Preview == nil {
		return ""
	}
	return m.ComposeBoundedPreviewBlock(rows)
}

func joinPanelBlocks(above, below string) string {
	if above == "" {
		return below
	}
	if below == "" {
		return above
	}
	return above + "\n" + below
}
