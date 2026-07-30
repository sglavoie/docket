package tui

import (
	"fmt"
	"image/color"
)

// Capture footer help strings. Each is a plain, unstyled " · "-separated help
// cluster so the shared wrapping renderer can break it on item boundaries at
// narrow widths without clipping required actions. captureFooterSpec selects
// among them per interaction state.
const (
	captureFooterNormal       = "Enter create · Ctrl+O create & open · Ctrl+D create & edit · Ctrl+V paste · ? syntax · Esc cancel"
	captureFooterHelp         = "Enter create · Ctrl+O create & open · Ctrl+D create & edit · Ctrl+V paste · ? close syntax · Esc close syntax"
	captureFooterHelpScroll   = "Enter create · Ctrl+O create & open · Ctrl+D create & edit · Ctrl+V paste · Ctrl+J/K scroll · PgUp/PgDn page · ? close syntax · Esc close syntax"
	captureFooterAutocomplete = "Tab/Enter accept · Ctrl+J/K navigate · Esc dismiss"
	captureFooterConfirm      = "Y discard · N/Esc keep editing"
)

// captureFooterSpec selects the contextual footer for a capture view (Quick
// Capture and the Linked Task Creator) from its current interaction state. The
// priority mirrors the views' input precedence: discard confirmation →
// autocomplete → expanded syntax → normal capture. This base variant assumes the
// expanded syntax guide does not overflow; captureFooterSpecFull augments the
// expanded-syntax state with scroll hints and a scroll percentage once overflow
// is measurable against the footer-aware viewport budget.
func captureFooterSpec(confirmDiscard, autocompleteActive, helpExpanded bool) StatusBarFooterSpec {
	return captureFooterSpecFull(confirmDiscard, autocompleteActive, helpExpanded, false, 0)
}

// captureFooterSpecFull selects the capture footer including the expanded-syntax
// scroll affordances. When the guide is expanded, not shadowed by autocomplete
// or a discard confirmation, and its viewport overflows, the footer advertises
// the scroll/page keys and right-aligns the scroll percentage. Otherwise it
// matches captureFooterSpec.
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

// FooterEscapeHints is the canonical escape-hatch hint cluster shown in
// full-height view footers. Esc unwinds one layer and ? opens per-view help.
// The order matches the app status bar's hint cluster (see app_view_status.go)
// so the escape hatches read identically whether the host view is full-height
// or relies on the app status bar. The cluster is rendered after the
// view-specific contextual hints so the footer reads as
// "<top 3 contextual> · <escape hatches>".
const FooterEscapeHints = "esc back · ? help"
