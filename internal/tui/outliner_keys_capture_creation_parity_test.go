package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/testutil"
)

// crossOutlinerWithConfig wraps crossOutlinerWithTwoProjects with a config
// populated so QuickCapture (which reads m.config.GetActiveProjects()) does
// not nil-dereference.
func crossOutlinerWithConfig(t *testing.T) (*OutlinerModel, *models.Project, *models.Project, *models.Task, *models.Task) {
	t.Helper()
	m, projA, projB, taskA, taskB := crossOutlinerWithTwoProjects(t)
	cfg := testutil.NewTestConfig()
	cfg.Projects = []*models.Project{projA, projB}
	m.config = cfg
	return m, projA, projB, taskA, taskB
}

func TestPrimaryViewKey_QuickCapture_CrossUsesCursorProject(t *testing.T) {
	m, projA, projB, _, _ := crossOutlinerWithConfig(t)

	m.cursor = 1 // task in projA
	msg := invokePrimaryViewKey(t, m, "i")
	show, ok := msg.(ShowQuickCaptureMsg)
	if !ok {
		t.Fatalf("expected ShowQuickCaptureMsg, got %T", msg)
	}
	if show.Project != projA {
		t.Fatalf("Project = %v, want %v", show.Project, projA)
	}

	m.cursor = 4 // task in projB
	msg = invokePrimaryViewKey(t, m, "i")
	show, _ = msg.(ShowQuickCaptureMsg)
	if show.Project != projB {
		t.Fatalf("Project = %v, want %v", show.Project, projB)
	}
}

func TestPrimaryViewKey_QuickCapture_CrossOnHeaderSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithConfig(t)
	m.cursor = 0 // header

	if msg := invokePrimaryViewKey(t, m, "i"); msg != nil {
		t.Fatalf("expected nil message on header, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestPrimaryViewKey_QuickCapture_CrossOnSeparatorSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithConfig(t)
	m.cursor = 2 // separator

	if msg := invokePrimaryViewKey(t, m, "i"); msg != nil {
		t.Fatalf("expected nil message on separator, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestPrimaryViewKey_QuickCapture_SingleProjectUnchanged(t *testing.T) {
	proj := &models.Project{ID: "p", Name: "P"}
	m := minimalOutliner()
	m.project = proj
	m.config = testutil.NewTestConfig()
	m.config.Projects = []*models.Project{proj}
	m.rows = []outlinerRow{{IsSectionHeader: true, SectionName: models.SectionInbox}}
	m.cursor = 0 // header, but single-project mode keeps m.project routing

	msg := invokePrimaryViewKey(t, m, "i")
	show, ok := msg.(ShowQuickCaptureMsg)
	if !ok {
		t.Fatalf("expected ShowQuickCaptureMsg, got %T", msg)
	}
	if show.Project != proj {
		t.Fatalf("Project = %v, want %v", show.Project, proj)
	}
}

func TestPrimaryViewKey_BulkCapture_CrossUsesCursorProject(t *testing.T) {
	m, projA, projB, _, _ := crossOutlinerWithConfig(t)

	m.cursor = 1
	msg := invokePrimaryViewKey(t, m, "I")
	show, ok := msg.(ShowBulkCaptureMsg)
	if !ok {
		t.Fatalf("expected ShowBulkCaptureMsg, got %T", msg)
	}
	if show.Project != projA {
		t.Fatalf("Project = %v, want %v", show.Project, projA)
	}

	m.cursor = 4
	msg = invokePrimaryViewKey(t, m, "I")
	show, _ = msg.(ShowBulkCaptureMsg)
	if show.Project != projB {
		t.Fatalf("Project = %v, want %v", show.Project, projB)
	}
}

func TestPrimaryViewKey_BulkCapture_CrossOnHeaderSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithConfig(t)
	m.cursor = 0

	if msg := invokePrimaryViewKey(t, m, "I"); msg != nil {
		t.Fatalf("expected nil message on header, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestPrimaryViewKey_BulkCapture_SingleProjectUnchanged(t *testing.T) {
	proj := &models.Project{ID: "p", Name: "P"}
	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{{IsSectionHeader: true, SectionName: models.SectionInbox}}
	m.cursor = 0

	msg := invokePrimaryViewKey(t, m, "I")
	show, ok := msg.(ShowBulkCaptureMsg)
	if !ok {
		t.Fatalf("expected ShowBulkCaptureMsg, got %T", msg)
	}
	if show.Project != proj {
		t.Fatalf("Project = %v, want %v", show.Project, proj)
	}
}

func TestHandleTaskPrefix_CrossNewTaskUsesCursorTaskAndProject(t *testing.T) {
	m, projA, projB, taskA, taskB := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	_, cmd := m.handleTaskPrefix("n", nil)
	if cmd == nil {
		t.Fatal("expected cmd from tn on task row")
	}
	msg := cmd()
	create, ok := msg.(CreateTaskMsg)
	if !ok {
		t.Fatalf("expected CreateTaskMsg, got %T", msg)
	}
	if create.ParentTask != taskA {
		t.Fatalf("ParentTask = %v, want %v", create.ParentTask, taskA)
	}
	if create.Project != projA {
		t.Fatalf("Project = %v, want %v", create.Project, projA)
	}
	if !create.SkipTemplatePicker {
		t.Fatal("tn must skip template picker")
	}

	m.cursor = 4
	_, cmd = m.handleTaskPrefix("n", nil)
	msg = cmd()
	create, _ = msg.(CreateTaskMsg)
	if create.ParentTask != taskB || create.Project != projB {
		t.Fatalf("got (%v, %v), want (%v, %v)", create.ParentTask, create.Project, taskB, projB)
	}
}

func TestHandleTaskPrefix_CrossNewTaskWithTemplateUsesCursorTaskAndProject(t *testing.T) {
	m, projA, _, taskA, _ := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	_, cmd := m.handleTaskPrefix("N", nil)
	if cmd == nil {
		t.Fatal("expected cmd from tN on task row")
	}
	msg := cmd()
	create, ok := msg.(CreateTaskMsg)
	if !ok {
		t.Fatalf("expected CreateTaskMsg, got %T", msg)
	}
	if create.ParentTask != taskA || create.Project != projA {
		t.Fatalf("got (%v, %v), want (%v, %v)", create.ParentTask, create.Project, taskA, projA)
	}
	if create.SkipTemplatePicker {
		t.Fatal("tN must NOT skip template picker")
	}
}

func TestHandleTaskPrefix_CrossNewTaskOnHeaderSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 0 // header

	_, cmd := m.handleTaskPrefix("n", nil)
	if cmd != nil {
		t.Fatalf("expected nil cmd on header, got %T", cmd())
	}
	if m.statusMessage != statusNoTaskSelected {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoTaskSelected)
	}
}

func TestHandleTaskPrefix_CrossNewTaskOnSeparatorSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 2 // separator

	_, cmd := m.handleTaskPrefix("N", nil)
	if cmd != nil {
		t.Fatalf("expected nil cmd on separator, got %T", cmd())
	}
	if m.statusMessage != statusNoTaskSelected {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoTaskSelected)
	}
}

