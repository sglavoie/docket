package tui

import (
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestHandleTagMessagesShowTagManager(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	msg := ShowTagManagerMsg{ProjectTree: app.projectTree}
	handled, _ := app.handleTagMessages(msg)

	if !handled {
		t.Error("ShowTagManagerMsg should be handled")
	}
	if app.currentView() != ViewTagManager {
		t.Errorf("currentView should be TagManager, got %v", app.currentView())
	}
}

func TestHandleTagMessagesCloseTagManager(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTagManager)

	app.Update(CloseTagManagerMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing tag manager, got %v", app.currentView())
	}
}

func TestHandleTagMessagesTagRenameKeepsActiveManagerOpen(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.Tags = []string{"backend"}
	project.Tasks = nil
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	handled, _ := app.handleTagMessages(ShowTagManagerMsg{ProjectTree: app.projectTree})
	if !handled {
		t.Fatal("ShowTagManagerMsg should be handled")
	}

	project.Tags = []string{"frontend"}
	app.projectTree.filter.Tags = map[string]bool{"backend": true}
	handled, cmd := app.handleTagMessages(TagRenamedMsg{OldName: "backend", NewName: "frontend"})
	if !handled {
		t.Fatal("TagRenamedMsg should be handled")
	}
	if cmd != nil {
		t.Fatal("TagRenamedMsg returned a command, want in-place manager refresh")
	}
	if app.currentView() != ViewTagManager {
		t.Fatalf("currentView = %v, want ViewTagManager", app.currentView())
	}
	if app.projectTree.filter.Tags["backend"] || !app.projectTree.filter.Tags["frontend"] {
		t.Fatalf("project tree tag filter = %v, want frontend only", app.projectTree.filter.Tags)
	}

	manager, ok := app.views.Get(ViewTagManager).(*TagManagerModel)
	if !ok {
		t.Fatalf("tag manager view = %T, want *TagManagerModel", app.views.Get(ViewTagManager))
	}
	got := summarizeTagInfos(manager.tags)
	want := []string{"frontend:0"}
	if !slices.Equal(got, want) {
		t.Fatalf("manager tags = %v, want %v", got, want)
	}
}

func TestHandleSectionMessagesShowSectionPicker(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowSectionPickerMsg{Task: task, Project: project}
	handled, _ := app.handleSectionMessages(msg)

	if !handled {
		t.Error("ShowSectionPickerMsg should be handled")
	}
	if app.currentView() != ViewSectionPicker {
		t.Errorf("currentView should be SectionPicker, got %v", app.currentView())
	}
}

func TestHandleClipboardMessagesTaskCopied(t *testing.T) {
	app := newTestAppModel()

	tests := []struct {
		name       string
		count      int
		fullDetail bool
		wantStatus string
	}{
		{
			name:       "single title",
			count:      1,
			fullDetail: false,
			wantStatus: "Copied title to clipboard",
		},
		{
			name:       "single description",
			count:      1,
			fullDetail: true,
			wantStatus: "Copied description to clipboard",
		},
		{
			name:       "multiple titles",
			count:      3,
			fullDetail: false,
			wantStatus: "Copied title for 3 tasks to clipboard",
		},
		{
			name:       "multiple descriptions",
			count:      5,
			fullDetail: true,
			wantStatus: "Copied description for 5 tasks to clipboard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.statusMessage = ""
			msg := TaskCopiedMsg{Count: tt.count, FullDetail: tt.fullDetail}
			handled, _ := app.handleClipboardMessages(msg)

			if !handled {
				t.Error("TaskCopiedMsg should be handled")
			}
			if app.statusMessage != tt.wantStatus {
				t.Errorf("statusMessage = %q, want %q", app.statusMessage, tt.wantStatus)
			}
		})
	}
}

func TestViewStackNavigation(t *testing.T) {
	app := newTestAppModel()

	// Initially at dashboard
	if app.currentView() != ViewDashboard {
		t.Error("should start at Dashboard")
	}
	if len(app.viewStack) != 1 {
		t.Errorf("viewStack should have 1 item initially, got %d", len(app.viewStack))
	}

	// Push some views
	app.pushView(ViewProjectTree)
	if app.currentView() != ViewProjectTree {
		t.Error("currentView should be ProjectTree after push")
	}

	app.pushView(ViewTaskForm)
	if app.currentView() != ViewTaskForm {
		t.Error("currentView should be TaskEditor after second push")
	}
	if len(app.viewStack) != 3 {
		t.Errorf("viewStack should have 3 items, got %d", len(app.viewStack))
	}

	// Pop back
	app.popView()
	if app.currentView() != ViewProjectTree {
		t.Error("currentView should be ProjectTree after pop")
	}

	app.popView()
	if app.currentView() != ViewDashboard {
		t.Error("currentView should be Dashboard after second pop")
	}

	// Pop at dashboard should not crash or go below 1
	app.popView()
	if app.currentView() != ViewDashboard {
		t.Error("currentView should still be Dashboard after pop at bottom")
	}
	if len(app.viewStack) != 1 {
		t.Errorf("viewStack should not go below 1, got %d", len(app.viewStack))
	}
}

