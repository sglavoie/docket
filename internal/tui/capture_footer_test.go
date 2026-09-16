package tui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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

// TestCaptureFooterKeepsControlsBesideATemporaryStatus is the transient
// boundary: a temporary app status message takes a row of its own above the
// contextual footer rather than the footer's place, and when it clears the
// footer is exactly what it was before the message arrived.
//
// Session 4 replaced TestCaptureFooterYieldsToTemporaryStatusThenReturns here.
// That test required the displacement — a message about the last keystroke
// silencing the only statement of what the next one does, while a draft stood
// open underneath it.
//
// The equality at the end is the whole second half: the controls have to come
// back to the same row and say the same thing, or "it returns" would be
// satisfied by a footer that merely mentions them again somewhere.
func TestCaptureFooterKeepsControlsBesideATemporaryStatus(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.viewNav.width = 80
	app.viewNav.height = 24

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.taskWorkspaceProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}

	before := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(before, "Enter create") {
		t.Fatalf("the capture controls are not on screen, so the state under test was not reached:\n%s", before)
	}

	app.feedback.message = "task saved"
	withStatus := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(withStatus, "task saved") {
		t.Fatalf("expected temporary status message to show, got:\n%s", withStatus)
	}
	for _, control := range []string{"Enter create", "Esc cancel"} {
		if !strings.Contains(withStatus, control) {
			t.Fatalf("temporary status displaced %q while a draft was open, got:\n%s", control, withStatus)
		}
	}
	if got, want := lipgloss.Height(withStatus), lipgloss.Height(before)+1; got != want {
		t.Fatalf("status grew the footer by %d rows, want exactly one:\n%s", got-lipgloss.Height(before), withStatus)
	}

	app.feedback.message = ""
	if afterStatus := ansi.Strip(app.renderStatusBar()); afterStatus != before {
		t.Fatalf("the footer did not return to what it was before the status:\nbefore:\n%s\nafter:\n%s",
			before, afterStatus)
	}
}

// TestCaptureFooterKeepsControlsBesideAnError is the same boundary for the half
// that matters most: a refused save is precisely when the keys that would
// correct or abandon the draft must still be named. The error and the message
// are one channel, so the error is what the row carries when both are set.
func TestCaptureFooterKeepsControlsBesideAnError(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.viewNav.width = 60
	app.viewNav.height = 20

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.taskWorkspaceProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}

	app.feedback.message = "task saved"
	app.feedback.err = errors.New("save refused: title is required")
	footer := ansi.Strip(app.renderStatusBar())

	if !strings.Contains(footer, "save refused: title is required") {
		t.Fatalf("expected the error on the feedback row, got:\n%s", footer)
	}
	if strings.Contains(footer, "task saved") {
		t.Fatalf("the success survived beside the failure of the same act, got:\n%s", footer)
	}
	for _, control := range []string{"Enter create", "Esc cancel"} {
		if !strings.Contains(footer, control) {
			t.Fatalf("the error displaced %q, leaving the refused draft with no named way out:\n%s", control, footer)
		}
	}
}
