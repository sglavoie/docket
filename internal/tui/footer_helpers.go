package tui

// FooterEscapeHints is the canonical escape-hatch hint cluster shown in
// full-height view footers. Session 11 fixes these three keys as the stable
// escape hatches across every primary daily-driver surface: ;; jumps back to
// the pipeline hub, : opens the command palette, ? opens the per-view help.
// The order matches the app status bar's hint cluster (see app_view_status.go)
// so the escape hatches read identically whether the host view is full-height
// or relies on the app status bar. The cluster is rendered after the
// view-specific contextual hints so the footer reads as
// "<top 3 contextual> · <escape hatches>".
const FooterEscapeHints = ";; hub · : cmd · ? help"

// composeFullHeightFooterLeft returns the styled left side of a full-height
// view footer: contextual hints concatenated with the canonical escape-hatch
// cluster. The contextual argument should be the plain unstyled string
// (e.g., "x done · a add · enter open"); pass an empty string to render only
// the escape hatches. The renderer (HelpStyle) is applied once over the
// joined string so the cluster reads as a single help line.
func composeFullHeightFooterLeft(contextual string) string {
	if contextual == "" {
		return HelpStyle.Render(FooterEscapeHints)
	}
	return HelpStyle.Render(contextual + " · " + FooterEscapeHints)
}
