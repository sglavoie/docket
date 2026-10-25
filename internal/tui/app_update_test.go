package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

type panicCustomMsg struct{}

type panicOnCustomView struct {
	listsurface.BaseView
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
	app.viewNav.views.Register(presentation.ViewCrossProjectOutliner, &panicOnCustomView{})

	model, _ := app.Update(panicCustomMsg{})
	if model == nil {
		t.Fatal("expected non-nil model after panic recovery")
	}
	if path := app.presentationPath(); len(path) != 1 || path[0] != presentation.ViewCrossProjectOutliner {
		t.Fatalf("expected navigation to recover at the Outliner root, got %#v", path)
	}
	if app.feedback.err == nil {
		t.Fatal("expected err to be set after panic recovery")
	}
}

func TestUpdatePanicRecoveryCancelsPendingCrossProjectRefresh(t *testing.T) {
	app := newTestAppModel()
	project := scopeTestProject("alpha", "Alpha", "task")
	cross := outliner.NewOutlinerModel(nil, app.domain.manager, app.domain.runtime)
	cross.Population.Projects = []*models.Project{project}
	cross.Population.SnapshotReady = true
	cross.SetSize(app.viewNav.viewSize())
	cross.Population.Invalidate()
	cross.CrossProjectRebuildRows()
	app.crossOutliner = cross
	app.viewNav.views.Register(presentation.ViewCrossProjectOutliner, cross)
	app.viewNav.views.Register(presentation.ViewTaskComments, &panicOnCustomView{})
	pushTestPresentationOverlay(app, presentation.ViewTaskComments)

	_ = cross.ReloadCrossProjects()
	abandonedGeneration := cross.Population.LoadGen
	cross.Population.SpinnerVisible = true

	model, cmd := app.Update(panicCustomMsg{})
	if model == nil || cmd != nil {
		t.Fatalf("panic recovery = model %T cmd %T, want model and no abandoned command", model, cmd)
	}
	if cross.Population.Loading || cross.Population.SpinnerVisible {
		t.Fatal("panic recovery left the cross-project refresh active")
	}
	if cross.Population.LoadGen == abandonedGeneration {
		t.Fatal("panic recovery did not invalidate the abandoned load generation")
	}
	if got := app.currentView(); got != presentation.ViewCrossProjectOutliner {
		t.Fatalf("recovery view = %s, want CrossProjectOutliner", got)
	}
	if content := app.View().Content; content == "" || strings.Contains(content, "Unknown view") {
		t.Fatalf("recovered root is not renderable:\n%s", content)
	}

	app.Update(outliner.CrossProjectsLoadedMsg{Generation: abandonedGeneration})
	if len(cross.Population.Projects) != 1 || cross.Population.Projects[0].ID != project.ID {
		t.Fatalf("stale load replaced the recovered snapshot: %#v", cross.Population.Projects)
	}
}
