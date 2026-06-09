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

func TestHandleGlobalKeyPress_SpaceFallsThroughWithoutCursorTask(t *testing.T) {
	app := newTestAppModel()

	handled, _ := app.handleGlobalKeyPress(syntheticSpecialKey(tea.KeySpace))
	if handled {
		t.Fatal("expected space to fall through when there is no cursor task")
	}
}

func TestCustomContextEditorSpaceTogglesProjectThroughApp(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.config.SetCustomContext("Work", []string{project.ID})
	editor := NewCustomContextEditorModel(app.config, "Work")
	app.showView(ViewCustomContextEditor, editor)

	if !editor.selectedIDs[project.ID] {
		t.Fatal("expected project to start selected in existing custom context")
	}

	app.Update(syntheticSpecialKey(tea.KeyTab))
	if !editor.projectListMode {
		t.Fatal("expected tab to move focus to project list")
	}

	app.Update(syntheticSpecialKey(tea.KeySpace))
	if editor.selectedIDs[project.ID] {
		t.Fatal("expected space to toggle selected project off through app dispatch")
	}
	if app.peekActive {
		t.Fatal("space should not activate peek in custom context editor")
	}
}
