package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectTaskJumperMatchesProjectAndSelectsProject(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha Project", Slug: "alpha"}
	alphaTask, _ := models.NewTask("shared", 1, models.TaskTypeTask, "Shared title")
	alpha.Tasks = []*models.Task{alphaTask}

	beta := &models.Project{ID: "beta", Name: "Beta Project", Slug: "beta"}
	betaTask, _ := models.NewTask("shared", 2, models.TaskTypeTask, "Shared title")
	beta.Tasks = []*models.Task{betaTask}

	m := NewCrossProjectTaskJumperModel([]*models.Project{alpha, beta})
	m.searchInput.SetValue("beta")
	m.filterTasks()

	if len(m.filtered) != 1 {
		t.Fatalf("filtered count = %d, want 1", len(m.filtered))
	}
	if m.filtered[0].project != beta {
		t.Fatalf("filtered project = %v, want beta", m.filtered[0].project)
	}

	_, cmd := m.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected enter command")
	}
	rawMsg := cmd()
	msg, ok := rawMsg.(TaskJumperSelectMsg)
	if !ok {
		t.Fatalf("message = %T, want TaskJumperSelectMsg", rawMsg)
	}
	if msg.Task != betaTask {
		t.Errorf("selected task = %v, want beta task", msg.Task)
	}
	if msg.Project != beta {
		t.Errorf("selected project = %v, want beta", msg.Project)
	}
}

func TestCrossProjectOutlinerJumpKeyOpensCrossProjectJumper(t *testing.T) {
	app := newTestAppModel()
	m := NewOutlinerModel(nil, app.configManager, app.runtime)
	project := &models.Project{ID: "project", Name: "Project", Path: t.TempDir()}
	task, _ := models.NewTask("task", 1, models.TaskTypeTask, "Task")
	project.Tasks = []*models.Task{task}
	m.loadedProjects = []*models.Project{project}

	_, cmd := m.Update(syntheticKey('\\'))
	if cmd == nil {
		t.Fatal("expected jump key command")
	}
	rawMsg := cmd()
	msg, ok := rawMsg.(ShowTaskJumperMsg)
	if !ok {
		t.Fatalf("message = %T, want ShowTaskJumperMsg", rawMsg)
	}
	if !msg.CrossProject {
		t.Fatal("CrossProject = false, want true")
	}
	if len(msg.Projects) != 1 || msg.Projects[0] != project {
		t.Fatalf("Projects = %v, want [%v]", msg.Projects, project)
	}
}

func TestJumpToTaskCommandAvailableForCrossProjectOutliner(t *testing.T) {
	app := newTestAppModel()
	project := &models.Project{ID: "cross-proj", Name: "Cross", Path: t.TempDir()}
	task, _ := models.NewTask("cross-task", 1, models.TaskTypeTask, "Cross task")
	project.Tasks = []*models.Task{task}

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = append(app.viewStack, ViewCrossProjectOutliner, ViewCommandPalette)

	cmd := findCommand(app.filterCommands(), "jump to task")
	if cmd == nil {
		t.Fatal("expected jump to task command")
	}
	foundCrossView := slices.Contains(cmd.Views, ViewCrossProjectOutliner)
	if !foundCrossView {
		t.Fatalf("jump to task views = %v, want ViewCrossProjectOutliner", cmd.Views)
	}

	rawMsg := cmd.Handler()
	msg, ok := rawMsg.(ShowTaskJumperMsg)
	if !ok {
		t.Fatalf("handler message = %T, want ShowTaskJumperMsg", rawMsg)
	}
	if !msg.CrossProject {
		t.Fatal("CrossProject = false, want true")
	}
	if len(msg.Projects) != 1 || msg.Projects[0] != project {
		t.Fatalf("Projects = %v, want [%v]", msg.Projects, project)
	}
}

func TestCrossProjectNavigateToTaskUsesProjectIDForDuplicateTaskIDs(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha"}
	alphaTask, _ := models.NewTask("shared", 1, models.TaskTypeTask, "Alpha task")
	alpha.Tasks = []*models.Task{alphaTask}

	beta := &models.Project{ID: "beta", Name: "Beta"}
	betaTask, _ := models.NewTask("shared", 1, models.TaskTypeTask, "Beta task")
	beta.Tasks = []*models.Task{betaTask}

	m := minimalOutliner()
	m.loadedProjects = []*models.Project{alpha, beta}
	m.rows = []outlinerRow{
		{task: alphaTask, ProjectID: alpha.ID},
		{task: betaTask, ProjectID: beta.ID},
	}

	m.NavigateToTaskInProject(betaTask, beta)

	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.cursor)
	}
}

func TestCrossProjectNavigateToTaskExpandsCollapsedSectionAndAncestors(t *testing.T) {
	parentID := "parent"
	parent := makeTask(parentID, "Parent", models.SectionInbox)
	child := makeTask("child", "Child", models.SectionInbox)
	child.ParentID = &parentID

	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{parent, child}}
	project.BuildTaskTree()

	m := minimalOutliner()
	m.activePerspective = PerspectiveAll
	m.loadedProjects = []*models.Project{project}
	m.collapsedSections["_xp:"+models.SectionInbox] = true
	parentIdentity := TaskIdentity{ProjectID: project.ID, TaskID: parent.ID}
	m.expandedNodes[parentIdentity] = false
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox, CollapseKey: "_xp:" + models.SectionInbox},
	}

	m.NavigateToTaskInProject(child, project)

	if m.collapsedSections["_xp:"+models.SectionInbox] {
		t.Fatal("inbox section remained collapsed")
	}
	if !m.expandedNodes[parentIdentity] {
		t.Fatal("parent remained folded")
	}
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		t.Fatalf("cursor = %d outside rows len %d", m.cursor, len(m.rows))
	}
	row := m.rows[m.cursor]
	if row.task == nil || row.task.ID != child.ID || row.ProjectID != project.ID {
		t.Fatalf("cursor row = %+v, want child in project", row)
	}
}
