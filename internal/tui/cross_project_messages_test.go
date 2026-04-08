package tui

import "testing"

func TestCloseCrossOutlinerMsgPreservesOutliner(t *testing.T) {
	app, project := newTestAppModelWithProject()

	projectOutliner := NewOutlinerModel(project, app.config, app.configManager)
	app.outliner = projectOutliner
	app.views.Register(ViewOutliner, projectOutliner)
	app.viewStack = append(app.viewStack, ViewOutliner)

	crossOutliner := NewOutlinerModel(nil, app.config, app.configManager)
	app.crossOutliner = crossOutliner
	app.views.Register(ViewCrossProjectOutliner, crossOutliner)
	app.viewStack = append(app.viewStack, ViewCrossProjectOutliner)

	handled, _ := app.handleViewMessages(CloseCrossOutlinerMsg{})
	if !handled {
		t.Fatal("CloseCrossOutlinerMsg should be handled by the app")
	}

	current := app.currentView()
	switch current {
	case ViewDashboard, ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner:
	default:
		t.Errorf("view stack should be at an anchor view after CloseCrossOutlinerMsg, got %v", current)
	}

	if app.crossOutliner == nil {
		t.Error("m.crossOutliner must not be nil after CloseCrossOutlinerMsg")
	}
	if app.outliner == nil {
		t.Error("m.outliner must not be nil after CloseCrossOutlinerMsg")
	}
	if app.views.Get(ViewOutliner) == nil {
		t.Error("ViewOutliner registry entry must not be nil after CloseCrossOutlinerMsg")
	}
}
