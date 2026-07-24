package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func projectContextShellProject(t *testing.T, id string, withTask bool) *models.Project {
	t.Helper()
	project := &models.Project{ID: id, Name: id, Slug: id, Path: t.TempDir()}
	if withTask {
		task, err := models.NewTask(id+"-task", 1, models.TaskTypeTask, id+" task")
		if err != nil {
			t.Fatalf("NewTask: %v", err)
		}
		task.WorkflowState = models.StateInProgress
		project.Tasks = []*models.Task{task}
		project.BuildTaskTree()
	}
	return project
}

func registerProjectContextShellOutliner(app *AppModel, outliner *OutlinerModel) {
	app.crossOutliner = outliner
	app.views.Register(ViewCrossProjectOutliner, outliner)
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner}
}

func newProjectContextHeaderShellApp(t *testing.T) (*AppModel, *models.Project) {
	t.Helper()
	app, stale := newTestAppModelWithProject()
	alpha := projectContextShellProject(t, "header-alpha", true)
	target := projectContextShellProject(t, "header-target", true)
	setTestAppProjects(app, stale, alpha, target)
	app.currentProject = stale

	outliner := NewOutlinerModel(nil, app.configManager, app.runtime)
	outliner.loadedProjects = []*models.Project{alpha, target}
	outliner.activePerspective = PerspectiveAll + 1
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	registerProjectContextShellOutliner(app, outliner)

	header := -1
	for i, row := range outliner.rows {
		if row.IsProjectHeader && row.ProjectID == target.ID {
			header = i
			break
		}
	}
	if header < 0 {
		t.Fatalf("target project header not found in %d rows", len(outliner.rows))
	}
	for outliner.cursor != header {
		before := outliner.cursor
		_, cmd := app.Update(syntheticKey('j'))
		if cmd != nil {
			_ = projectContextShellMessages(cmd)
		}
		if outliner.cursor == before {
			t.Fatalf("AppModel.Update navigation stalled at row %d", before)
		}
	}
	if app.currentProject == target {
		t.Fatal("fixture currentProject must remain stale and differ from the header target")
	}
	return app, target
}

func newProjectContextScopedEmptyShellApp(t *testing.T) (*AppModel, *models.Project) {
	t.Helper()
	app, stale := newTestAppModelWithProject()
	target := projectContextShellProject(t, "empty-scope-target", false)
	setTestAppProjects(app, stale, target)
	app.currentProject = stale

	outliner := NewOutlinerModel(nil, app.configManager, app.runtime)
	outliner.loadedProjects = []*models.Project{target}
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	registerProjectContextShellOutliner(app, outliner)

	_, _ = app.Update(ApplyProjectScopeMsg{ProjectID: target.ID})
	if outliner.scopedProjectID != target.ID {
		t.Fatalf("scope = %q, want %q", outliner.scopedProjectID, target.ID)
	}
	if got := outliner.contextProject(); got != target {
		t.Fatalf("contextProject = %v, want empty scoped project %v", got, target)
	}
	if app.currentProject == target {
		t.Fatal("fixture currentProject must remain stale and differ from the scope target")
	}
	return app, target
}

func projectContextShellMessages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var messages []tea.Msg
		for _, child := range batch {
			messages = append(messages, projectContextShellMessages(child)...)
		}
		return messages
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

func sendProjectContextShellKeys(app *AppModel, keys ...tea.KeyPressMsg) tea.Msg {
	var messages []tea.Msg
	for _, key := range keys {
		_, cmd := app.Update(key)
		messages = append(messages, projectContextShellMessages(cmd)...)
	}
	if len(messages) == 0 {
		return nil
	}
	return messages[len(messages)-1]
}

func projectFromShellMessage(t *testing.T, msg tea.Msg) *models.Project {
	t.Helper()
	switch msg := msg.(type) {
	case ShowQuickCaptureMsg:
		return msg.Project
	case ShowBulkCaptureMsg:
		return msg.Project
	case ShowNotesMsg:
		return msg.Project
	case ShowNoteCreatorMsg:
		return msg.Project
	case ShowBlockersMsg:
		return msg.Project
	case ShowInboxProcessorMsg:
		return msg.Project
	case ShowProjectRecentActivityMsg:
		return msg.Project
	case EditTaskMsg:
		return msg.Project
	default:
		t.Fatalf("shell keyflow emitted %T", msg)
		return nil
	}
}

func projectContextShellKeyCases() []struct {
	name string
	keys []tea.KeyPressMsg
} {
	return []struct {
		name string
		keys []tea.KeyPressMsg
	}{
		{name: "quick capture", keys: []tea.KeyPressMsg{syntheticKey('i')}},
		{name: "bulk capture", keys: []tea.KeyPressMsg{syntheticKey('I')}},
		{name: "open notes", keys: []tea.KeyPressMsg{syntheticKey('{')}},
		{name: "create note", keys: []tea.KeyPressMsg{syntheticKey('}')}},
		{name: "blockers", keys: []tea.KeyPressMsg{syntheticKey('B')}},
		{name: "inbox processor", keys: []tea.KeyPressMsg{syntheticKey('W')}},
		{name: "project activity", keys: []tea.KeyPressMsg{syntheticKey('g'), syntheticKey('l')}},
		{name: "edit last modified", keys: []tea.KeyPressMsg{syntheticCtrlKey('l')}},
	}
}

func TestProjectContextShellHeaderKeyflows(t *testing.T) {
	for _, tc := range projectContextShellKeyCases() {
		t.Run(tc.name, func(t *testing.T) {
			app, target := newProjectContextHeaderShellApp(t)
			msg := sendProjectContextShellKeys(app, tc.keys...)
			if got := projectFromShellMessage(t, msg); got != target {
				t.Fatalf("project = %v, want navigated header project %v", got, target)
			}
		})
	}
}

func TestProjectContextShellScopedEmptyKeyflows(t *testing.T) {
	for _, tc := range projectContextShellKeyCases() {
		if tc.name == "edit last modified" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			app, target := newProjectContextScopedEmptyShellApp(t)
			msg := sendProjectContextShellKeys(app, tc.keys...)
			if got := projectFromShellMessage(t, msg); got != target {
				t.Fatalf("project = %v, want empty scoped project %v", got, target)
			}
		})
	}

	t.Run("edit last modified reports logical no-task outcome", func(t *testing.T) {
		app, _ := newProjectContextScopedEmptyShellApp(t)
		if msg := sendProjectContextShellKeys(app, syntheticCtrlKey('l')); msg != nil {
			t.Fatalf("edit last modified emitted %T, want status-only outcome", msg)
		}
		if got := app.crossOutliner.statusMessage; got != "No recent tasks" {
			t.Fatalf("status = %q, want logical no-task outcome", got)
		}
	})
}
