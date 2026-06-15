package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) projectTreeProject() *models.Project {
	if m.projectTree != nil && m.projectTree.getProject() != nil {
		return m.projectTree.getProject()
	}
	return m.currentProject
}

func (m *AppModel) projectTreeBulkSelectedTasks() []*models.Task {
	if m.projectTree == nil {
		return nil
	}
	return m.projectTree.getMultiSelectedTasks()
}

func (m *AppModel) projectTreeHasBulkSelection() bool {
	return len(m.projectTreeBulkSelectedTasks()) > 0
}

func (m *AppModel) projectTreeBulkTaskRefs() []TaskProjectRef {
	return taskProjectRefsFromTasks(m.projectTreeBulkSelectedTasks(), m.projectTreeProject())
}

func (m *AppModel) projectTreeClipboardMsg(key string) tea.Msg {
	if m.projectTree == nil {
		return StatusMsg("No task selected")
	}

	handled, cmd := handleClipboardKey(
		key,
		m.selectedTreeTask,
		m.projectTree.getMultiSelectedTasks,
		m.projectTree.clearMultiSelect,
	)
	if handled && cmd != nil {
		return cmd()
	}
	return StatusMsg("No task selected")
}

func (m *AppModel) runProjectTreeCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}

	msg := cmd()
	switch msg := msg.(type) {
	case treeRefreshMsg:
		if m.projectTree != nil {
			_, _ = m.projectTree.Update(msg)
		}
		return nil
	case tea.BatchMsg:
		for _, batchCmd := range msg {
			if nested := m.runProjectTreeCmd(batchCmd); nested != nil {
				return nested
			}
		}
		return nil
	default:
		return msg
	}
}

func (m *AppModel) runProjectTreeBulkDueDatePrefix(key string) tea.Msg {
	cmd, _ := handleDueDatePrefix(key, m.projectTree)
	return m.runProjectTreeCmd(cmd)
}

func (m *AppModel) runProjectTreeBulkStatusTransition(status models.TaskStatus) tea.Msg {
	if m.projectTree == nil {
		return nil
	}
	wasBulk := m.projectTreeHasBulkSelection()
	_, cmd := m.projectTree.handleStatusTransition(status, m.projectTree.visibleNodes())
	if !wasBulk {
		return nil
	}
	return m.runProjectTreeCmd(cmd)
}
