package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestCaptureFooterSpecContainsRequiredActions pins the four capture footer
// states to their required action vocabulary. The state selection priority is
// discard confirmation → autocomplete → expanded syntax → normal capture, and
// each state must advertise its full action set so no control is hidden.
func TestCaptureFooterSpecContainsRequiredActions(t *testing.T) {
	cases := []struct {
		name               string
		confirmDiscard     bool
		autocompleteActive bool
		helpExpanded       bool
		required           []string
	}{
		{
			name:     "normal",
			required: []string{"Enter create", "Ctrl+D create & edit", "Ctrl+V paste", "? syntax", "Esc cancel"},
		},
		{
			name:               "autocomplete",
			autocompleteActive: true,
			required:           []string{"Tab/Enter accept", "Ctrl+J/K navigate", "Esc dismiss"},
		},
		{
			name:         "help",
			helpExpanded: true,
			required:     []string{"Enter create", "Ctrl+D create & edit", "Ctrl+V paste", "? close syntax", "Esc close syntax"},
		},
		{
			name:           "confirmation",
			confirmDiscard: true,
			required:       []string{"Y discard", "N/Esc keep editing"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := captureFooterSpecFull(tc.confirmDiscard, tc.autocompleteActive, tc.helpExpanded, false, 0)
			for _, want := range tc.required {
				if !strings.Contains(spec.Left, want) {
					t.Fatalf("%s footer missing %q, got %q", tc.name, want, spec.Left)
				}
			}
		})
	}
}

// TestCaptureFooterSpecStatePriority verifies the selection precedence when
// multiple state flags are set simultaneously: discard confirmation wins over
// autocomplete, which wins over expanded syntax.
func TestCaptureFooterSpecStatePriority(t *testing.T) {
	if got := captureFooterSpecFull(true, true, true, false, 0); got.Left != captureFooterConfirm {
		t.Fatalf("discard confirmation must take precedence, got %q", got.Left)
	}
	if got := captureFooterSpecFull(false, true, true, false, 0); got.Left != captureFooterAutocomplete {
		t.Fatalf("autocomplete must take precedence over help, got %q", got.Left)
	}
	if got := captureFooterSpecFull(false, false, true, false, 0); got.Left != captureFooterHelp {
		t.Fatalf("expanded syntax expected, got %q", got.Left)
	}
}

// TestCaptureFooterWrapsAt60ColumnsWithoutTruncation verifies that the full
// normal-state capture footer survives the 60-column minimum width: every
// action wraps onto additional footer rows rather than being clipped. A render
// audit confirms no required text was truncated, and all five actions remain
// present in the composed output.
func TestCaptureFooterWrapsAt60ColumnsWithoutTruncation(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.width = 60
	app.height = 20

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.activeProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}

	content, issues := withRenderAudit(func() string { return app.View().Content })
	if len(issues) != 0 {
		t.Fatalf("expected no required-text truncation at 60 columns, got %d issue(s): %+v", len(issues), issues)
	}

	plain := ansi.Strip(content)
	for _, action := range []string{"Enter create", "Ctrl+D create & edit", "Ctrl+V paste", "? syntax", "Esc cancel"} {
		if !strings.Contains(plain, action) {
			t.Fatalf("expected capture action %q at 60 columns, got:\n%s", action, plain)
		}
	}
}

// TestCaptureFooterSurvivesPinnedTasks confirms that a pinned task cannot
// replace the capture controls: the contextual footer sits above the pinned
// bar in the status-bar precedence, so with Quick Capture active the footer
// shows the capture actions rather than the pinned task strip.
func TestCaptureFooterSurvivesPinnedTasks(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.width = 80
	app.height = 24

	const sentinel = "PINNED_SENTINEL_TASK"
	project.Tasks[0].Title = sentinel
	project.Tasks[0].Pinned = true

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.activeProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}

	// Sanity: the project genuinely has a pinned task available to the bar.
	if len(app.collectPinnedTasks()) == 0 {
		t.Fatal("expected a pinned task to be collectable for the test setup")
	}

	statusBar := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(statusBar, "Enter create") {
		t.Fatalf("expected capture controls in status bar, got:\n%s", statusBar)
	}
	if strings.Contains(statusBar, sentinel) {
		t.Fatalf("pinned task must not replace capture controls, got:\n%s", statusBar)
	}
}

// TestCaptureFooterYieldsToTemporaryStatusThenReturns verifies the status-bar
// precedence at the transient boundary: a temporary app status message
// outranks the contextual footer, and once it clears the capture controls
// return.
func TestCaptureFooterYieldsToTemporaryStatusThenReturns(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.width = 80
	app.height = 24

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.activeProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}

	app.statusMessage = "task saved"
	withStatus := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(withStatus, "task saved") {
		t.Fatalf("expected temporary status message to show, got:\n%s", withStatus)
	}
	if strings.Contains(withStatus, "Enter create") {
		t.Fatalf("temporary status message must replace capture controls, got:\n%s", withStatus)
	}

	app.statusMessage = ""
	afterStatus := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(afterStatus, "Enter create") {
		t.Fatalf("expected capture controls to return after status clears, got:\n%s", afterStatus)
	}
}
