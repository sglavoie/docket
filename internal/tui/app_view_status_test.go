package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/tui/query"
)

func pickerStatusBarApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	app.viewNav.width = 100
	app.viewNav.height = 35
	app.crossOutliner.SetSize(app.viewNav.viewSize())
	return app
}

func TestOpenPickerLeavesStatusBarWithTheActiveView(t *testing.T) {
	app := pickerStatusBarApp(t)

	hints := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(hints, "capture") {
		t.Fatalf("expected the Outliner hints before the picker opens, got:\n%s", hints)
	}

	app.operations.ValuePicker = newSemanticTagPickerForTest(t, []query.TagInfo{{Name: "urgent", TaskCount: 1}}, true)
	withPicker := ansi.Strip(app.renderStatusBar())
	if strings.Contains(withPicker, "colors its title.") {
		t.Fatalf("picker explainer leaked into status bar:\n%s", withPicker)
	}
	if !strings.Contains(withPicker, "capture") {
		t.Fatalf("expected Outliner hints while picker is open, got:\n%s", withPicker)
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
	if strings.Contains(statusBar, "colors its title.") {
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
