package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/conflict"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operationhost"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

// The active interaction owns guidance. These are the contracts over who that
// is: the layer a key press reaches is the only layer allowed to describe keys,
// and the layers underneath keep their rows without speaking.
//
// Every case here asserts three things rather than one, because each of the
// three fails differently. The host's text is gone, so nothing unreachable is
// advertised. The height is unchanged, so nothing above the footer moved. And
// putting the interaction down restores the frame byte for byte, so suppression
// is a loan and not a write.

// outlinerHostRank is text only the Outliner's own ranked footer produces, so
// finding it under an overlay is finding the host still speaking.
const outlinerHostRank = "a capture"

func guidanceOwnershipApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	app.viewNav.width = 100
	app.viewNav.height = 35
	app.crossOutliner.SetSize(app.viewNav.viewSize())
	return app
}

// suppressionFeedbackMessage is a message standing in the footer while an
// interaction is armed over it. It is the other half of the frame the shell
// composes, and the half a suppressed occupant that shrank to nothing only shows
// up in: with nothing above them, the blanked rows and the absent ones are both
// the footer's last row, so the loss is invisible until feedback is there to
// take the row instead.
const suppressionFeedbackMessage = "Copied title to clipboard"

// assertSuppressedWithHostRows is the whole of the ownership claim for one
// mounted interaction: it takes the resting footer, arms the interaction, checks
// the three properties, and puts it back down. It reads that claim twice, once
// against a quiet footer and once against one already carrying a message, since
// only the second composes the suppressed rows with a block above them.
func assertSuppressedWithHostRows(t *testing.T, app *AppModel, name string, arm, disarm func()) {
	t.Helper()

	assertSuppressionKeepsTheFrame(t, app, name, arm, disarm)

	app.feedback.message = suppressionFeedbackMessage
	defer func() { app.feedback.message = "" }()
	assertSuppressionKeepsTheFrame(t, app, name+" over feedback", arm, disarm)
}

// assertSuppressionKeepsTheFrame is that claim for one footer state: the host's
// text is gone, the height is unchanged in both the paint and the measurement
// the frame was reserved against, and putting the interaction down restores the
// footer byte for byte.
func assertSuppressionKeepsTheFrame(t *testing.T, app *AppModel, name string, arm, disarm func()) {
	t.Helper()

	resting := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(resting, outlinerHostRank) {
		t.Fatalf("%s: the resting footer does not carry the host's ranks, so there is nothing to suppress:\n%s", name, resting)
	}

	arm()
	covered := ansi.Strip(app.renderStatusBar())
	if strings.Contains(covered, outlinerHostRank) {
		t.Fatalf("%s: the footer still names %q while the interaction owns every key:\n%s", name, outlinerHostRank, covered)
	}
	if got, want := lipgloss.Height(covered), lipgloss.Height(resting); got != want {
		t.Fatalf("%s: the footer is %d rows covered and %d rows at rest; the space is not retained:\ncovered:\n%s\nresting:\n%s",
			name, got, want, covered, resting)
	}
	if got, want := lipgloss.Height(app.renderFooterChrome()), app.shellFrameMetrics().FooterRows(); got != want {
		t.Fatalf("%s: the covered footer drew %d rows and measured %d, so the frame reserved the wrong budget", name, got, want)
	}

	disarm()
	if restored := ansi.Strip(app.renderStatusBar()); restored != resting {
		t.Fatalf("%s: putting the interaction down did not restore the host frame:\ngot:\n%s\nwant:\n%s", name, restored, resting)
	}
}

