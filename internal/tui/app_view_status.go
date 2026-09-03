package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

const (
	footerDividerIdleGlyph  = '─'
	footerDividerFlashGlyph = '━'
)

// renderFooterDivider turns a status change into three visible phases: a brief
// full-width impact, an eased severity sweep, and a shrinking dissolve. The
// glyph motion remains visible when the explicit --no-color flag is active.
func (m *AppModel) renderFooterDivider(width int) string {
	if width <= 0 {
		width = 80
	}
	accent := m.currentAccentColor()
	if !m.footerFlash.active() {
		return renderHorizontalDivider(width, WithDividerColor(accent))
	}

	highlight := m.footerFlash.severity.color()
	frame := min(max(m.footerFlash.frame, 0), footerFlashFrames-1)
	if frame < footerFlashImpactFrames {
		return renderHorizontalDivider(
			width,
			WithDividerColor(highlight),
			WithDividerGlyph(footerDividerFlashGlyph),
		)
	}

	segmentWidth := footerFlashSegmentWidth(width)
	if frame < footerFlashImpactFrames+footerFlashSweepFrames {
		sweepFrame := frame - footerFlashImpactFrames
		progress := float64(sweepFrame) / float64(footerFlashSweepFrames-1)
		start := int(math.Round(easeInOutCubic(progress) * float64(width-segmentWidth)))
		return renderFooterSweep(width, start, segmentWidth, accent, highlight)
	}

	settleFrame := frame - footerFlashImpactFrames - footerFlashSweepFrames
	progress := float64(settleFrame+1) / float64(footerFlashSettleFrames+1)
	eased := easeInOutCubic(progress)
	segmentWidth = max(int(math.Round(float64(segmentWidth)*(1-eased))), 1)
	start := width - segmentWidth
	highlight = blendColor(highlight, accent, eased)
	return renderFooterSweep(width, start, segmentWidth, accent, highlight)
}

func renderFooterSweep(width, start, segmentWidth int, accent, highlight color.Color) string {
	start = min(max(start, 0), width)
	segmentWidth = min(max(segmentWidth, 0), width-start)

	var divider strings.Builder
	divider.Grow(width * 4)
	writeFooterDividerRun(&divider, footerDividerIdleGlyph, start, accent)
	for index := range segmentWidth {
		headProgress := 1.0
		if segmentWidth > 1 {
			headProgress = float64(index) / float64(segmentWidth-1)
		}
		trailFade := footerFlashTrailFade * (1 - easeInOutCubic(headProgress))
		cellColor := blendColor(highlight, accent, trailFade)
		writeFooterDividerRun(&divider, footerDividerFlashGlyph, 1, cellColor)
	}
	writeFooterDividerRun(&divider, footerDividerIdleGlyph, width-start-segmentWidth, accent)
	return divider.String()
}

func writeFooterDividerRun(builder *strings.Builder, glyph rune, count int, foreground color.Color) {
	if count <= 0 {
		return
	}
	style := lipgloss.NewStyle().Foreground(foreground)
	builder.WriteString(style.Render(strings.Repeat(string(glyph), count)))
}

// renderFooterContent fills each footer content row with the active flash wash.
// Child foreground and badge styles remain intact; after a child style resets,
// the wash is restored so unstyled text and trailing cells do not develop holes.
func (m *AppModel) renderFooterContent(content string, width int) string {
	if !m.footerFlash.active() || content == "" {
		return content
	}
	if width <= 0 {
		width = 80
	}

	background := m.footerFlashBackgroundColor()
	prefix, suffix := footerBackgroundStyleParts(background)
	if prefix == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	for index, line := range lines {
		line = truncateToVisualWidth(line, width)
		if gap := width - lipgloss.Width(line); gap > 0 {
			line += strings.Repeat(" ", gap)
		}
		line = strings.ReplaceAll(line, ansi.ResetStyle, ansi.ResetStyle+prefix)
		line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+prefix)
		lines[index] = prefix + line + suffix
	}
	return strings.Join(lines, "\n")
}

func footerBackgroundStyleParts(background color.Color) (prefix, suffix string) {
	const marker = "x"
	styled := lipgloss.NewStyle().Background(background).Render(marker)
	prefix, suffix, ok := strings.Cut(styled, marker)
	if !ok || prefix == "" {
		return "", ""
	}
	return prefix, suffix
}

