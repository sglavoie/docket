package tui

import "testing"

func TestHandleViewMessagesCloseTaskJumper(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskJumper)

	app.Update(CloseTaskJumperMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing jumper, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowProjectPreview(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowProjectPreviewMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowProjectPreviewMsg should be handled")
	}
	if app.currentView() != ViewProjectPreview {
		t.Errorf("currentView should be ProjectPreview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseProjectPreview(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectPreview)

	app.Update(CloseProjectPreviewMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing preview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowDailyReview(t *testing.T) {
	app := newTestAppModel()

	msg := ShowDailyReviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowDailyReviewMsg should be handled")
	}
	if app.currentView() != ViewDailyReview {
		t.Errorf("currentView should be DailyReview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseDailyReview(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewDailyReview)

	app.Update(CloseDailyReviewMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing review, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowProjectFocusInOutliner(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowOutlinerMsg{Project: project, Options: OutlinerOpenOptions{Focus: OutlinerNext, SetFocus: true}}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowOutlinerMsg should be handled")
	}
	if app.currentView() != ViewOutliner || app.outliner.activePerspective != PerspectiveNext {
		t.Errorf("project focus should use Next Outliner, got %v perspective %d", app.currentView(), app.outliner.activePerspective)
	}
}

func TestHandleViewMessagesCloseCommandPalette(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewCommandPalette)

	app.Update(CloseCommandPaletteMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing palette, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowPriorityFocusInOutliner(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowOutlinerMsg{Project: project, Options: OutlinerOpenOptions{Focus: OutlinerHighPriority, SetFocus: true}}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowOutlinerMsg should be handled")
	}
	if app.currentView() != ViewOutliner || app.outliner.smartFocus == nil || *app.outliner.smartFocus != OutlinerHighPriority {
		t.Errorf("priority focus should use Outliner, got %v focus %v", app.currentView(), app.outliner.smartFocus)
	}
}

func TestHandleViewMessagesShowWeeklyReview(t *testing.T) {
	app := newTestAppModel()

	msg := ShowWeeklyReviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowWeeklyReviewMsg should be handled")
	}
	if app.currentView() != ViewWeeklyReview {
		t.Errorf("currentView should be WeeklyReview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseWeeklyReview(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewWeeklyReview)

	app.Update(CloseWeeklyReviewMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing weekly review, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowAllTasksInCrossProjectOutliner(t *testing.T) {
	app := newTestAppModel()

	msg := ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{Focus: OutlinerAll, SetFocus: true}}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowGlobalOutlinerMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner || app.crossOutliner.activePerspective != PerspectiveAll {
		t.Errorf("all tasks should use cross-project Outliner, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowFocusBoard(t *testing.T) {
	app := newTestAppModel()

	msg := ShowFocusBoardMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowFocusBoardMsg should be handled")
	}
	if app.currentView() != ViewFocusBoard {
		t.Errorf("currentView should be FocusBoard, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowRecentActivity(t *testing.T) {
	app := newTestAppModel()

	msg := ShowRecentActivityMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowRecentActivityMsg should be handled")
	}
	if app.currentView() != ViewRecentActivity {
		t.Errorf("currentView should be RecentActivity, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseRecentActivity(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewRecentActivity)

	app.Update(CloseRecentActivityMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing recent activity, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowBlockers(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowBlockersMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowBlockersMsg should be handled")
	}
	if app.currentView() != ViewBlockers {
		t.Errorf("currentView should be Blockers, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseBlockers(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewBlockers)

	app.Update(CloseBlockersMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing blockers, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowGlobalBlockers(t *testing.T) {
	app := newTestAppModel()

	msg := ShowGlobalBlockersMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowGlobalBlockersMsg should be handled")
	}
	if app.currentView() != ViewGlobalBlockers {
		t.Errorf("currentView should be GlobalBlockers, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseGlobalBlockers(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewGlobalBlockers)

	app.Update(CloseGlobalBlockersMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing global blockers, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowTaskAttachments(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskAttachmentsMsg{Task: task, Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowTaskAttachmentsMsg should be handled")
	}
	if app.currentView() != ViewTaskAttachments {
		t.Errorf("currentView should be TaskAttachments, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseTaskAttachments(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskAttachments)

	app.Update(CloseTaskAttachmentsMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing attachments, got %v", app.currentView())
	}
}

func TestGetCurrentCursorTask_TemporaryOutliner(t *testing.T) {
	app, project := newTestAppModelWithProject()

	// Create a temporary outliner and register it (simulating the drill-down
	// from the cross-project outliner). The temporary outliner is NOT stored
	// in m.outliner — it only lives in the view registry.
	tempOutliner := NewOutlinerModel(project, app.configManager, app.runtime)
	tempOutliner.SetSize(80, 22)
	app.views.Register(ViewTemporaryOutliner, tempOutliner)
	app.pushView(ViewTemporaryOutliner)

	// getCurrentCursorTask must resolve via the registry, not m.outliner.
	// Before the fix, m.outliner was nil so the result was always nil.
	task := app.getCurrentCursorTask()

	// If the outliner has rows with tasks, we should get a task back.
	// If there are no rows (empty project), nil is acceptable.
	if len(tempOutliner.rows) > 0 && !tempOutliner.rows[0].IsSectionHeader && !tempOutliner.rows[0].IsSeparator {
		if task == nil {
			t.Error("getCurrentCursorTask should return a task from the temporary outliner, got nil")
		}
	}

	// Verify that m.outliner is still nil (not clobbered)
	if app.outliner != nil {
		t.Error("m.outliner should remain nil when using a temporary outliner")
	}
}

func TestHasActiveMultiSelect_TemporaryOutliner(t *testing.T) {
	app, project := newTestAppModelWithProject()

	tempOutliner := NewOutlinerModel(project, app.configManager, app.runtime)
	tempOutliner.SetSize(80, 22)
	tempOutliner.multiSelectMode = true
	app.views.Register(ViewTemporaryOutliner, tempOutliner)
	app.pushView(ViewTemporaryOutliner)

	if !app.hasActiveMultiSelect() {
		t.Error("hasActiveMultiSelect should detect multi-select on a temporary outliner")
	}
}
