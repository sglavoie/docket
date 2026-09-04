package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
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
		viewTyp presentation.PresentationID
		view    route.View
	}{
		{
			name:    "TextEditor",
			viewTyp: presentation.ViewTextEditor,
			view:    chrome.NewTextEditorModel("title", "some value"),
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
			caps, ok := tc.view.(route.ViewCapabilities)
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

	t.Run("TaskComposer", func(t *testing.T) {
		app := newTestAppModel()
		_ = app.openTaskComposer(TaskComposerPreset{
			Kind:    ComposerProject,
			Project: testutil.NewTestProjectWithTasks("test"),
		})
		if app.currentView() != presentation.ViewTaskComposer {
			t.Fatalf("expected current view %v, got %v", presentation.ViewTaskComposer, app.currentView())
		}
		if !app.taskComposer.ShouldCaptureEsc() {
			t.Fatal("TaskComposer.ShouldCaptureEsc() returned false")
		}
		handled, _ := app.handleEscKey()
		if handled {
			t.Error("TaskComposer: handleEscKey() returned handled=true; expected false")
		}
	})
}

// TestHandleEscKey_PopsNonModalViews verifies that for views without modal
// capture, handleEscKey() consumes the key and pops the view.
func TestHandleEscKey_PopsNonModalViews(t *testing.T) {
	app := newTestAppModel()
	app.pushView(presentation.ViewHelp)

	if app.currentView() != presentation.ViewHelp {
		t.Fatal("expected ViewHelp to be current")
	}

	handled, _ := app.handleEscKey()
	if !handled {
		t.Error("handleEscKey() should have consumed esc for a non-modal view")
	}
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Errorf("expected Outliner root after pop, got %v", app.currentView())
	}
}

// TestHandleEscKey_NopAtOutlinerRoot verifies that esc at the Outliner root
// is not consumed (nothing to pop).
func TestHandleEscKey_NopAtOutlinerRoot(t *testing.T) {
	app := newTestAppModel()

	handled, _ := app.handleEscKey()
	if handled {
		t.Error("handleEscKey() at the Outliner root should return handled=false")
	}
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Errorf("currentView should remain at the Outliner root, got %v", app.currentView())
	}
}

func TestParentSelectorEscClearsQueryBeforeClosing(t *testing.T) {
	parentA, err := models.NewGenericTask("parent-a", 1, "Alpha parent")
	if err != nil {
		t.Fatalf("NewTask A: %v", err)
	}
	parentB, err := models.NewGenericTask("parent-b", 2, "Beta parent")
	if err != nil {
		t.Fatalf("NewTask B: %v", err)
	}

	app := newTestAppModel()
	selector := listsurface.NewParentSelectorModel([]*models.Task{parentA, parentB}, nil, true, presentation.ViewCrossProjectOutliner)
	app.showView(presentation.ViewParentSelector, selector)
	selector.Init()

	app.Update(syntheticKey('b'))
	if selector.SearchQuery() != "b" {
		t.Fatalf("expected search query %q, got %q", "b", selector.SearchQuery())
	}

	_, cmd := app.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd != nil {
		t.Fatal("first esc with a query should clear search, not close the selector")
	}
	if app.currentView() != presentation.ViewParentSelector {
		t.Fatalf("expected selector to remain open after first esc, got %v", app.currentView())
	}
	if selector.SearchQuery() != "" {
		t.Fatalf("expected cleared search query, got %q", selector.SearchQuery())
	}
	if len(selector.FilteredItems()) != 3 {
		t.Fatalf("expected no-parent plus full parent list, got %d item(s)", len(selector.FilteredItems()))
	}

	_, cmd = app.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd != nil {
		app.Update(cmd())
	}
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatalf("expected selector to close on second esc, got %v", app.currentView())
	}
}