// renderStatusBar composes the shell-owned bottom bar: the active view's status
// strip, when it has one, sits on top of the footer chrome under a static dim
// divider. Growing the status bar is what pins the strip directly above the
// footer, since AppModel.View sizes and pads content against its height.
func (m *AppModel) renderStatusBar() string {
	footer := m.renderFooterChrome()
	provider, ok := m.activeView().(ViewStatusStripProvider)
	if !ok {
		return footer
	}
	strip := provider.ViewStatusStrip(m.width)
	if strip == "" {
		return footer
	}
	// Deliberately static: the strip is view content, so it is neither one of
	// the flash-capable dividers nor a taker of the footer content wash.
	return renderHorizontalDivider(m.width) + "\n" + strip + "\n" + footer
}

func (m *AppModel) renderFooterChrome() string {
	// Top border separator for visual distinction; tinted with current view accent.
	separator := m.renderFooterDivider(m.width)
	renderFooter := func(content string) string {
		return separator + "\n" + m.renderFooterContent(content, m.width)
	}

	// Helper to render left-aligned message with spacer.
	renderLeftAlignedMsg := func(msg string) string {
		width := m.width
		if width <= 0 {
			width = 80
		}
		msg = truncateToVisualWidth(msg, width)
		gap := max(width-lipgloss.Width(msg), 0)
		spacer := lipgloss.NewStyle().
			Width(gap).
			Background(m.footerFlashBackgroundColor()).
			Render("")
		return lipgloss.JoinHorizontal(lipgloss.Top, msg, spacer)
	}

	// Show error message if present.
	if m.err != nil {
		msg := theme.ErrorStatusStyle.Render(icons.Get("error") + " " + m.err.Error())
		return renderFooter(renderLeftAlignedMsg(msg))
	}

	// An unfinished chord owns the footer: the which-key row is live input
	// feedback, so it outranks banners and transient status text.
	if spec, ok := m.pendingChordGuidance(); ok {
		return renderFooter(renderGuidanceText(spec, m.width))
	}

	// Show temporary status message if present.
	if m.statusMessage != "" {
		var msg string
		if m.statusIsInfo {
			msg = theme.InfoStatusStyle.Render("· " + m.statusMessage)
		} else {
			msg = theme.SuccessStyle.Render(icons.Get("success") + " " + m.statusMessage)
		}
		return renderFooter(renderLeftAlignedMsg(msg))
	}

	// Show the current view's contextual guidance if it provides any (e.g. the
	// capture views). This sits above the pinned-task bar and generic breadcrumb
	// but below the transient loading/error/status overlays handled above, so a
	// temporary status message still replaces capture controls and the controls
	// return once it clears. renderGuidance emits its own separator, so
	// return it directly rather than prefixing the local one.
	if view := m.activeView(); view != nil {
		if provider, ok := view.(ContextualGuidanceProvider); ok {
			if spec, ok := provider.ContextualGuidance(); ok {
				return renderFooter(renderGuidanceText(spec, m.width))
			}
		}
	}

	// Show pinned tasks bar if any tasks are pinned.
	if pinnedBar := m.renderPinnedTasksBar(m.width); pinnedBar != "" {
		return renderFooter(pinnedBar)
	}

	status := m.buildBreadcrumb()

	// Check if view supports help shortcut hint; only show if view implements HelpProvider.
	currentView := m.currentView()
	showHelp := false
	if view := m.activeView(); view != nil {
		if _, ok := view.(HelpProvider); ok {
			showHelp = true
		}
	}

	help := ""
	if showHelp {
		if hint := pipelineNextHint(currentView); hint != "" {
			help = theme.HelpStyle.Render(hint)
		} else {
			help = strings.Join(
				renderGuidanceLines(escapeGuidanceSpec(), m.width),
				theme.HelpStyle.Render(footerHelpSeparator),
			)
		}
	}

	accent := m.currentAccentColor()
	accentBarStyle := lipgloss.NewStyle().
		Foreground(accent).
		Background(m.footerFlashBackgroundColor()).
		Padding(0, 1)

	left := accentBarStyle.Render(status)

	// Add filter pill if current view has active filters.
	if view := m.activeView(); view != nil {
		if fp, ok := view.(ActiveFilterProvider); ok {
			if count := fp.ActiveFilterCount(); count > 0 {
				filterLabel := "filters"
				if count == 1 {
					filterLabel = "filter"
				}
				left += " " + theme.FilterBadgeStyle.Render(fmt.Sprintf("%d %s", count, filterLabel))
			}
		}
	}

	// Prepend mode badge when the current view is in a named mode.
	if view := m.activeView(); view != nil {
		if mi, ok := view.(ModeIndicator); ok {
			if label, c := mi.ActiveMode(); label != "" {
				badge := lipgloss.NewStyle().
					Bold(true).
					Background(c).
					Foreground(theme.ColorBackground).
					Padding(0, 1).
					Render(label)
				left = badge + " " + left
			}
		}
	}

	right := help
	left, right = fitStatusBarSegments(left, right, m.width)
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	spacer := lipgloss.NewStyle().
		Width(gap).
		Background(m.footerFlashBackgroundColor()).
		Render("")

	return renderFooter(lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right))
}

