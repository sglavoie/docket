package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// Session 16.1 (FR-018) parity: the shared "peek task detail" navigation command
// (app_commands_nav.go) is exposed in Review Hub's Daily/Weekly/Activity palettes
// and Manage Hub's project/global inbox palettes. Before the repair its handler
// called getCurrentCursorTaskAndProject, whose activeReviewHub/activeManageHub
// gates are false while the palette shadows the hub, so it resolved no task and
// returned nil — which CommandPaletteExecuteMsg treats as a non-modal result and
// closes the palette without opening Peek. These end-to-end tests execute the
// registered navigation command through the palette on a Review Hub task module
// and a Manage Hub inbox module, asserting it targets the same cursor task as the
// direct Shift+Enter key and preserves the palette's query, cursor, and selection.

// TestReviewHubPaletteNavPeekResolvesCursorTask exercises the shared navigation
// Peek command while the command palette shadows the live Review Activity module.
func TestReviewHubPaletteNavPeekResolvesCursorTask(t *testing.T) {
	app := newReviewHubPeekApp(t)
	assertHubPaletteNavPeek(t, app)
}

// TestManageInboxPaletteNavPeekResolvesCursorTask exercises the same command
// while the palette shadows the live Manage Hub project inbox module.
func TestManageInboxPaletteNavPeekResolvesCursorTask(t *testing.T) {
	app := newManageInboxPeekApp(t)
	assertHubPaletteNavPeek(t, app)
}

// assertHubPaletteNavPeek drives the direct Shift+Enter key and then the shared
// palette "peek task detail" command over the same hub cursor task. The palette
// path proves three things the pre-repair command failed: it emits
// PalettePeekRequestMsg for the underlying primary task (not nil), executing it
// through CommandPaletteExecuteMsg activates the same in-app Peek overlay as the
// direct key, and the palette overlay is preserved — its query, cursor, and
// selection survive, and dismissing Peek reveals the same palette rather than
// popping back to the hub.
//
// Unlike the Task Workspace parity check, currentView() while Peek is active
// short-circuits to the hub presentation (currentPresentationView resolves the
// hub module from the Primary route), so preservation is asserted on the palette
// model's retained state plus the palette reappearing when Peek is dismissed.
func assertHubPaletteNavPeek(t *testing.T, app *AppModel) {
	t.Helper()

	// Direct Shift+Enter activates the in-app Peek overlay for the hub cursor task.
	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	if !handled || cmd != nil {
		t.Fatalf("direct Shift+Enter handled=%v cmd!=nil=%v, want peek activation", handled, cmd != nil)
	}
	if !app.peekActive || app.peekTask == nil {
		t.Fatalf("direct Shift+Enter did not activate Peek: active=%v task=%v", app.peekActive, app.peekTask)
	}
	directTaskID := app.peekTask.ID
	directIdentity := taskIdentityFor(app.peekTask, app.peekProject)
	app.dismissPeek()

	// Open a real command palette over the hub with a live query and selection so
	// the flow can prove they survive the Peek.
	hubView := app.currentView()
	palette := NewCommandPaletteModel(app.runtime, NewCommandRegistry(), hubView, nil)
	palette.searchInput.SetValue("peek")
	palette.selectedTasks[directIdentity] = true
	paletteCursor := palette.cursor
	app.showView(ViewCommandPalette, palette)
	if app.currentView() != ViewCommandPalette {
		t.Fatalf("failed to open palette over hub: view=%v", app.currentView())
	}

	// The shared navigation Peek command resolves the underlying primary task even
	// though the palette shadows the hub, and emits PalettePeekRequestMsg (the
	// shell-owned overlay message) instead of mutating peek and returning nil.
	peek := findParityCommand(t, app.navCommands(), "peek task detail")
	res := peek.Handler()
	req, ok := res.(PalettePeekRequestMsg)
	if !ok {
		t.Fatalf("palette Peek returned %T, want PalettePeekRequestMsg (in-app overlay, not nil)", res)
	}
	if req.Task == nil || req.Task.ID != directTaskID {
		t.Fatalf("palette Peek task = %v, want direct-key target %q", req.Task, directTaskID)
	}

	// Route through the execute flow: Peek must activate the same overlay while
	// keeping the palette on the stack.
	handled, _ = app.handleSearchMessages(CommandPaletteExecuteMsg{ResultMsg: res})
	if !handled {
		t.Fatal("CommandPaletteExecuteMsg carrying the palette Peek was not handled")
	}
	if !app.peekActive || app.peekTask == nil || app.peekTask.ID != directTaskID {
		t.Fatalf("palette Peek did not activate the same overlay: active=%v task=%v want=%q", app.peekActive, app.peekTask, directTaskID)
	}

	// Palette state survived beneath Peek.
	if got := palette.searchInput.Value(); got != "peek" {
		t.Fatalf("palette query = %q after Peek, want %q", got, "peek")
	}
	if !palette.selectedTasks[directIdentity] {
		t.Fatal("palette selection was cleared by Peek")
	}
	if palette.cursor != paletteCursor {
		t.Fatalf("palette cursor = %d after Peek, want %d", palette.cursor, paletteCursor)
	}
	app.dismissPeek()
	if app.currentView() != ViewCommandPalette {
		t.Fatalf("dismissing Peek did not reveal the preserved palette: view=%v", app.currentView())
	}
}

