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

func TestUpdatePanicRecoveryRestoresOutlinerRoot(t *testing.T) {
	app := newTestAppModel()
	app.views.Register(ViewCrossProjectOutliner, &panicOnCustomView{})

	model, _ := app.Update(panicCustomMsg{})
	if model == nil {
		t.Fatal("expected non-nil model after panic recovery")
	}
	if path := app.presentationPath(); len(path) != 1 || path[0] != ViewCrossProjectOutliner {
		t.Fatalf("expected navigation to recover at the Outliner root, got %#v", path)
	}
	if app.err == nil {
		t.Fatal("expected err to be set after panic recovery")
	}
}
