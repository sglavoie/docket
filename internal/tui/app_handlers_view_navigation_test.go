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
	handled, cmd := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowDailyReviewMsg should be handled")
	}
	// Daily Review is an Outliner computed mode: the launch message redirects into
	// the global Outliner and selects the Daily Review mode.
	if cmd == nil {
		t.Fatal("ShowDailyReviewMsg should emit an Outliner redirect command")
	}
	outliner, ok := cmd().(ShowGlobalOutlinerMsg)
	if !ok || !outliner.Options.SetMode || outliner.Options.Mode != ModeDailyReview {
		t.Errorf("ShowDailyReviewMsg redirect = %#v, want Daily Review Outliner mode", cmd())
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
	if app.outliner != nil {
		t.Fatal("project navigation must not construct a separate AppModel.outliner root")
	}
	if app.crossOutliner == nil {
		t.Fatal("expected the single cross-project Outliner to own the scoped work")
	}
	if app.crossOutliner.scopedProjectID != project.ID {
		t.Errorf("expected project scope %q, got %q", project.ID, app.crossOutliner.scopedProjectID)
	}
	if app.crossOutliner.activePerspective != PerspectiveNext {
		t.Errorf("project focus should use Next perspective, got %d", app.crossOutliner.activePerspective)
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
	if app.outliner != nil {
		t.Fatal("project navigation must not construct a separate AppModel.outliner root")
	}
	if app.crossOutliner == nil || app.crossOutliner.smartFocus == nil || *app.crossOutliner.smartFocus != OutlinerHighPriority {
		t.Errorf("priority focus should use the scoped cross Outliner, got focus %v", func() any {
			if app.crossOutliner == nil {
				return nil
			}
			return app.crossOutliner.smartFocus
		}())
	}
	if app.crossOutliner != nil && app.crossOutliner.scopedProjectID != project.ID {
		t.Errorf("expected project scope %q, got %q", project.ID, app.crossOutliner.scopedProjectID)
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

func TestHandleViewMessagesShowRecentActivity(t *testing.T) {
	app := newTestAppModel()

	msg := ShowRecentActivityMsg{}
	handled, cmd := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowRecentActivityMsg should be handled")
	}
	// Activity is an Outliner computed mode: the launch message redirects into the
	// global Outliner and selects the Activity mode.
	if cmd == nil {
		t.Fatal("ShowRecentActivityMsg should emit an Outliner redirect command")
	}
	outliner, ok := cmd().(ShowGlobalOutlinerMsg)
	if !ok || !outliner.Options.SetMode || outliner.Options.Mode != ModeActivity {
		t.Errorf("ShowRecentActivityMsg redirect = %#v, want Activity Outliner mode", cmd())
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
