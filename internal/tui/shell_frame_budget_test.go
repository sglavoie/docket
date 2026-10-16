package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

func TestHelpFooterRemainsAtBottomWhileFilteringAndResizing(t *testing.T) {
	d := polishDriver(t)
	uxClarityPress(t, d, uxClarityKeys("?", "/")...)
	if err := d.runAction(UITestAction{Type: UITestActionText, Text: "task"}); err != nil {
		t.Fatal(err)
	}
	for _, size := range append(uxClarityMatrixSizes, [2]int{60, 20}) {
		d.dispatch(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		d.drain()
		frame := shellFrameFooterLines(d.App().View().Content)
		if len(frame) != size[1] || strings.TrimSpace(frame[len(frame)-1]) == "" {
			t.Fatalf("filtered Help footer floats after resize to %v:\n%s", size, strings.Join(frame, "\n"))
		}
	}
}

// These are session 4's contracts over the composed frame: the footer says what
// just happened and what the keys do, both at once, and every row it spends
// beyond the two a view was sized against is measured once and charged once.
//
// They are geometry rather than snapshots. The claim is never "the screen looks
// like this" but "these rows add up to the terminal", which is the claim a
// double reservation, a clipped footer, or a compensating guess each breaks in a
// way that names itself.

// shellFrameFooterLines is the footer split into rows, with the trailing fill
// each row is padded with removed. Comparisons below are about which row carries
// what, so the padding would only make a mismatch unreadable.
func shellFrameFooterLines(footer string) []string {
	lines := strings.Split(ansi.Strip(footer), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " ")
	}
	return lines
}

// TestShellFrameMetricsAreAPureMeasurement is the constraint the whole delivery
// rests on: the shell measures its footer before the frame is drawn, and the
// measurement is taken again — by the render — a moment later. A measurement
// that consumed anything would leave the second reading looking at a different
// footer than the first one reserved rows for.
//
// The Outliner's focus notice is the one-shot this is really about. It is
// consumed by ViewStatusStrip, which is view content and deliberately outside
// the footer measurement, so it has to survive being measured around.
func TestShellFrameMetricsAreAPureMeasurement(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureUXAcceptance, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	app := driver.App()
	view := app.crossOutliner
	if view == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	const notice = "FOCUS_NOTICE_SENTINEL"
	view.Focus.Notice = notice
	app.feedback.message = "Copied title to clipboard"

	first := app.shellFrameMetrics()
	if view.Focus.Notice != notice {
		t.Fatal("measuring the footer consumed the one-shot focus notice")
	}
	second := app.shellFrameMetrics()
	if first != second {
		t.Fatalf("two readings of one frame disagree: %+v then %+v", first, second)
	}
	if view.Focus.Notice != notice {
		t.Fatal("the second measurement consumed the one-shot focus notice")
	}
	if app.feedback.message == "" {
		t.Fatal("measuring the footer cleared the status it was measuring")
	}

	// And the render still gets it, which is what proves the notice was owed to
	// the strip rather than merely left lying around.
	if strip := ansi.Strip(view.ViewStatusStrip(100)); !strings.Contains(strip, notice) {
		t.Fatalf("the strip did not receive the notice the measurement preserved:\n%s", strip)
	}
}

// TestShellFrameMetricsMatchTheFooterThatGetsDrawn holds the measurement to the
// paint. They read the same two blocks, so a footer whose height the shell
// reported wrongly is a view reserving rows for a footer that does not exist.
func TestShellFrameMetricsMatchTheFooterThatGetsDrawn(t *testing.T) {
	for _, size := range uxClarityMatrixSizes {
		t.Run(uxClaritySizeName(size), func(t *testing.T) {
			for _, test := range []struct {
				name  string
				stage func(app *AppModel)
			}{
				{name: "resting", stage: func(*AppModel) {}},
				{name: "status", stage: func(app *AppModel) { app.feedback.message = "Copied title to clipboard" }},
				{name: "error", stage: func(app *AppModel) { app.feedback.err = errors.New("save refused") }},
			} {
				t.Run(test.name, func(t *testing.T) {
					driver, _ := uxClarityStagedDriver(t, "feedback-over-guidance", size[0], size[1])
					app := driver.App()
					app.feedback.message = ""
					app.feedback.err = nil
					test.stage(app)

					metrics := app.shellFrameMetrics()
					if got, want := lipgloss.Height(app.renderFooterChrome()), metrics.FooterRows(); got != want {
						t.Fatalf("the footer drew %d rows and measured %d:\n%s",
							got, want, ansi.Strip(app.renderFooterChrome()))
					}
				})
			}
		})
	}
}