// TestShellModalsOwnGuidanceAndKeepTheHostRows covers every arm of
// shellModalMounted, which is every interaction handleActiveKeyOverlay routes a
// key to. The list is enumerated rather than sampled: an arm added to the
// dispatcher and forgotten here is exactly the regression — an overlay taking
// every key while the footer keeps advertising the host's.
func TestShellModalsOwnGuidanceAndKeepTheHostRows(t *testing.T) {
	cases := []struct {
		name string
		arm  func(*AppModel)
		put  func(*AppModel)
	}{
		{
			name: "check-in setup pass",
			arm:  func(app *AppModel) { app.checkInSetup = &checkInSetupPass{} },
			put:  func(app *AppModel) { app.checkInSetup = nil },
		},
		{
			name: "check-in editor",
			arm:  func(app *AppModel) { app.operations.CheckInEditor = &taskform.CheckInEditorModel{} },
			put:  func(app *AppModel) { app.operations.CheckInEditor = nil },
		},
		{
			name: "operation text prompt",
			arm:  func(app *AppModel) { app.operations.TextPrompt = &operationhost.OperationTextPromptModel{} },
			put:  func(app *AppModel) { app.operations.TextPrompt = nil },
		},
		{
			name: "value picker",
			arm: func(app *AppModel) {
				app.operations.ValuePicker = newSemanticTagPickerForTest(t, nil, true)
			},
			put: func(app *AppModel) { app.operations.ValuePicker = nil },
		},
		{
			name: "entity action sheet",
			arm:  func(app *AppModel) { app.operations.ActionSheet = &listsurface.ContextActionSheetModel{} },
			put:  func(app *AppModel) { app.operations.ActionSheet = nil },
		},
		{
			name: "named lens picker",
			arm:  func(app *AppModel) { app.lenses.Open(&lens.NamedLensPickerModel{}) },
			put:  func(app *AppModel) { app.lenses.Close() },
		},
		{
			name: "conflict browser",
			arm:  func(app *AppModel) { app.conflictBrowser = &conflict.ConflictBrowserModel{} },
			put:  func(app *AppModel) { app.conflictBrowser = nil },
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			app := guidanceOwnershipApp(t)
			assertSuppressedWithHostRows(t, app, testCase.name,
				func() { testCase.arm(app) },
				func() { testCase.put(app) },
			)
			testCase.arm(app)
			if owner, _ := app.activeGuidance(); owner != route.GuidanceOwnerModal {
				t.Fatalf("%s resolves to owner %s rather than modal", testCase.name, owner)
			}
		})
	}
}

// TestShellConfirmationsOwnGuidanceAndKeepTheHostRows covers the three
// shell-owned confirmations. Each names its own answers inside its box, so the
// footer underneath has nothing left to say that a key press could act on.
func TestShellConfirmationsOwnGuidanceAndKeepTheHostRows(t *testing.T) {
	cases := []struct {
		name string
		arm  func(*AppModel)
		put  func(*AppModel)
	}{
		{
			name: "project archive",
			arm: func(app *AppModel) {
				app.project.ArmArchive(&models.Project{Name: "Inbox"}, true)
			},
			put: func(app *AppModel) { app.project.ClearArchive() },
		},
		{
			name: "task delete",
			arm:  func(app *AppModel) { app.taskMutation.deleteConfirm.Show("delete", "Delete this task?") },
			put:  func(app *AppModel) { app.taskMutation.deleteConfirm.Cancel() },
		},
		{
			name: "contextual operation",
			arm:  func(app *AppModel) { app.operations.Confirm.Show("delete", "Delete 2 tasks?") },
			put:  func(app *AppModel) { app.operations.Confirm.Cancel() },
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			app := guidanceOwnershipApp(t)
			assertSuppressedWithHostRows(t, app, testCase.name,
				func() { testCase.arm(app) },
				func() { testCase.put(app) },
			)
			testCase.arm(app)
			if owner, _ := app.activeGuidance(); owner != route.GuidanceOwnerConfirmation {
				t.Fatalf("%s resolves to owner %s rather than confirmation", testCase.name, owner)
			}
		})
	}
}