func TestPopAndNavigateToTaskNilTaskReturnsPopCmd(t *testing.T) {
	app := newTestAppModel()
	// Set up: push a tracked view and then ViewCrossProjectOutliner so popping
	// back to it will trigger a ViewDidAppearMsg command.
	app.pushView(ViewCrossProjectOutliner)
	app.pushView(ViewTaskForm)

	handled, cmd := app.popAndNavigateToTask(nil, nil)
	if !handled {
		t.Error("popAndNavigateToTask should return handled=true")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("expected current view ViewCrossProjectOutliner, got %v", app.currentView())
	}
	// When popping into ViewCrossProjectOutliner, popView returns a cmd
	// that sends ViewDidAppearMsg. With nil task/project, that cmd must
	// be propagated back to the caller, not dropped.
	if cmd == nil {
		t.Fatal("expected pop cmd when popping into ViewCrossProjectOutliner")
	}
	msg := cmd()
	if _, ok := msg.(ViewDidAppearMsg); !ok {
		t.Errorf("expected ViewDidAppearMsg, got %T", msg)
	}
}

// TestCloseViewMsgPropagatesPopCmd verifies that the generic closeViewMsg
// handler in Update() forwards the tea.Cmd returned by popView() (or
// popToAnchorView()) when the revealed view is ViewCrossProjectOutliner.
// Previously the cmd was dropped, leaving the outliner unaware that it had
// just become visible and stale after mutations performed in a sub-view.
func TestCloseViewMsgPropagatesPopCmd(t *testing.T) {
	t.Run("popOne", func(t *testing.T) {
		app := newTestAppModel()
		app.pushView(ViewCrossProjectOutliner)
		app.pushView(ViewTaskDetail)

		_, cmd := app.Update(CloseTaskDetailMsg{})
		if app.currentView() != ViewCrossProjectOutliner {
			t.Fatalf("expected ViewCrossProjectOutliner, got %v", app.currentView())
		}
		if cmd == nil {
			t.Fatal("expected ViewDidAppearMsg cmd, got nil")
		}
		msg := cmd()
		// Batched cmds drain through tea.Batch — execute eagerly.
		if batch, ok := msg.(tea.BatchMsg); ok {
			found := false
			for _, c := range batch {
				if c == nil {
					continue
				}
				if _, ok := c().(ViewDidAppearMsg); ok {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected ViewDidAppearMsg in batch, got %v", batch)
			}
			return
		}
		if _, ok := msg.(ViewDidAppearMsg); !ok {
			t.Errorf("expected ViewDidAppearMsg, got %T", msg)
		}
	})

	t.Run("popToAnchor", func(t *testing.T) {
		app := newTestAppModel()
		app.pushView(ViewCrossProjectOutliner)
		// Push a non-anchor view that the anchor-pop will unwind past.
		app.pushView(ViewTaskDetail)
		// Push a global overlay-style anchor target. CloseFocusBoardMsg is
		// declared with PopToAnchorMsg so this exercises the popToAnchor arm.
		app.pushView(ViewFocusBoard)

		_, cmd := app.Update(CloseFocusBoardMsg{})
		if app.currentView() != ViewCrossProjectOutliner {
			t.Fatalf("expected ViewCrossProjectOutliner, got %v", app.currentView())
		}
		if cmd == nil {
			t.Fatal("expected ViewDidAppearMsg cmd, got nil")
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			found := false
			for _, c := range batch {
				if c == nil {
					continue
				}
				if _, ok := c().(ViewDidAppearMsg); ok {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected ViewDidAppearMsg in batch, got %v", batch)
			}
			return
		}
		if _, ok := msg.(ViewDidAppearMsg); !ok {
			t.Errorf("expected ViewDidAppearMsg, got %T", msg)
		}
	})
}

func TestStatusTimeout(t *testing.T) {
	cmd := statusTimeout(100 * time.Millisecond)
	if cmd == nil {
		t.Error("statusTimeout should return a command")
	}

	// Execute the command and verify it returns StatusClearMsg
	msg := cmd()
	if _, ok := msg.(StatusClearMsg); !ok {
		t.Errorf("statusTimeout should return StatusClearMsg, got %T", msg)
	}
}

func TestErrorCmd(t *testing.T) {
	cmd := errorCmd("test error: %s", "details")
	if cmd == nil {
		t.Fatal("errorCmd should return a command")
	}

	msg := cmd()
	errMsg, ok := msg.(ErrorMsg)
	if !ok {
		t.Fatalf("errorCmd should return ErrorMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Error("ErrorMsg.Err should not be nil")
	}
	if errMsg.Err.Error() != "test error: details" {
		t.Errorf("error message = %q, want %q", errMsg.Err.Error(), "test error: details")
	}
}

func TestUnhandledMessages(t *testing.T) {
	app := newTestAppModel()

	// Create a custom message type that shouldn't be handled
	type unknownMsg struct{}

	handled, _ := app.handleProjectMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleProjectMessages")
	}

	handled, _ = app.handleTaskMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleTaskMessages")
	}

	handled, _ = app.handleViewMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleViewMessages")
	}

	handled, _ = app.handleTagMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleTagMessages")
	}

	handled, _ = app.handleSectionMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleSectionMessages")
	}

	handled, _ = app.handleClipboardMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleClipboardMessages")
	}
}
