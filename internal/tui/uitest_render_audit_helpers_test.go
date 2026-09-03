package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

// This file adds render-time audit plumbing for the `tt tui-test` review
// harness. Geometry problems (invalid UTF-8, width/height overflow) are caught
// after the fact by normalizeScreen, but *required* text that a renderer clips
// to make it fit (footer help, layout chrome, indicators) leaves no geometric
// trace: the screen still fits its terminal. The render audit lets render code
// report those clips so an observation can flag them.
//
// The audit is active only while withRenderAudit runs, which UITestDriver.Observe
// uses to wrap the app render.

// requiredTextTruncatedKind is the UITestIssue.Kind emitted when a renderer had
// to clip text it was supposed to show in full.
const requiredTextTruncatedKind = "required_text_truncated"

// renderAuditSnippetMax bounds the length (in runes) of the ANSI-stripped text
// snippets embedded in an audit issue message, keeping observations compact.
const renderAuditSnippetMax = 60

// renderAudit collects required-text-truncation issues recorded during a single
// withRenderAudit call.
type renderAudit struct {
	issues []UITestIssue
}

// activeRenderAudit points at the audit collecting issues for the innermost
// in-progress withRenderAudit call, or nil when no audit is active. The UI test
// driver bypasses the tea.Program event loop and drives the app on a single
// goroutine, so a plain package variable is sufficient and avoids threading a
// collector through every render call site.
var activeRenderAudit *renderAudit

// withRenderAudit runs render within a render-audit context and returns its
// output plus any required-text-truncation issues recorded during the call.
// Nesting is supported (the previous audit is restored afterward), but only the
// innermost audit receives records; withRenderAudit is expected to wrap a single
// top-level render.
func withRenderAudit(render func() string) (string, []UITestIssue) {
	audit := &renderAudit{}
	prev := activeRenderAudit
	prevSink := textfit.RequiredTextTruncationSink
	activeRenderAudit = audit
	textfit.RequiredTextTruncationSink = func(context, full, shown string) {
		audit.issues = append(audit.issues, UITestIssue{
			Kind: requiredTextTruncatedKind,
			Line: 0,
			Message: fmt.Sprintf("%s: required text truncated: %q -> %q",
				context, renderAuditSnippet(full), renderAuditSnippet(shown)),
		})
	}
	defer func() {
		activeRenderAudit = prev
		textfit.RequiredTextTruncationSink = prevSink
	}()

	out := render()
	return out, audit.issues
}

// renderAuditSnippet strips ANSI escapes, collapses newlines to spaces, and
// clamps the result to renderAuditSnippetMax runes so audit messages stay short
// and single-line.
func renderAuditSnippet(s string) string {
	s = ansi.Strip(s)
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) > renderAuditSnippetMax {
		return string(runes[:renderAuditSnippetMax]) + "…"
	}
	return s
}