// TestPendingChordOutranksTheSurfaceButNotAnOverlay places the chord where it
// belongs in the order. It is live input feedback about keys that are really
// pending, so it displaces the surface underneath it; but a mounted overlay has
// already taken the keys the chord would complete, so the which-key row would be
// describing a continuation nothing can type.
func TestPendingChordOutranksTheSurfaceButNotAnOverlay(t *testing.T) {
	app := guidanceOwnershipApp(t)
	app.operations.Chord = []string{"w"}

	owner, spec := app.activeGuidance()
	if owner != route.GuidanceOwnerPendingChord {
		t.Fatalf("a pending chord over the Outliner resolves to owner %s", owner)
	}
	if len(spec.Hints) == 0 {
		t.Fatal("the resolved chord owner carries no hints to render")
	}
	chordFooter := ansi.Strip(app.renderStatusBar())
	if strings.Contains(chordFooter, outlinerHostRank) {
		t.Fatalf("the pending chord left the host's ranks on screen:\n%s", chordFooter)
	}

	app.operations.ValuePicker = newSemanticTagPickerForTest(t, nil, true)
	if owner, _ := app.activeGuidance(); owner != route.GuidanceOwnerModal {
		t.Fatalf("a chord pending under a mounted picker resolves to owner %s rather than modal", owner)
	}
	covered := ansi.Strip(app.renderStatusBar())
	for _, hint := range spec.Hints {
		label := ansi.Strip(hint.Label)
		if label == "" {
			continue
		}
		if strings.Contains(covered, label) {
			t.Fatalf("the which-key row survived under the picker, naming %q:\n%s", label, covered)
		}
	}
}

// TestExpandedComposerSilencesTheShellFooter is the SelfGuidedPresentation case.
// One PresentationID names the composer and the composer owns two presentations:
// the compact panel the shell speaks for, and the expanded form that draws its
// own footer over the whole content area. The static policy cannot tell them
// apart, which is why the view answers per frame.
func TestExpandedComposerSilencesTheShellFooter(t *testing.T) {
	composer := &taskform.TaskComposerModel{}
	if composer.PublishesOwnGuidance() {
		t.Fatal("the compact capture panel claims to draw its own guidance; the shell publishes it")
	}
	composer.Mode = taskform.ComposerExpanded
	if !composer.PublishesOwnGuidance() {
		t.Fatal("the expanded form does not claim its own guidance, so the shell would draw a second set under it")
	}

	// Reached through the real route rather than by assigning the field, since
	// what the shell consults is the live overlay stack.
	driver, workspace, err := NewFixtureDriver(FixtureUXAcceptance, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()

	uxClarityPress(t, driver, uxClarityKeys("a")...)
	if owner, _ := app.activeGuidance(); owner != route.GuidanceOwnerShell {
		t.Fatalf("the compact capture panel resolves to owner %s; the shell is its voice", owner)
	}
	compact := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(compact, "Enter create") {
		t.Fatalf("the compact panel's controls are not in the footer, so the state under test was not reached:\n%s", compact)
	}

	uxClarityPress(t, driver, uxClarityKeys("ctrl+f")...)
	if owner, _ := app.activeGuidance(); owner != route.GuidanceOwnerPresentation {
		t.Fatalf("the expanded form resolves to owner %s rather than presentation", owner)
	}
	// The divider is chrome and stays; what must go is every word under it, so
	// the assertion is that the bar carries no label text at all rather than that
	// it is empty.
	expanded := ansi.Strip(app.renderStatusBar())
	if strings.ContainsAny(expanded, "abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("the shell footer speaks under the expanded form, which draws its own:\n%s", expanded)
	}
	if lipgloss.Height(expanded) < lipgloss.Height(compact)-1 {
		t.Fatalf("the suppressed footer gave up more than the guidance rows: %d against %d",
			lipgloss.Height(expanded), lipgloss.Height(compact))
	}
}

// TestAFullscreenSurfaceKeepsTheFooterItReplacedTheHostFor draws the boundary
// the resolver's last rung stands on, which is not "did this view draw its own
// acts" but "is anyone else still under it".
//
// A transient presentation is composited over a host that is still drawn and
// still filling the footer, so those ranks belong to a layer no press reaches. A
// fullscreen surface replaced that host: the footer beneath it holds its own
// breadcrumb and its own `Esc back · ? help`, the one statement of either on the
// screen, and blanking it would take away the only advertisement of a key the
// surface answers rather than a duplicate of one.
func TestAFullscreenSurfaceKeepsTheFooterItReplacedTheHostFor(t *testing.T) {
	app := guidanceOwnershipApp(t)
	if owner, _ := app.activeGuidance(); owner != route.GuidanceOwnerShell {
		t.Fatalf("the resting Outliner resolves to owner %s rather than shell", owner)
	}
	if !app.activePresentationPublishesToShell() {
		t.Fatal("the Outliner is fullscreen, so the footer must still be its voice")
	}

	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	live := driver.App()

	// The launch already seated the cursor on a task, so `;` is the whole gesture.
	uxClarityPress(t, driver, uxClarityKeys(";")...)
	if got := live.currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("`;` reached %s rather than the Comments panel", got)
	}
	if !live.activePresentationPublishesToShell() {
		t.Fatal("the Comments panel is fullscreen, so the shell still owes it a breadcrumb row")
	}
	if owner, _ := live.activeGuidance(); owner != route.GuidanceOwnerShell {
		t.Fatalf("the Comments panel resolves to owner %s rather than shell", owner)
	}
	frame := ansi.Strip(live.renderStatusBar())
	if !strings.Contains(frame, "? help") {
		t.Fatalf("the panel's `?` is advertised nowhere: the footer under it says\n%s", frame)
	}
	if strings.Contains(frame, outlinerHostRank) {
		t.Fatalf("the footer under the panel carries the Outliner's ranks:\n%s", frame)
	}
}

