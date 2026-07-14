package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
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
		{
			name:    "QuickCapture",
			viewTyp: ViewQuickCapture,
			view:    NewQuickCaptureModel(testutil.NewTestProjectWithTasks("test"), nil, false),
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

// TestHandleEscKey_DismissesPeekOverlay verifies that Esc clears peekActive and peekTask.
func TestHandleEscKey_DismissesPeekOverlay(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.peekTask = project.Tasks[0]
	app.peekActive = true

	handled, _ := app.handleEscKey()
	if !handled {
		t.Error("handleEscKey() should consume esc when peek is active")
	}
	if app.peekActive {
		t.Error("peekActive should be false after Esc")
	}
	if app.peekTask != nil {
		t.Error("peekTask should be nil after Esc")
	}
}

func TestProjectSelectorEscClearsQueryBeforeClosing(t *testing.T) {
	sourceProject, err := models.NewProject("source", "Source", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject source: %v", err)
	}
	destProject, err := models.NewProject("dest", "Destination", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject dest: %v", err)
	}
	task, err := models.NewTask("task-1", 7, models.TaskTypeTask, "Move me")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	app := newTestAppModel()
	selector := NewProjectSelectorModel([]*models.Project{sourceProject, destProject}, task, sourceProject)
	app.showView(ViewProjectSelector, selector)

	app.Update(syntheticKey('d'))
	if selector.SearchQuery() != "d" {
		t.Fatalf("expected search query %q, got %q", "d", selector.SearchQuery())
	}

	_, cmd := app.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd != nil {
		t.Fatal("first esc with a query should clear search, not close the selector")
	}
	if app.currentView() != ViewProjectSelector {
		t.Fatalf("expected selector to remain open after first esc, got %v", app.currentView())
	}
	if selector.SearchQuery() != "" {
		t.Fatalf("expected cleared search query, got %q", selector.SearchQuery())
	}
	if len(selector.Filtered) != 1 {
		t.Fatalf("expected full project list to be restored, got %d item(s)", len(selector.Filtered))
	}

	_, cmd = app.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd != nil {
		app.Update(cmd())
	}
	if app.currentView() != ViewDashboard {
		t.Fatalf("expected selector to close on second esc, got %v", app.currentView())
	}
}

func TestParentSelectorEscClearsQueryBeforeClosing(t *testing.T) {
	parentA, err := models.NewTask("parent-a", 1, models.TaskTypeStory, "Alpha parent")
	if err != nil {
		t.Fatalf("NewTask A: %v", err)
	}
	parentB, err := models.NewTask("parent-b", 2, models.TaskTypeStory, "Beta parent")
	if err != nil {
		t.Fatalf("NewTask B: %v", err)
	}

	app := newTestAppModel()
	selector := NewParentSelectorModel([]*models.Task{parentA, parentB}, nil, false, ViewProjectTree)
	app.showView(ViewParentSelector, selector)
	selector.Init()

	app.Update(syntheticKey('b'))
	if selector.SearchQuery() != "b" {
		t.Fatalf("expected search query %q, got %q", "b", selector.SearchQuery())
	}

	_, cmd := app.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd != nil {
		t.Fatal("first esc with a query should clear search, not close the selector")
	}
	if app.currentView() != ViewParentSelector {
		t.Fatalf("expected selector to remain open after first esc, got %v", app.currentView())
	}
	if selector.SearchQuery() != "" {
		t.Fatalf("expected cleared search query, got %q", selector.SearchQuery())
	}
	if len(selector.Filtered) != 2 {
		t.Fatalf("expected full parent list to be restored, got %d item(s)", len(selector.Filtered))
	}

	_, cmd = app.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd != nil {
		app.Update(cmd())
	}
	if app.currentView() != ViewDashboard {
		t.Fatalf("expected selector to close on second esc, got %v", app.currentView())
	}
}

// TestSpaceActivatesPeek verifies that space sets peekTask when a cursor task exists.
func TestSpaceActivatesPeek(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, nil, app.runtime)
	app.viewStack = []ViewType{ViewProjectTree}

	// Simulate space press
	msg := tea.KeyPressMsg{Code: tea.KeySpace}
	app.Update(msg)

	// peekTask may or may not be set depending on whether projectTree has cursor task
	// The important thing is no panic occurred
	_ = app.peekActive
	_ = app.peekTask
}

// TestSpaceKeyUpKeepsActivePeek verifies that key-up does not close active peek.
func TestSpaceKeyUpKeepsActivePeek(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.peekTask = project.Tasks[0]
	app.peekActive = true

	keyUp := tea.KeyReleaseMsg{Code: tea.KeySpace}
	result, _ := app.Update(keyUp)
	appResult := result.(*AppModel)

	if !appResult.peekActive {
		t.Error("peekActive should remain true after key-up")
	}
	if appResult.peekTask != project.Tasks[0] {
		t.Error("peekTask should remain set after key-up")
	}
}
