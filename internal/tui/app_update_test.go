package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
)

type panicCustomMsg struct{}

type panicCloseViewMsg struct{}

func (panicCloseViewMsg) closePopStyle() popStyle {
	panic("forced panic before close navigation")
}

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

func TestUpdatePanicRecoveryCancelsRegisteredAskStream(t *testing.T) {
	for _, tc := range []struct {
		name       string
		coveredAsk bool
	}{
		{name: "ask on top"},
		{name: "another view on top", coveredAsk: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := newBlockingAskStream()
			ask := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{
				streams: []external.Stream{stream},
			})
			start := askStartMessage(t, ask.submitQuery("panic request"))
			_, _ = ask.Update(start)

			app := newTestAppModel()
			app.views.Register(ViewAsk, ask)
			app.pushView(ViewAsk)
			if tc.coveredAsk {
				app.pushView(ViewMessageLog)
			}

			model, cmd := app.Update(panicCloseViewMsg{})
			if model == nil || cmd != nil || app.err == nil {
				t.Fatalf("panic recovery = model %v cmd %v err %v", model, cmd, app.err)
			}
			if !stream.Canceled() || ask.cancelFn != nil || ask.stream != nil || ask.state != askStateInput {
				t.Fatalf("Ask panic cleanup = canceled %v cancelFn %v stream %v state %v", stream.Canceled(), ask.cancelFn != nil, ask.stream, ask.state)
			}
			if path := app.presentationPath(); len(path) != 1 || app.currentView() != ViewCrossProjectOutliner {
				t.Fatalf("panic path = %#v, want Outliner root", path)
			}
		})
	}
}
