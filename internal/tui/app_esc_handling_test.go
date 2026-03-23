package tui

import (
	"testing"
)

// TestHandleEscKey_PropagatesForModalViews verifies that views implementing
// ViewCapabilities.ShouldCaptureEsc() correctly cause handleEscKey() to return
// (false, nil) — meaning esc is NOT consumed by the app shell and is passed
// through to the view for internal handling.
//
// This test acts as a guard: any new modal view that needs to handle esc
// internally MUST implement ViewCapabilities with ShouldCaptureEsc() returning
// true, otherwise the app shell will pop the view instead of letting it handle
// the key.
func TestHandleEscKey_PropagatesForModalViews(t *testing.T) {
	alwaysCaptureViews := []struct {
		name    string
		viewTyp ViewType
		view    View
	}{
		{
			name:    "RecurrenceEditor",
			viewTyp: ViewRecurrenceEditor,
			view:    NewRecurrenceEditorModel(nil, nil),
		},
		{
			name:    "TextEditor",
			viewTyp: ViewTextEditor,
			view:    NewTextEditorModel("title", "some value"),
		},
	}

	for _, tc := range alwaysCaptureViews {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppModel()
			app.showView(tc.viewTyp, tc.view)

			if app.currentView() != tc.viewTyp {
				t.Fatalf("expected current view %v, got %v", tc.viewTyp, app.currentView())
			}

			// Verify the view reports ShouldCaptureEsc() == true
			caps, ok := tc.view.(ViewCapabilities)
			if !ok {
				t.Fatalf("%s does not implement ViewCapabilities", tc.name)
			}
			if !caps.ShouldCaptureEsc() {
				t.Fatalf("%s.ShouldCaptureEsc() returned false; modal views must return true", tc.name)
			}

			// handleEscKey must NOT consume the key (return false) so the view
			// can handle it internally (e.g. cancel an input, dismiss a sub-panel).
			handled, _ := app.handleEscKey()
			if handled {
				t.Errorf("%s: handleEscKey() returned handled=true; expected false so esc propagates to view", tc.name)
			}
		})
	}
}

// TestHandleEscKey_PopsNonModalViews verifies that for views without modal
// capture, handleEscKey() consumes the key and pops the view.
func TestHandleEscKey_PopsNonModalViews(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewHelp)

	if app.currentView() != ViewHelp {
		t.Fatal("expected ViewHelp to be current")
	}

	handled, _ := app.handleEscKey()
	if !handled {
		t.Error("handleEscKey() should have consumed esc for a non-modal view")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("expected ViewDashboard after pop, got %v", app.currentView())
	}
}

// TestHandleEscKey_NopAtDashboard verifies that esc at the dashboard root
// is not consumed (nothing to pop).
func TestHandleEscKey_NopAtDashboard(t *testing.T) {
	app := newTestAppModel()

	handled, _ := app.handleEscKey()
	if handled {
		t.Error("handleEscKey() at dashboard should return handled=false (nothing to pop)")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should remain ViewDashboard, got %v", app.currentView())
	}
}