func fitStatusBarSegments(left, right string, width int) (string, string) {
	if width <= 0 {
		width = 80
	}

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	minGap := 0
	if leftWidth > 0 && rightWidth > 0 {
		minGap = 1
	}
	if leftWidth+rightWidth+minGap > width {
		availableLeft := max(width-rightWidth-minGap, 1)
		left = truncateToVisualWidth(left, availableLeft)
		leftWidth = lipgloss.Width(left)
		if leftWidth+rightWidth+minGap > width {
			right = truncateToVisualWidth(right, max(width-leftWidth-minGap, 1))
		}
	}

	return left, right
}

// buildBreadcrumb constructs a compact route/overlay breadcrumb.
// Each segment uses the view's StatusBarLabel() when available, with fallbacks for common views.
// Project names are truncated to keep the breadcrumb short.
func (m *AppModel) buildBreadcrumb() string {
	const maxProjectLen = 16
	const sep = " > "

	path := m.presentationPath()

	// Build a deduplicated renderer path, keeping the LAST occurrence of each
	// view type (e.g. for stacked project views the deepest project wins).
	lastIdx := make(map[presentation.PresentationID]int)
	for i, vt := range path {
		lastIdx[vt] = i
	}

	var parts []string
	for i, vt := range path {
		if lastIdx[vt] != i {
			continue // skip earlier duplicates
		}
		label := m.breadcrumbLabel(vt, maxProjectLen)
		if label != "" {
			parts = append(parts, label)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	// Deep stacks: abbreviate to last 2 segments with a depth indicator.
	if len(parts) > 3 {
		depth := len(parts)
		abbreviated := "… > " + strings.Join(parts[len(parts)-2:], sep)
		return abbreviated + " " + theme.MutedStyle.Render(fmt.Sprintf("[%d]", depth))
	}

	crumb := strings.Join(parts, sep)
	// Truncate total breadcrumb if it would crowd out the help text.
	maxLen := max(m.width/2, 20)
	if lipgloss.Width(crumb) > maxLen {
		crumb = truncateToVisualWidth(crumb, maxLen)
	}

	// Esc-preview: show what pressing Esc will return to. The Outliner is the
	// sole root, so suppress the hint there.
	atRoot := m.navigation.Primary.Depth() <= 1 && !m.navigation.Overlays.Active()
	if len(path) >= 2 && !atRoot {
		prevVT := path[len(path)-2]
		prevLabel := m.breadcrumbLabel(prevVT, maxProjectLen)
		if prevLabel != "" {
			hint := theme.MutedStyle.Render(" ← " + prevLabel)
			if lipgloss.Width(crumb)+lipgloss.Width(hint) <= maxLen {
				crumb += hint
			}
		}
	}

	return crumb
}

// breadcrumbLabel returns a short human-readable label for a single view type.
func (m *AppModel) breadcrumbLabel(vt presentation.PresentationID, maxProjectLen int) string {
	path := m.presentationPath()
	var label string
	if m.activeManageHub() && len(path) > 0 && vt == path[len(path)-1] {
		label = m.manageHub.StatusBarLabel()
	}

	// Use the registered view's StatusBarLabel when available.
	if label == "" {
		if view := m.views.Get(vt); view != nil {
			if labeler, ok := view.(StatusBarLabeler); ok {
				raw := labeler.StatusBarLabel()
				// Strip "Project: " prefix from project tree label for compactness.
				label = strings.TrimPrefix(raw, "Project: ")
				// Also strip cycle-position suffix (starts with " [") for compactness.
				if idx := strings.Index(label, " ["); idx > 0 {
					label = label[:idx]
				}
				if lipgloss.Width(label) > maxProjectLen {
					label = truncateToVisualWidth(label, maxProjectLen)
				}
			}
		}
	}

	if label == "" {
		return ""
	}

	// Prepend pipeline stage when the view belongs to a named stage.
	if stage := pipelineStage(vt); stage != "" {
		return stage + ": " + label
	}
	return label
}