// TestHandleCreateTask_PrefersMsgProject ensures handleCreateTask routes the
// task form to msg.Project when set, rather than m.currentProject. This is
// the explicit-project path the cross-project outliner uses.
func TestHandleCreateTask_PrefersMsgProject(t *testing.T) {
	app, currentProject := newTestAppModelWithProject()
	target := testutil.NewTestProjectWithTasks("Other Project")
	app.config.Projects = append(app.config.Projects, target)

	msg := CreateTaskMsg{Project: target, SkipTemplatePicker: true}
	handled, _ := app.handleTaskMessages(msg)
	if !handled {
		t.Fatal("CreateTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskForm {
		t.Fatalf("currentView should be TaskForm, got %v", app.currentView())
	}
	if app.currentProject != currentProject {
		t.Fatal("currentProject must remain unchanged when msg.Project is set")
	}
}

// TestHandleCreateTask_TemplatePickerRoutesMsgProject verifies that when the
// template picker is shown (enabled templates + SkipTemplatePicker=false),
// handleCreateTask stores msg.Project in pendingCreateTaskProject so the
// subsequent task form is created against the explicit project rather than
// m.currentProject.
func TestHandleCreateTask_TemplatePickerRoutesMsgProject(t *testing.T) {
	app, currentProject := newTestAppModelWithProject()
	target := testutil.NewTestProjectWithTasks("Other Project")
	app.config.Projects = append(app.config.Projects, target)
	app.config.TaskTemplates = []template.TaskTemplate{
		{ID: "t1", Name: "Bug", Enabled: true, Type: models.TaskTypeBug},
	}

	msg := CreateTaskMsg{Project: target, SkipTemplatePicker: false}
	handled, _ := app.handleTaskMessages(msg)
	if !handled {
		t.Fatal("CreateTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskTemplatePicker {
		t.Fatalf("currentView should be TaskTemplatePicker, got %v", app.currentView())
	}
	if app.pendingCreateTaskProject != target {
		t.Errorf("pendingCreateTaskProject = %v, want %v (msg.Project)", app.pendingCreateTaskProject, target)
	}
	if app.currentProject != currentProject {
		t.Error("currentProject must remain unchanged when msg.Project is set")
	}
}

// TestHandleCreateTask_FallsBackToCurrentProject is the legacy path: when
// msg.Project is nil, handleCreateTask must still use m.currentProject.
func TestHandleCreateTask_FallsBackToCurrentProject(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := CreateTaskMsg{SkipTemplatePicker: true}
	handled, _ := app.handleTaskMessages(msg)
	if !handled {
		t.Fatal("CreateTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskForm {
		t.Fatalf("currentView should be TaskForm, got %v", app.currentView())
	}
	if app.currentProject != project {
		t.Fatal("currentProject should still be set")
	}
}
