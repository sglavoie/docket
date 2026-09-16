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
	app.feedback.message = "Tagged 1 task(s)"

	statusBar := ansi.Strip(app.renderStatusBar())
	if strings.Contains(statusBar, valuepicker.SemanticTaskTagExplainerSummary) {
		t.Fatalf("picker explainer leaked into transient status row:\n%s", statusBar)
	}
	if !strings.Contains(statusBar, "Tagged 1 task(s)") {
		t.Fatalf("expected transient status message, got:\n%s", statusBar)
	}
}

func TestGuidancePreservesShellPrecedenceAndRestores(t *testing.T) {
	app := pickerStatusBarApp(t)
	baseline := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(baseline, "a capture") {
		t.Fatalf("test requires persistent Outliner guidance, got:\n%s", baseline)
	}

	assertOwnsStatus := func(name, want string) {
		t.Helper()
		status := ansi.Strip(app.renderStatusBar())
		if !strings.Contains(status, want) {
			t.Fatalf("%s did not own status bar; wanted %q in:\n%s", name, want, status)
		}
		if strings.Contains(status, "a capture") {
			t.Fatalf("%s leaked persistent guidance:\n%s", name, status)
		}
	}

	app.feedback.err = errors.New("storage failed")
	assertOwnsStatus("error", "storage failed")
	app.feedback.err = nil

	app.feedback.message = "task saved"
	assertOwnsStatus("transient status", "task saved")
	app.feedback.message = ""

	app.operations.Chord = []string{"w"}
	app.feedback.message = "hidden status"
	assertOwnsStatus("pending chord", "Esc cancel")
	app.operations.Chord = nil
	app.feedback.message = ""

	if restored := ansi.Strip(app.renderStatusBar()); restored != baseline {
		t.Fatalf("persistent guidance did not restore after shell overlays cleared:\n%s", restored)
	}
}
