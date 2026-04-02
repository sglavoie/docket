package tui

import "testing"

func TestHandleViewMessagesCloseTaskJumper(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskJumper)

	msg := CloseTaskJumperMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseTaskJumperMsg should be handled")
	}
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

	msg := CloseProjectPreviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseProjectPreviewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing preview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowDueDateOverview(t *testing.T) {
	app := newTestAppModel()

	msg := ShowDueDateOverviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowDueDateOverviewMsg should be handled")
	}
	if app.currentView() != ViewDueDateOverview {
		t.Errorf("currentView should be DueDateOverview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseDueDateOverview(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewDueDateOverview)

	msg := CloseDueDateOverviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseDueDateOverviewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing overview, got %v", app.currentView())
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

	msg := CloseDailyReviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseDailyReviewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing review, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowFocusView(t *testing.T) {
	app := newTestAppModel()

	msg := ShowFocusViewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowFocusViewMsg should be handled")
	}
	if app.currentView() != ViewFocusView {
		t.Errorf("currentView should be FocusView, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowProjectFocusView(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowProjectFocusViewMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowProjectFocusViewMsg should be handled")
	}
	if app.currentView() != ViewProjectFocusView {
		t.Errorf("currentView should be ProjectFocusView, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseProjectFocusView(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectFocusView)

	msg := CloseProjectFocusViewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseProjectFocusViewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing project focus, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowGlobalSearch(t *testing.T) {
	app := newTestAppModel()

	msg := ShowGlobalSearchMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowGlobalSearchMsg should be handled")
	}
	if app.currentView() != ViewGlobalSearch {
		t.Errorf("currentView should be GlobalSearch, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseGlobalSearch(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewGlobalSearch)

	msg := CloseGlobalSearchMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseGlobalSearchMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing search, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseCommandPalette(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewCommandPalette)

	msg := CloseCommandPaletteMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseCommandPaletteMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing palette, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowPriorityView(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowPriorityViewMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowPriorityViewMsg should be handled")
	}
	if app.currentView() != ViewPriorityView {
		t.Errorf("currentView should be PriorityView, got %v", app.currentView())
	}
}

func TestHandleViewMessagesClosePriorityView(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewPriorityView)

	msg := ClosePriorityViewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ClosePriorityViewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing priority view, got %v", app.currentView())
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

	msg := CloseWeeklyReviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseWeeklyReviewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing weekly review, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowAllTasks(t *testing.T) {
	app := newTestAppModel()

	msg := ShowAllTasksMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowAllTasksMsg should be handled")
	}
	if app.currentView() != ViewAllTasks {
		t.Errorf("currentView should be AllTasks, got %v", app.currentView())
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

	msg := CloseRecentActivityMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseRecentActivityMsg should be handled")
	}
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

	msg := CloseBlockersMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseBlockersMsg should be handled")
	}
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

	msg := CloseGlobalBlockersMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseGlobalBlockersMsg should be handled")
	}
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

	msg := CloseTaskAttachmentsMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseTaskAttachmentsMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing attachments, got %v", app.currentView())
	}
}
