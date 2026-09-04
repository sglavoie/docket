package taskform

import (
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

func navigateToScheduleField(m *TaskFormModel) *TaskFormModel {
	m.activeTab = formTabSchedule
	m.focusIndex = 0
	m.syncInputFocus()
	return m
}

func navigateToChildrenTab(m *TaskFormModel) *TaskFormModel {
	m.activeTab = formTabChildren
	m.focusIndex = 0
	return m
}
