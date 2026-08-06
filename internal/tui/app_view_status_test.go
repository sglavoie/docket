package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func pickerStatusBarApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	app.width = 100
	app.height = 35
	app.crossOutliner.SetSize(app.width, app.height-2)
	return app
}

func TestOpenPickerLeavesStatusBarWithTheActiveView(t *testing.T) {
	app := pickerStatusBarApp(t)

	hints := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(hints, "capture") {
		t.Fatalf("expected the Outliner hints before the picker opens, got:\n%s", hints)
	}

	app.operationValuePicker = newSemanticTagPickerForTest(t, []TagInfo{{Name: "urgent", TaskCount: 1}}, true)
	withPicker := ansi.Strip(app.renderStatusBar())
	if strings.Contains(withPicker, "colors its title.") {
		t.Fatalf("picker explainer leaked into status bar:\n%s", withPicker)
	}
	if !strings.Contains(withPicker, "capture") {
		t.Fatalf("expected Outliner hints while picker is open, got:\n%s", withPicker)
	}

	app.operationValuePicker = nil
	if restored := ansi.Strip(app.renderStatusBar()); restored != hints {
		t.Fatalf("expected the hints to return once the picker closes, got:\n%s", restored)
	}
}

func TestTransientStatusMessageStillOwnsStatusBarWhilePickerIsOpen(t *testing.T) {
	app := pickerStatusBarApp(t)
	app.operationValuePicker = newSemanticTagPickerForTest(t, nil, true)
	app.statusMessage = "Tagged 1 task(s)"

	statusBar := ansi.Strip(app.renderStatusBar())
	if strings.Contains(statusBar, "colors its title.") {
		t.Fatalf("picker explainer leaked into transient status row:\n%s", statusBar)
	}
	if !strings.Contains(statusBar, "Tagged 1 task(s)") {
		t.Fatalf("expected transient status message, got:\n%s", statusBar)
	}
}