// newReviewHubPeekApp builds a typed-navigation AppModel whose active primary is
// the project-scoped Review Activity module with the cursor on a single task.
// reviewHubPrimary resolves it beneath the palette overlay.
func newReviewHubPeekApp(t *testing.T) *AppModel {
	t.Helper()
	app, project := newTestAppModelWithProject()
	app.enableTypedNavigation()

	task := &models.Task{ID: "review-peek-task", ShortID: 4242, Title: "Review peek target", WorkflowState: models.StateReady}
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()

	if cmd := app.activateReviewHub(ReviewActivity, project); cmd != nil {
		cmd()
	}
	hub, _, ok := app.reviewHubPrimary()
	if !ok {
		t.Fatal("reviewHubPrimary did not resolve the live Review Hub")
	}
	activity, ok := hub.ActiveModuleView().(*RecentActivityModel)
	if !ok {
		t.Fatalf("active Review module = %T, want *RecentActivityModel", hub.ActiveModuleView())
	}
	activity.loadTasks()
	if resolved, _ := hub.getTaskRef(); resolved == nil || resolved.ID != task.ID {
		t.Fatalf("review activity cursor = %v, want %q", resolved, task.ID)
	}
	if !app.activeReviewHub() {
		t.Fatal("review hub is not the active primary surface")
	}
	return app
}

// newManageInboxPeekApp builds a typed-navigation AppModel whose active primary is
// the Manage Hub project inbox module with one processable inbox task under the
// cursor. manageHubPrimary resolves it beneath the palette overlay.
func newManageInboxPeekApp(t *testing.T) *AppModel {
	t.Helper()
	app, project := newTestAppModelWithProject()
	app.enableTypedNavigation()

	task := &models.Task{ID: "inbox-peek-task", ShortID: 4343, Title: "Inbox peek target", WorkflowState: models.StateInbox}
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()

	handled, cmd := app.handleInboxMessages(ShowInboxProcessorMsg{Project: project})
	if !handled {
		t.Fatal("ShowInboxProcessorMsg was not handled")
	}
	if cmd != nil {
		cmd()
	}
	hub, _, ok := app.manageHubPrimary()
	if !ok {
		t.Fatal("manageHubPrimary did not resolve the live Manage Hub")
	}
	if resolved, _ := hub.getTaskRef(); resolved == nil || resolved.ID != task.ID {
		t.Fatalf("manage inbox cursor = %v, want %q", resolved, task.ID)
	}
	if !app.activeManageHub() {
		t.Fatal("manage hub is not the active primary surface")
	}
	return app
}
