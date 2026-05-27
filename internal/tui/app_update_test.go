package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

type panicCustomMsg struct{}

type panicOnCustomView struct {
	BaseView
}

func (v *panicOnCustomView) Init() tea.Cmd { return nil }

func (v *panicOnCustomView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(panicCustomMsg); ok {
		panic("forced panic for recovery test")
	}
	return v, nil
}

func (v *panicOnCustomView) View() tea.View { return tea.NewView("") }

func TestUpdatePanicRecoveryRestoresEmptyViewStack(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = nil
	app.views.Register(ViewDashboard, &panicOnCustomView{})

	model, _ := app.Update(panicCustomMsg{})
	if model == nil {
		t.Fatal("expected non-nil model after panic recovery")
	}
	if len(app.viewStack) != 1 || app.viewStack[0] != ViewDashboard {
		t.Fatalf("expected viewStack to be restored to [ViewDashboard], got %#v", app.viewStack)
	}
	if app.err == nil {
		t.Fatal("expected err to be set after panic recovery")
	}
}
