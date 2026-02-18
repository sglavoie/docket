package tui

import (
	"errors"
	"testing"
)

func TestHandleViewMessagesBack(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectTree)
	app.pushView(ViewTaskEditor)

	msg := BackMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BackMsg should be handled")
	}
	if app.currentView() != ViewProjectTree {
		t.Errorf("currentView should be ProjectTree after back, got %v", app.currentView())
	}
}

func TestHandleViewMessagesError(t *testing.T) {
	app := newTestAppModel()
	testErr := errors.New("task not found")

	msg := ErrorMsg{Err: testErr}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ErrorMsg should be handled")
	}
	if !errors.Is(app.err, testErr) {
		t.Error("app.err should be set to the error from message")
	}
}

func TestHandleViewMessagesSettings(t *testing.T) {
	app := newTestAppModel()

	msg := SettingsMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("SettingsMsg should be handled")
	}
	if app.currentView() != ViewSettings {
		t.Errorf("currentView should be Settings, got %v", app.currentView())
	}
}

func TestHandleViewMessagesSettingsSaved(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewSettings)

	msg := SettingsSavedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("SettingsSavedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after settings saved, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowHelp(t *testing.T) {
	app := newTestAppModel()
	items := []HelpItem{{Key: "q", Description: "quit"}}

	msg := ShowHelpMsg{Items: items, SourceView: ViewDashboard, ViewName: "Dashboard"}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowHelpMsg should be handled")
	}
	if app.currentView() != ViewHelp {
		t.Errorf("currentView should be Help, got %v", app.currentView())
	}
	if app.helpView == nil {
		t.Error("helpView should be initialized")
	}
}

func TestHandleViewMessagesCloseHelp(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewHelp)

	msg := CloseHelpMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseHelpMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing help, got %v", app.currentView())
	}
}

func TestHandleViewMessagesStatusClear(t *testing.T) {
	app := newTestAppModel()
	app.statusMessage = "Some status"

	msg := StatusClearMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.statusMessage != "" {
		t.Errorf("statusMessage should be cleared, got %q", app.statusMessage)
	}
}

func TestHandleViewMessagesMoveBlocked(t *testing.T) {
	app := newTestAppModel()

	msg := MoveBlockedMsg{Reason: "Cannot move completed task"}
	handled, cmd := app.handleViewMessages(msg)

	if !handled {
		t.Error("MoveBlockedMsg should be handled")
	}
	if app.statusMessage != "Cannot move completed task" {
		t.Errorf("statusMessage should be set to reason, got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("MoveBlockedMsg should return a status timeout command")
	}
}

func TestHandleViewMessagesRefresh(t *testing.T) {
	app := newTestAppModel()
	app.dashboard = NewDashboardModel(app.config)

	msg := RefreshMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("RefreshMsg should be handled")
	}
}

func TestHandleViewMessagesShowWorkflowRules(t *testing.T) {
	app := newTestAppModel()

	msg := ShowWorkflowRulesMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowWorkflowRulesMsg should be handled")
	}
	if app.currentView() != ViewWorkflowRules {
		t.Errorf("currentView should be WorkflowRules, got %v", app.currentView())
	}
}

func TestHandleViewMessagesWorkflowRulesUpdated(t *testing.T) {
	app := newTestAppModel()

	msg := WorkflowRulesUpdatedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("WorkflowRulesUpdatedMsg should be handled")
	}
}

func TestHandleViewMessagesWorkflowRuleCreated(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewWorkflowRuleCreator)

	msg := WorkflowRuleCreatedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("WorkflowRuleCreatedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after rule created, got %v", app.currentView())
	}
}

func TestHandleViewMessagesWorkflowRuleEdited(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewWorkflowRuleEditor)

	msg := WorkflowRuleEditedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("WorkflowRuleEditedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after rule edited, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowFilter(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	msg := ShowFilterMsg{ProjectTree: app.projectTree}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowFilterMsg should be handled")
	}
	if app.currentView() != ViewFilter {
		t.Errorf("currentView should be Filter, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseFilter(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewFilter)

	msg := CloseFilterMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseFilterMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing filter, got %v", app.currentView())
	}
}

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

func TestHandleViewMessagesShowQuickCapture(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowQuickCaptureMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowQuickCaptureMsg should be handled")
	}
	if app.currentView() != ViewQuickCapture {
		t.Errorf("currentView should be QuickCapture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseQuickCapture(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewQuickCapture)

	msg := CloseQuickCaptureMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseQuickCaptureMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing quick capture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowBulkCapture(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowBulkCaptureMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowBulkCaptureMsg should be handled")
	}
	if app.currentView() != ViewBulkCapture {
		t.Errorf("currentView should be BulkCapture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseBulkCapture(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewBulkCapture)

	msg := CloseBulkCaptureMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseBulkCaptureMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing bulk capture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesBulkEditDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewBulkEdit)

	msg := BulkEditDoneMsg{Applied: false}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BulkEditDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after bulk edit done, got %v", app.currentView())
	}
}

func TestHandleViewMessagesRecurrenceEditorDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewRecurrenceEditor)

	msg := RecurrenceEditorDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("RecurrenceEditorDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after recurrence editor done, got %v", app.currentView())
	}
}

func TestHandleViewMessagesTextEditorDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTextEditor)

	msg := TextEditorDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("TextEditorDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after text editor done, got %v", app.currentView())
	}
}
