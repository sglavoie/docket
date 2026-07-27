package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func truncateToVisualWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width < 2 {
		return ansi.Truncate(text, width, "")
	}
	return ansi.Truncate(text, width-1, "") + "…"
}

func fitStyledLineToWidth(line string, width int) string {
	if width <= 0 || lipgloss.Width(line) <= width {
		return line
	}
	return ansi.Truncate(line, width, "")
}

// fitStyledChromeToWidth is the required-auditing counterpart to
// fitStyledLineToWidth for layout chrome (fullscreen titles, picker/header
// chrome) that is meant to stay fully visible. It returns the line unchanged
// when it already fits, but when a clip is unavoidable it routes through
// truncateRequired so the render audit flags chrome the layout could not fit.
// site names the render site (e.g. "fullscreen title"). Use fitStyledLineToWidth
// for dense data lines whose clipping is acceptable.
func fitStyledChromeToWidth(site, line string, width int) string {
	if width <= 0 || lipgloss.Width(line) <= width {
		return line
	}
	return truncateRequired(site, line, width)
}

// truncateIntentional performs ANSI-aware truncation for dense data where
// clipping to fit is the intended, acceptable behavior: list rows, cards,
// previews, and scrollable content that legitimately cannot always show every
// character. It records NO render-audit issue — dense data is allowed to
// truncate. tail is appended when the text is clipped (pass "…" for an ellipsis
// or "" for a hard clip). This is the default policy shared by the existing data
// call sites and is the intentional counterpart to truncateRequired.
//
// truncateIntentional and truncateRequired are the only sanctioned wrappers over
// raw ansi.Truncate outside this file; TestNoRawANSITruncate enforces that.
func truncateIntentional(text string, width int, tail string) string {
	return ansi.Truncate(text, width, tail)
}

// truncateRequired performs ANSI-aware truncation for layout chrome, footer
// help, and indicators that are required to stay fully visible. When a clip
// actually occurs it records a required-text-truncation audit issue against
// site (e.g. "status footer left") so the `tt tui-test` render audit can flag
// chrome the layout failed to fit; when the text already fits it is a no-op.
// Use "" as the truncation tail so audited chrome never spends a column on an
// ellipsis. Returns the (possibly clipped) text.
func truncateRequired(site, text string, width int) string {
	truncated := ansi.Truncate(text, width, "")
	if truncated != text {
		recordRequiredTextTruncation(site, text, truncated)
	}
	return truncated
}