// TestShellFrameBudgetIsChargedExactlyOnce is the double-reservation contract.
// A footer taller than the baseline costs the content area exactly that
// difference: charged twice and the frame ends in a blank band above the footer,
// charged not at all and the footer is clipped off the bottom of the terminal.
//
// It is read at every size in the matrix with capture open over the outline,
// which is the deepest live stack — a bounded panel, its host underneath, and a
// status standing above the panel's own controls — and therefore the one with
// the most places to charge a row twice.
func TestShellFrameBudgetIsChargedExactlyOnce(t *testing.T) {
	for _, size := range uxClarityMatrixSizes {
		t.Run(uxClaritySizeName(size), func(t *testing.T) {
			driver, _ := uxClarityStagedDriver(t, "feedback-over-guidance", size[0], size[1])
			app := driver.App()

			metrics := app.shellFrameMetrics()
			if metrics.ExcessRows() == 0 {
				t.Fatalf("the staged footer costs no more than the baseline (%+v), so this size proves nothing", metrics)
			}

			content := app.View().Content
			if got := lipgloss.Height(content); got != size[1] {
				t.Fatalf("the composed frame is %d rows in a %d-row terminal", got, size[1])
			}

			// The footer is the frame's last rows, in the same order and with the
			// same text. That is the whole claim: nothing was inserted below it and
			// nothing of it was cut off.
			frame := shellFrameFooterLines(content)
			footer := shellFrameFooterLines(app.renderStatusBar())
			tail := frame[len(frame)-len(footer):]
			for index := range footer {
				if tail[index] != footer[index] {
					t.Fatalf("frame row %d is %q, want the footer's %q\nframe tail:\n%s\nfooter:\n%s",
						len(frame)-len(footer)+index, tail[index], footer[index],
						strings.Join(tail, "\n"), strings.Join(footer, "\n"))
				}
			}

			if obs := driver.Observe(ObserveOptions{}); len(obs.LayoutIssues) != 0 {
				t.Fatalf("composed frame has layout issues: %+v", obs.LayoutIssues)
			}
		})
	}
}

// TestShellFrameBudgetReachesOnlyTheActiveView states who pays. A bounded panel
// and the host drawn under it are both on screen, but only one of them is
// reserving rows for the footer: the panel is the active view and the host keeps
// the budget its layout promised it. Telling both is the double reservation this
// separation exists to make impossible.
func TestShellFrameBudgetReachesOnlyTheActiveView(t *testing.T) {
	driver, _ := uxClarityStagedDriver(t, "feedback-over-guidance", 100, 35)
	app := driver.App()

	if got := app.currentPresentationView(); got != presentation.ViewTaskComposer {
		t.Fatalf("the active presentation is %v, so the state under test was not reached", got)
	}
	if !app.shellFrame.active || app.shellFrame.view != presentation.ViewTaskComposer {
		t.Fatalf("the budget went to %+v, want the composer", app.shellFrame)
	}
	if app.shellFrame.metrics.ExcessRows() == 0 {
		t.Fatalf("the staged footer costs nothing extra (%+v), so this proves nothing", app.shellFrame.metrics)
	}
	if got := app.crossOutliner.ShellFrame; got != (chrome.ShellFrameMetrics{}) {
		t.Fatalf("the host under the panel is also reserving footer rows: %+v", got)
	}

	// Closing the panel puts the budget down. The surface underneath is the one
	// drawing under the footer now, so it is the one that may be told about it.
	// The close rides a command rather than the key press, so the press has to be
	// applied rather than merely dispatched.
	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	app = driver.App()
	if app.currentPresentationView() == presentation.ViewTaskComposer {
		t.Fatal("the composer is still open after esc")
	}
	if app.shellFrame.active && app.shellFrame.view == presentation.ViewTaskComposer {
		t.Fatalf("the closed panel still holds the budget: %+v", app.shellFrame)
	}
}

