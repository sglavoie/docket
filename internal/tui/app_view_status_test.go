package tui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
)

func pickerStatusBarApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	app.viewNav.width = 100
	app.viewNav.height = 35
	app.crossOutliner.SetSize(app.viewNav.viewSize())
	return app
}

// TestOpenPickerTakesTheStatusBarAndGivesItBack is the unit-level statement of
// the ownership rule: a mounted picker is the interaction receiving input, so
// the host's hints leave the status bar, the rows they cost stay reserved, and
// unmounting the picker restores the bar byte for byte.
//
// The picker's own explainer must not arrive in their place. The bar is the
// shell's, and the picker speaks inside its own box; a suppression that let the
// overlay's text through would be the same two-sets problem with the layers
// swapped.
func TestOpenPickerTakesTheStatusBarAndGivesItBack(t *testing.T) {
	app := pickerStatusBarApp(t)

	hints := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(hints, "capture") {
		t.Fatalf("expected the Outliner hints before the picker opens, got:\n%s", hints)
	}

	app.operations.ValuePicker = newSemanticTagPickerForTest(t, []query.TagInfo{{Name: "urgent", TaskCount: 1}}, true)
	withPicker := ansi.Strip(app.renderStatusBar())
	if strings.Contains(withPicker, valuepicker.SemanticTaskTagExplainerSummary) {
		t.Fatalf("picker explainer leaked into status bar:\n%s", withPicker)
	}
	if strings.Contains(withPicker, "capture") {
		t.Fatalf("the status bar still names the host's hints while the picker owns every key:\n%s", withPicker)
	}
	if got, want := lipgloss.Height(withPicker), lipgloss.Height(hints); got != want {
		t.Fatalf("the status bar is %d rows with the picker open and %d rows without it; the space is not retained", got, want)
	}

	app.operations.ValuePicker = nil
	if restored := ansi.Strip(app.renderStatusBar()); restored != hints {
		t.Fatalf("expected the hints to return once the picker closes, got:\n%s", restored)
	}
}

func TestTransientStatusMessageStillOwnsStatusBarWhilePickerIsOpen(t *testing.T) {
	app := pickerStatusBarApp(t)
	app.operations.ValuePicker = newSemanticTagPickerForTest(t, nil, true)
	app.feedback.message = "Tagged 1 task"

	statusBar := ansi.Strip(app.renderStatusBar())
	if strings.Contains(statusBar, valuepicker.SemanticTaskTagExplainerSummary) {
		t.Fatalf("picker explainer leaked into transient status row:\n%s", statusBar)
	}
	if !strings.Contains(statusBar, "Tagged 1 task") {
		t.Fatalf("expected transient status message, got:\n%s", statusBar)
	}
}

// TestFeedbackStandsBesideGuidanceAndAChordReplacesIt is the precedence this
// footer actually keeps. Feedback and controls are two blocks rather than two
// tenants of one row, so a success or an error stands above the guidance instead
// of taking its place — the one thing that does replace the guidance is another
// instruction set, because a pending chord owns every key the ranks would name.
func TestFeedbackStandsBesideGuidanceAndAChordReplacesIt(t *testing.T) {
	app := pickerStatusBarApp(t)
	baseline := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(baseline, "a capture") {
		t.Fatalf("test requires persistent Outliner guidance, got:\n%s", baseline)
	}

	assertStandsBeside := func(name, want string) {
		t.Helper()
		status := ansi.Strip(app.renderStatusBar())
		if !strings.Contains(status, want) {
			t.Fatalf("%s is not on screen; wanted %q in:\n%s", name, want, status)
		}
		if !strings.Contains(status, "a capture") {
			t.Fatalf("%s displaced the persistent guidance:\n%s", name, status)
		}
		if got, want := lipgloss.Height(status), lipgloss.Height(baseline)+1; got != want {
			t.Fatalf("%s made the status bar %d rows, want %d — one row above the ranks:\n%s",
				name, got, want, status)
		}
	}

	app.feedback.err = errors.New("storage failed")
	assertStandsBeside("error", "storage failed")
	app.feedback.err = nil

	app.feedback.message = "task saved"
	assertStandsBeside("transient status", "task saved")
	app.feedback.message = ""

	// The chord is the exception, and only over the controls: it replaces the
	// ranks it has suspended while the message it did not cause stays put.
	app.operations.Chord = []string{"w"}
	app.feedback.message = "task saved"
	chord := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(chord, "Esc cancel") {
		t.Fatalf("the pending chord does not name its own exit:\n%s", chord)
	}
	if strings.Contains(chord, "a capture") {
		t.Fatalf("the ranks the chord suspended are still named:\n%s", chord)
	}
	if !strings.Contains(chord, "task saved") {
		t.Fatalf("the chord swallowed a message it did not cause:\n%s", chord)
	}
	app.operations.Chord = nil
	app.feedback.message = ""

	if restored := ansi.Strip(app.renderStatusBar()); restored != baseline {
		t.Fatalf("persistent guidance did not restore after shell overlays cleared:\n%s", restored)
	}
}
