package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

type confirmQueueToggleView struct {
	BaseView
	task          *models.Task
	project       *models.Project
	confirmActive bool
}

func (v *confirmQueueToggleView) Init() tea.Cmd { return nil }

func (v *confirmQueueToggleView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

func (v *confirmQueueToggleView) View() tea.View { return tea.NewView("") }

func (v *confirmQueueToggleView) getTaskRef() (*models.Task, *models.Project) {
	return v.task, v.project
}

func (v *confirmQueueToggleView) IsConfirmActive() bool { return v.confirmActive }

func TestHandleGlobalKeyPress_QIgnoredDuringConfirmDialog(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.focusQueueIDs = map[string]bool{}
	app.dailyPlanIDs = map[string]bool{}

	view := &confirmQueueToggleView{
		task:          project.Tasks[0],
		project:       project,
		confirmActive: true,
	}
	app.views.Register(ViewDailyReview, view)
	app.viewStack = []ViewType{ViewDashboard, ViewDailyReview}

	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if handled {
		t.Fatal("expected q to fall through while confirm dialog is active")
	}
	if len(app.focusQueue) != 0 {
		t.Fatalf("expected queue unchanged, got %d items", len(app.focusQueue))
	}
	if app.focusQueueIDs[project.Tasks[0].ID] {
		t.Fatal("expected task to remain out of queue while confirm dialog is active")
	}
}