// TestShellFrameFeedbackReachesASurfaceOwnedFooter covers the other half of the
// composition. Help draws its own footer and skips the shell's, so the shell has
// to put feedback into that frame without taking the footer away — and without
// making the frame taller than the terminal.
func TestShellFrameFeedbackReachesASurfaceOwnedFooter(t *testing.T) {
	for _, size := range uxClarityMatrixSizes {
		t.Run(uxClaritySizeName(size), func(t *testing.T) {
			driver, workspace, err := NewFixtureDriver(FixtureUXAcceptance, size[0], size[1])
			if err != nil {
				t.Fatalf("fixture: %v", err)
			}
			t.Cleanup(func() { _ = workspace.Cleanup() })
			uxClarityPress(t, driver, uxClarityKeys("?")...)
			app := driver.App()
			view := app.activeView()
			caps, ok := view.(route.ViewCapabilities)
			if !ok || !caps.IsFullHeight() {
				t.Fatalf("the help viewer is not the full-height view under test: %T", view)
			}

			before := ansi.Strip(app.View().Content)
			if got := lipgloss.Height(before); got != size[1] {
				t.Fatalf("Help uses %d rows in a %d-row terminal", got, size[1])
			}
			if tail := strings.Split(before, "\n"); strings.TrimSpace(tail[len(tail)-1]) == "" {
				t.Fatal("Help leaves blank rows below its footer")
			}
			app.feedback.message = "Copied title to clipboard"
			after := ansi.Strip(app.View().Content)

			if !strings.Contains(after, "Copied title to clipboard") {
				t.Fatalf("the shell's feedback never reached the surface-owned frame:\n%s", after)
			}
			if got, want := lipgloss.Height(after), lipgloss.Height(before); got != want {
				t.Fatalf("feedback changed the frame height: %d rows, want %d", got, want)
			}

			// The footer the surface published is still there, whole: its last row
			// is unchanged and the feedback sits above its divider rather than in
			// its place.
			beforeLines, afterLines := shellFrameFooterLines(before), shellFrameFooterLines(after)
			if beforeLines[len(beforeLines)-1] != afterLines[len(afterLines)-1] {
				t.Fatalf("feedback displaced the surface's own footer row:\nbefore: %q\nafter:  %q",
					beforeLines[len(beforeLines)-1], afterLines[len(afterLines)-1])
			}
			divider := surfaceFooterDividerIndex(afterLines)
			if divider < 1 {
				t.Fatalf("the composed frame lost its footer divider:\n%s", after)
			}
			if !strings.Contains(afterLines[divider-1], "Copied title to clipboard") {
				t.Fatalf("feedback is not the row directly above the surface's footer:\n%s",
					strings.Join(afterLines[max(divider-3, 0):], "\n"))
			}
		})
	}
}

// TestShellFrameFailedSaveKeepsTheDraftAndItsControls is the review's named
// recovery case, at the floor where there is least room for it. A refused save
// is exactly when the draft must not be lost and the keys that would fix or
// abandon it must still be named — and it is exactly when the footer is tallest,
// because the error is standing on a row of its own.
func TestShellFrameFailedSaveKeepsTheDraftAndItsControls(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.viewNav.width = 60
	app.viewNav.height = 20

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.taskWorkspaceProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}
	const draft = "DRAFT_SENTINEL"
	app.taskComposer.Input.SetValue(draft)
	app.syncShellFrameMetrics()

	app.feedback.err = errors.New("save refused: title is required")
	app.syncShellFrameMetrics()

	content, issues := withRenderAudit(func() string { return app.View().Content })
	if len(issues) != 0 {
		t.Fatalf("a refused save at 60x20 clipped required text: %+v", issues)
	}
	plain := ansi.Strip(content)

	if !strings.Contains(plain, draft) {
		t.Fatalf("the refusal cost the draft:\n%s", plain)
	}
	if !strings.Contains(plain, "Save refused: title is required") {
		t.Fatalf("the refusal is not on screen:\n%s", plain)
	}
	// "Esc discard" rather than "Esc cancel": the draft the refusal preserved is
	// exactly what makes chrome.DraftEscapeHint spell the destructive word, so the
	// hint naming the way out is also the evidence the way back in survived.
	for _, control := range []string{"Enter create", "Esc discard"} {
		if !strings.Contains(plain, control) {
			t.Fatalf("the refusal left %q unnamed, so the draft has no named way out:\n%s", control, plain)
		}
	}
	if got := lipgloss.Height(content); got != 20 {
		t.Fatalf("the refused frame is %d rows in a 20-row terminal:\n%s", got, plain)
	}
}
