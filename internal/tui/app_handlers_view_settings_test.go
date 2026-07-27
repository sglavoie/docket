package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after settings saved, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowHelp(t *testing.T) {
	app := newTestAppModel()
	items := []HelpItem{{Key: "esc", Description: "close"}}

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

	app.Update(CloseHelpMsg{})

	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after closing help, got %v", app.currentView())
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
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after rule created, got %v", app.currentView())
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
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after rule edited, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowFilter(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.crossOutliner = NewOutlinerModel(project, nil, app.runtime)

	msg := ShowFilterMsg{CommonFilter: &app.crossOutliner.filter.CommonFilterState, Project: project}
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

	app.Update(CloseFilterMsg{})

	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after closing filter, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowTaskTemplates(t *testing.T) {
	app := newTestAppModel()

	msg := ShowTaskTemplatesMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowTaskTemplatesMsg should be handled")
	}
	if app.currentView() != ViewTaskTemplates {
		t.Errorf("currentView should be TaskTemplates, got %v", app.currentView())
	}
}

func TestHandleViewMessagesTaskTemplatesUpdated(t *testing.T) {
	app := newTestAppModel()

	msg := TaskTemplatesUpdatedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("TaskTemplatesUpdatedMsg should be handled")
	}
}

func TestHandleViewMessagesShowNotificationCenter(t *testing.T) {
	app := newTestAppModel()

	msg := ShowNotificationCenterMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowNotificationCenterMsg should be handled")
	}
	if app.currentView() != ViewNotificationCenter {
		t.Errorf("currentView should be NotificationCenter, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseNotificationCenter(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewNotificationCenter)

	app.Update(CloseNotificationCenterMsg{})

	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after closing notification center, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowTutor(t *testing.T) {
	app := newTestAppModel()

	msg := ShowTutorMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowTutorMsg should be handled")
	}
	if app.currentView() != ViewTutor {
		t.Errorf("currentView should be Tutor, got %v", app.currentView())
	}
}

func TestHandleViewMessagesFilterBuilderSaved(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewFilterBuilder)

	msg := FilterBuilderSavedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("FilterBuilderSavedMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after filter builder saved, got %v", app.currentView())
	}
}

func TestHandleViewMessagesTemplatePickerDoneCanceled(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskTemplatePicker)
	app.pendingCreateTaskParent = &models.Task{}

	msg := TemplatePickerDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("TemplatePickerDoneMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after canceled template picker, got %v", app.currentView())
	}
	if app.pendingCreateTaskParent != nil {
		t.Error("pendingCreateTaskParent should be cleared after canceling template picker")
	}
}
