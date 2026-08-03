package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Capture footer help strings. Each is a plain, unstyled " · "-separated help
// cluster so the shared wrapping renderer can break it on item boundaries at
// narrow widths without clipping required actions.
const (
	captureFooterNormal       = "Enter create · Ctrl+D create & edit · Ctrl+V paste · ? syntax · Esc cancel"
	captureFooterHelp         = "Enter create · Ctrl+D create & edit · Ctrl+V paste · ? close syntax · Esc close syntax"
	captureFooterHelpScroll   = "Enter create · Ctrl+D create & edit · Ctrl+V paste · Ctrl+J/K scroll · PgUp/PgDn page · ? close syntax · Esc close syntax"
	captureFooterAutocomplete = "Tab/Enter accept · Ctrl+J/K navigate · Esc dismiss"
	captureFooterConfirm      = "Y discard · N/Esc keep editing"
)

// captureFooterSpecFull selects the capture footer including the expanded-syntax
// scroll affordances. When the guide is expanded, not shadowed by autocomplete
// or a discard confirmation, and its viewport overflows, the footer advertises
// the scroll/page keys and right-aligns the scroll percentage. Otherwise it
// uses the normal expanded-help footer.
func captureFooterSpecFull(confirmDiscard, autocompleteActive, helpExpanded, syntaxOverflow bool, scrollPercent int) StatusBarFooterSpec {
	switch {
	case confirmDiscard:
		return StatusBarFooterSpec{Left: captureFooterConfirm}
	case autocompleteActive:
		return StatusBarFooterSpec{Left: captureFooterAutocomplete}
	case helpExpanded && syntaxOverflow:
		return StatusBarFooterSpec{Left: captureFooterHelpScroll, Right: fmt.Sprintf("%d%%", scrollPercent)}
	case helpExpanded:
		return StatusBarFooterSpec{Left: captureFooterHelp}
	default:
		return StatusBarFooterSpec{Left: captureFooterNormal}
	}
}

// renderContextualFooter renders a StatusBarFooterSpec into the shared wrapping
// footer (separator line + a left/right-aligned help row that wraps on " · "
// boundaries). HelpStyle is applied once over each non-empty side so the spec
// stays plain text at its source. It is the single renderer used by both the
// app-shell status bar and later footer-aware viewport height calculations, so
// measurement and display share identical wrapping behavior. Pass an optional
// accent color to tint the separator.
func renderContextualFooter(spec StatusBarFooterSpec, width int, accent ...color.Color) string {
	left := ""
	if spec.Left != "" {
		left = HelpStyle.Render(spec.Left)
	}
	right := ""
	if spec.Right != "" {
		right = HelpStyle.Render(spec.Right)
	}
	return renderStatusBarFooter(left, right, width, accent...)
}

// fitFooterHintLine greedily keeps complete operation metadata pairs in their
// ranked order. Once the next pair does not fit, lower-ranked pairs are omitted
// as well so a narrow terminal cannot promote a less important action.
func fitFooterHintLine(pairs []string, width int) string {
	if width <= 0 {
		width = 80
	}
	var kept []string
	used := 0
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		pairWidth := lipgloss.Width(pair)
		if pairWidth > width {
			break
		}
		nextWidth := pairWidth
		if len(kept) > 0 {
			nextWidth += lipgloss.Width(footerHelpSeparator)
		}
		if used+nextWidth > width {
			break
		}
		kept = append(kept, pair)
		used += nextWidth
	}
	return strings.Join(kept, footerHelpSeparator)
}

// fitPendingChordLines packs complete which-key entries into at most two
// physical rows. Hints retain declaration order. The cancellation entry is
// mandatory, so if the two-row budget is exhausted the latest continuation
// hints are removed until "Esc cancel" fits.
func fitPendingChordLines(prefix string, hints []ChordHint, width int) []string {
	if width <= 0 {
		width = 80
	}
	entries := make([]string, 0, len(hints)+2)
	if prefix = strings.TrimSpace(prefix); prefix != "" {
		entries = append(entries, prefix+"…")
	}
	for _, hint := range hints {
		entry := strings.TrimSpace(hint.Key + " " + hint.Label)
		if entry != "" {
			entries = append(entries, entry)
		}
	}
	const cancel = "Esc cancel"
	entries = append(entries, cancel)

	for len(entries) > 0 {
		if lines, ok := packFooterEntries(entries, width, 2); ok {
			return lines
		}
		// Keep the prefix and mandatory cancellation entry. Remove the last
		// declared continuation that prevents them fitting.
		if len(entries) <= 2 {
			return []string{fitPlainLineToWidth(strings.Join(entries, footerHelpSeparator), width)}
		}
		entries = append(entries[:len(entries)-2], cancel)
	}
	return nil
}

func packFooterEntries(entries []string, width, maxLines int) ([]string, bool) {
	if len(entries) == 0 || width <= 0 || maxLines <= 0 {
		return nil, len(entries) == 0
	}
	lines := make([]string, 0, maxLines)
	for _, entry := range entries {
		if entry == "" || lipgloss.Width(entry) > width {
			return nil, false
		}
		if len(lines) == 0 {
			lines = append(lines, entry)
			continue
		}
		candidate := lines[len(lines)-1] + footerHelpSeparator + entry
		if lipgloss.Width(candidate) <= width {
			lines[len(lines)-1] = candidate
			continue
		}
		if len(lines) == maxLines {
			return nil, false
		}
		lines = append(lines, entry)
	}
	return lines, true
}

func fitPlainLineToWidth(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}
	return truncateToVisualWidth(line, max(width, 1))
}

func renderPendingChordFooter(lines []string, width int, accent ...color.Color) string {
	if width <= 0 {
		width = 80
	}
	var separatorStyle lipgloss.Style
	if len(accent) > 0 && accent[0] != nil {
		separatorStyle = lipgloss.NewStyle().Foreground(accent[0])
	} else {
		separatorStyle = MutedStyle
	}
	styled := make([]string, 0, min(len(lines), 2))
	for _, line := range lines {
		if len(styled) == 2 {
			break
		}
		styled = append(styled, HelpStyle.Render(fitPlainLineToWidth(line, width)))
	}
	return separatorStyle.Render(strings.Repeat("─", width)) + "\n" + strings.Join(styled, "\n")
}

// FooterEscapeHints is the canonical escape-hatch hint cluster shown in
// full-height view footers. Esc unwinds one layer and ? opens per-view help.
// The order matches the app status bar's hint cluster (see app_view_status.go)
// so the escape hatches read identically whether the host view is full-height
// or relies on the app status bar. The cluster is rendered after the
// view-specific contextual hints so the footer reads as
// "<top 3 contextual> · <escape hatches>".
const FooterEscapeHints = "esc back · ? help"
