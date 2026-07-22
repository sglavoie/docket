package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// inboxOpsView abstracts the view-specific bits needed by shared inbox
// operations. Both InboxProcessorModel and GlobalInboxProcessorModel implement this.
type inboxOpsView interface {
	getInboxTaskRef() (task *models.Task, project *models.Project)
	inboxSave() tea.Cmd
	inboxReload() tea.Cmd
	inboxCaptureSnapshot(task *models.Task) *inboxSnapshot
	inboxRecordAction(desc string, snapshot *inboxSnapshot)
	setInboxStatus(string)
}

// --- "Open" operations: construct and return a message ---

func inboxOpenSectionPicker(v inboxOpsView) tea.Cmd {
	task, project := v.getInboxTaskRef()
	if task == nil {
		return nil
	}
	return func() tea.Msg {
		return ShowSectionPickerMsg{Task: task, Project: project}
	}
}

func inboxOpenTaskEditor(v inboxOpsView) tea.Cmd {
	task, project := v.getInboxTaskRef()
	if task == nil {
		return nil
	}
	return func() tea.Msg {
		return EditTaskMsg{Task: task, Project: project}
	}
}

func inboxOpenDueDateInput(v inboxOpsView) tea.Cmd {
	task, project := v.getInboxTaskRef()
	if task == nil {
		return nil
	}
	return func() tea.Msg {
		return SetDueDateMsg{Task: task, Project: project}
	}
}

func inboxOpenSnoozeInput(v inboxOpsView) tea.Cmd {
	task, project := v.getInboxTaskRef()
	if task == nil {
		return nil
	}
	return func() tea.Msg {
		return SetSnoozeDateMsg{Task: task, Project: project}
	}
}

func inboxOpenDependencySelector(v inboxOpsView) tea.Cmd {
	task, project := v.getInboxTaskRef()
	if task == nil {
		return nil
	}
	return func() tea.Msg {
		return ManageDependenciesMsg{Task: task, Project: project}
	}
}

func inboxOpenSubtaskCreator(v inboxOpsView) tea.Cmd {
	task, _ := v.getInboxTaskRef()
	if task == nil {
		return nil
	}
	return func() tea.Msg {
		return CreateTaskMsg{ParentTask: task}
	}
}

func inboxOpenTagSelector(v inboxOpsView) tea.Cmd {
	task, project := v.getInboxTaskRef()
	if task == nil {
		return nil
	}
	return func() tea.Msg {
		return ShowTagSelectorMsg{
			Task:        task,
			Project:     project,
			CurrentTags: append([]string{}, task.Tags...),
		}
	}
}

// --- Simple "handle done" operations ---

func inboxHandleTaskUpdated(v inboxOpsView) tea.Cmd {
	v.setInboxStatus("Task updated")
	return v.inboxReload()
}

func inboxHandleDueDateChanged(v inboxOpsView) tea.Cmd {
	v.setInboxStatus("Due date updated")
	return v.inboxReload()
}

func inboxHandleDependencySelectorDone(v inboxOpsView) tea.Cmd {
	v.setInboxStatus("Dependencies updated")
	return v.inboxReload()
}

// --- Tag selector handling (shared logic, view-specific save) ---

// inboxHandleTagSelectorDone processes tag selector completion. Returns true
// if the operation was handled (caller should save), false if canceled/invalid.
func inboxHandleTagSelectorDone(v inboxOpsView, msg TagSelectorDoneMsg) (handled bool, cmd tea.Cmd) {
	if msg.Canceled {
		v.setInboxStatus("Tag editing canceled")
		return false, nil
	}

	task, _ := v.getInboxTaskRef()
	if task == nil {
		v.setInboxStatus("Error: no current task")
		return false, nil
	}
	if task.ID != msg.Task.ID {
		v.setInboxStatus(fmt.Sprintf("Error: task ID mismatch (current: %s, msg: %s)", task.ID, msg.Task.ID))
		return false, nil
	}

	_, project := v.getInboxTaskRef()
	current := project.GetTaskByID(task.ID)
	if current == nil {
		v.setInboxStatus("Error: task no longer exists in project")
		return false, nil
	}

	snapshot := v.inboxCaptureSnapshot(current)
	if !project.MutateTask(task.ID, func(t *models.Task) {
		t.Tags = msg.SelectedTags
	}) {
		v.setInboxStatus("Error: task no longer exists in project")
		return false, nil
	}

	v.inboxRecordAction("tags updated", snapshot)
	if len(msg.SelectedTags) == 0 {
		v.setInboxStatus("Tags cleared")
	} else {
		v.setInboxStatus(fmt.Sprintf("Tags set: %s", strings.Join(msg.SelectedTags, ", ")))
	}

	return true, v.inboxSave()
}

// --- Section picker handling (shared logic, view-specific post-processing) ---

// inboxHandleSectionPickerCore processes section picker completion. Returns true
// if the section was applied (caller should handle save + any view-specific logic).
func inboxHandleSectionPickerCore(v inboxOpsView, msg SectionPickerDoneMsg) bool {
	if msg.Canceled {
		return false
	}

	task, _ := v.getInboxTaskRef()
	if task == nil || task.ID != msg.Task.ID {
		return false
	}

	_, project := v.getInboxTaskRef()
	current := project.GetTaskByID(task.ID)
	if current == nil {
		v.setInboxStatus("Error: task no longer exists in project")
		return false
	}

	snapshot := v.inboxCaptureSnapshot(current)
	if !project.MutateTask(task.ID, func(t *models.Task) {
		t.SetSection(msg.Section)
	}) {
		v.setInboxStatus("Error: task no longer exists in project")
		return false
	}

	sectionName := msg.Section
	if sectionName == "" {
		sectionName = "(no section)"
	}

	v.inboxRecordAction(fmt.Sprintf("section set to %s", sectionName), snapshot)
	v.setInboxStatus(fmt.Sprintf("Section set to '%s'", sectionName))
	return true
}
