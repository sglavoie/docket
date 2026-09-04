package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestCaptureFooterWrapsAt60ColumnsWithoutTruncation verifies that the full
// normal-state capture footer survives the 60-column minimum width: every
// action wraps onto additional footer rows rather than being clipped. A render
// audit confirms no required text was truncated, and all five actions remain
// present in the composed output.
func TestCaptureFooterWrapsAt60ColumnsWithoutTruncation(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.viewNav.width = 60
	app.viewNav.height = 20

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.taskWorkspaceProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}

	content, issues := withRenderAudit(func() string { return app.View().Content })
	if len(issues) != 0 {
		t.Fatalf("expected no required-text truncation at 60 columns, got %d issue(s): %+v", len(issues), issues)
	}

	plain := ansi.Strip(content)
	for _, action := range []string{"Enter create", "Ctrl+E create & edit", "Ctrl+V paste", "? syntax", "Esc cancel"} {
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
	app.viewNav.width = 80
	app.viewNav.height = 24

	const sentinel = "PINNED_SENTINEL_TASK"
	project.Tasks[0].Title = sentinel
	project.Tasks[0].Pinned = true

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.taskWorkspaceProjects()))
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
	app.viewNav.width = 80
	app.viewNav.height = 24

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.taskWorkspaceProjects()))
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
