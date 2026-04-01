package tui

import "testing"

func TestCrossProjectOutlinerQAtRootEmitsCloseCrossOutlinerMsg(t *testing.T) {
	app := newTestAppModel()
	m := NewOutlinerModel(nil, app.config, app.configManager)

	key := syntheticKey('q')
	_, cmd := m.handleKeyMsg(key)
	if cmd == nil {
		t.Fatal("expected a command from 'q' at cross-project root, got nil")
	}
	msg := cmd()
	if _, ok := msg.(CloseCrossOutlinerMsg); !ok {
		t.Errorf("expected CloseCrossOutlinerMsg, got %T", msg)
	}
}

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