// TestViewLocalModalsKeepSoleOwnershipThroughTheirSurface covers the modals that
// never reach the shell at all: the Outliner's menu bar, its relation picker,
// and its query readout are view-local gated sub-states, so the surface itself
// is what must stop publishing the ranks underneath them. The shell's job there
// is to keep publishing whatever the surface says, which is what makes one
// resolver enough for both kinds of modal.
func TestViewLocalModalsKeepSoleOwnershipThroughTheirSurface(t *testing.T) {
	app := guidanceOwnershipApp(t)
	resting := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(resting, outlinerHostRank) {
		t.Fatalf("the resting footer does not carry the host's ranks:\n%s", resting)
	}

	app.crossOutliner.MenuBar.Open = true
	covered := ansi.Strip(app.renderStatusBar())
	if strings.Contains(covered, outlinerHostRank) {
		t.Fatalf("the open menu bar left the row-level ranks in the footer:\n%s", covered)
	}
	if strings.TrimSpace(covered) == "" {
		t.Fatalf("the open menu bar left the footer empty rather than naming its own keys:\n%q", covered)
	}
	if owner, _ := app.activeGuidance(); owner != route.GuidanceOwnerShell {
		t.Fatalf("a view-local modal resolves to owner %s; the surface is still the one publishing", owner)
	}

	app.crossOutliner.MenuBar.Open = false
	if restored := ansi.Strip(app.renderStatusBar()); restored != resting {
		t.Fatalf("closing the menu bar did not restore the footer:\ngot:\n%s\nwant:\n%s", restored, resting)
	}
}

// TestSuppressedGuidanceSpendsExactlyTheRowsItHeld is the measurement on its own,
// away from any particular overlay. The frame is laid out against the footer's
// height, so a suppressed occupant that returned fewer rows would move
// everything above it and closing the overlay would move it back.
//
// It reads the height the way the footer does as well as the way lipgloss does,
// because those two disagree about the empty string: lipgloss counts it as a
// row, while footerBlockRows and footerChromeContent read it as a block that is
// not there. A one-row occupant blanked to "" measures correctly in isolation
// and vanishes from the composition, which is the regression this second
// reading names.
func TestSuppressedGuidanceSpendsExactlyTheRowsItHeld(t *testing.T) {
	const width = 40
	for _, content := range []string{"", "one row", "two\nrows", "three\nrows\nhere"} {
		blanked := suppressGuidanceRows(content, width)
		if got, want := lipgloss.Height(blanked), lipgloss.Height(content); got != want {
			t.Fatalf("suppressing %q returned %d rows instead of %d", content, got, want)
		}
		if got, want := footerBlockRows(blanked), footerBlockRows(content); got != want {
			t.Fatalf("suppressing %q costs the composed footer %d rows instead of %d", content, got, want)
		}
		if strings.TrimSpace(blanked) != "" {
			t.Fatalf("suppressing %q left text behind: %q", content, blanked)
		}
		if content == "" {
			continue
		}
		for index, row := range strings.Split(blanked, "\n") {
			if got := lipgloss.Width(row); got != width {
				t.Fatalf("suppressing %q left row %d %d cells wide instead of the footer's %d", content, index, got, width)
			}
		}
	}
}
