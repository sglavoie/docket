package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestTruncateToWidthUsesVisualWidth(t *testing.T) {
	input := "修复界面布局  详情说明内容"
	truncated := truncateToVisualWidth(input, 10)

	if lipgloss.Width(truncated) > 10 {
		t.Fatalf("truncateToVisualWidth exceeded visual width: %q (width=%d)", truncated, lipgloss.Width(truncated))
	}
	if !strings.HasSuffix(truncated, "…") {
		t.Fatalf("expected ellipsis for truncated text, got %q", truncated)
	}
}

func TestRenderStatusBarFooterRespectsWidth(t *testing.T) {
	footer := renderStatusBarFooter(
		HelpStyle.Render("j/k: navigate · Tab: expand · Enter: open · Esc: close"),
		MutedStyle.Render("3 items"),
		24,
	)

	for line := range strings.SplitSeq(footer, "\n") {
		if lipgloss.Width(line) > 24 {
			t.Fatalf("footer line exceeds width 24: %q (width=%d)", ansi.Strip(line), lipgloss.Width(line))
		}
	}
}

// TestRenderStatusBarFooterWrapsHelpInsteadOfClipping verifies that when a
// multi-item help string is too wide for the terminal, renderStatusBarFooter
// wraps it on " · " boundaries onto extra lines rather than clipping required
// help text. The wrapped footer records no audit issue and each rendered line
// stays within the terminal width (no geometry overflow).
func TestRenderStatusBarFooterWrapsHelpInsteadOfClipping(t *testing.T) {
	const width = 48

	var footer string
	_, issues := withRenderAudit(func() string {
		footer = renderStatusBarFooter(
			HelpStyle.Render("j/k: navigate · Tab: expand · Enter: open · Esc: close"),
			MutedStyle.Render("3 items"),
			width,
		)
		return footer
	})

	if hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("expected no %q audit issue for a wrappable footer, got %+v", requiredTextTruncatedKind, issues)
	}

	// The wrapped footer is taller than one help line but never wider than the
	// terminal: reserve enough height so normalizeScreen only checks width.
	lines := strings.Split(footer, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected the footer to wrap onto extra lines, got %d line(s):\n%s", len(lines), ansi.Strip(footer))
	}
	plain, geomIssues := normalizeScreen(footer, width, len(lines))
	if len(geomIssues) > 0 {
		t.Fatalf("expected no geometry issues for the wrapped footer, got %+v\n\n%s", geomIssues, plain)
	}
}

// TestRenderStatusBarFooterRecordsAuditIssue verifies that last-resort audited
// truncation still fires when the help genuinely cannot be wrapped to fit: a
// single indivisible help item that is wider than the whole terminal must be
// clipped, and that clip is recorded as a required_text_truncated issue even
// though the resulting footer still fits geometrically.
func TestRenderStatusBarFooterRecordsAuditIssue(t *testing.T) {
	const width, height = 12, 2

	// A single item (no " · ") longer than the terminal cannot be wrapped, so it
	// falls back to audited truncation.
	oneWideItem := HelpStyle.Render("supercalifragilisticexpialidocious: run")

	var footer string
	_, issues := withRenderAudit(func() string {
		footer = renderStatusBarFooter(oneWideItem, MutedStyle.Render("x"), width)
		return footer
	})

	if !hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("expected a %q audit issue for the unwrappable footer, got %+v", requiredTextTruncatedKind, issues)
	}

	// The clip leaves no geometric trace: the footer still fits its width.
	plain, geomIssues := normalizeScreen(footer, width, height)
	if len(geomIssues) > 0 {
		t.Fatalf("expected no geometry issues for the fitted footer, got %+v\n\n%s", geomIssues, plain)
	}
}

// TestFullscreenContentHeightForBlocksBudgetsWrappedFooter verifies that the
// shared JoinVertical-based content-height helper reserves the actual rendered
// footer height. At a narrow width the
// footer help wraps beyond the historical two rows, and the joined
// title/tabBar/content/footer view must still fit its height budget rather than
// overflowing by the extra wrapped rows.
func TestFullscreenContentHeightForBlocksBudgetsWrappedFooter(t *testing.T) {
	const width, height = 40, 20

	title := renderFullscreenTitle("Example", "All Projects", nil)
	tabBar := RenderTabBar([]string{"First", "Second", "Third"}, 0, width, 1, nil)
	helpText := HelpStyle.Render("Tab tabs · j/k cursor · H/L scroll · space expand · ↵ open · r refresh · ? help")
	footer := renderStatusBarFooter("", helpText, width, nil)

	// Guard: the scenario only exercises the multi-row-footer budgeting path when
	// the footer actually wraps beyond the historical two rows.
	if got := lipgloss.Height(footer); got <= 2 {
		t.Fatalf("expected footer to wrap beyond two rows at width %d, got height %d", width, got)
	}

	contentHeight := fullscreenContentHeightForBlocks(height, title, tabBar, footer)
	view := lipgloss.JoinVertical(lipgloss.Left, title, tabBar, fitBlockHeight("", contentHeight), footer)
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("joined view height %d exceeds budget %d\n\n%s", got, height, ansi.Strip(view))
	}
}

// TestRenderStatusBarFooterConstrainsOverWideLabel verifies that when the fixed
// (non-help) side is wider than the terminal, renderStatusBarFooter constrains it
// to the available width instead of overflowing while it wraps the opposite help
// side. The over-wide label is clipped with audited truncation (so the render
// check flags it) and no rendered line exceeds the terminal width.
func TestRenderStatusBarFooterConstrainsOverWideLabel(t *testing.T) {
	const width = 40

	// A fixed label wider than the whole terminal paired with a wrappable help
	// string: the help wraps and the label must be constrained, not left full-width.
	overWideLabel := MutedStyle.Render("Absurdly Long Fixed Status Label That Exceeds The Terminal Width")
	helpText := HelpStyle.Render("j/k nav · Tab expand · Enter open · d dismiss · Esc close")

	var footer string
	_, issues := withRenderAudit(func() string {
		footer = renderStatusBarFooter(helpText, overWideLabel, width)
		return footer
	})

	if !hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("expected a %q audit issue for the over-wide fixed label, got %+v", requiredTextTruncatedKind, issues)
	}

	lines := strings.Split(footer, "\n")
	plain, geomIssues := normalizeScreen(footer, width, len(lines))
	if len(geomIssues) > 0 {
		t.Fatalf("expected no geometry issues for the constrained footer, got %+v\n\n%s", geomIssues, plain)
	}
}
