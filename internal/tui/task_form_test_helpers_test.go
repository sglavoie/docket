package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/testutil"
)

func newTestTaskForm() *TaskFormModel {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := newTestConfig()
	return NewTaskFormModelCreate(project, testRuntimeAt(cfg.DataDir(), cfg), nil)
}

func sendFormKey(m *TaskFormModel, key tea.KeyPressMsg) *TaskFormModel {
	next, _ := m.Update(key)
	return next.(*TaskFormModel)
}

func navigateToField(m *TaskFormModel, idx int) *TaskFormModel {
	m.focusIndex = idx
	m.focusTitleInput()
	return m
}

func sendFormMsg(m *TaskFormModel, msg tea.Msg) *TaskFormModel {
	next, _ := m.Update(msg)
	return next.(*TaskFormModel)
}

func navigateToScheduleField(m *TaskFormModel, idx int) *TaskFormModel {
	m.activeTab = formTabSchedule
	m.focusIndex = idx
	m.syncInputFocus()
	return m
}

func navigateToChildrenTab(m *TaskFormModel) *TaskFormModel {
	m.activeTab = formTabChildren
	m.focusIndex = 0
	return m
}

func newSaveableEditForm(t *testing.T) (*TaskFormModel, func()) {
	t.Helper()
	tmpDir, cleanup := testutil.TempProjectDir(t)
	project := testutil.NewTestProjectWithTasks("Save Test")
	project.Path = filepath.Join(tmpDir, "save-edit-project")
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	cfg := newTestConfig()
	if len(project.Tasks) == 0 {
		t.Fatal("test project has no tasks")
	}
	task := project.Tasks[0]
	m := NewTaskFormModelEdit(project, task, testRuntimeAt(cfg.DataDir(), cfg))
	return m, cleanup
}
