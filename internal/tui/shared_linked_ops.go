package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// resolveTaskInProject re-fetches a task pointer by ID after a project reload
// may have replaced project.Tasks with fresh objects. Returns nil if not found.
func resolveTaskInProject(project *models.Project, taskID string) *models.Task {
	return project.GetTaskByID(taskID)
}

// handleLinkedTaskPrefixKey handles the "c" prefix dispatch for linked task
// operations. supportUnlink enables cx (unlink) and supportCrossProject enables
// cp (cross-project linking). Returns (handled, cmd).
func handleLinkedTaskPrefixKey(key string, task *models.Task, project *models.Project, supportUnlink, supportCrossProject bool) (bool, tea.Cmd) {
	// Navigate to linked tasks
	if key == "n" {
		return true, func() tea.Msg {
			return ShowLinkedTaskSelectorMsg{Task: task, Project: project}
		}
	}

	// Unlink linked task
	if key == "x" && supportUnlink {
		return true, func() tea.Msg {
			return ShowLinkedTaskSelectorMsg{Task: task, Project: project, ForUnlink: true}
		}
	}

	// Cross-project linking
	if key == "p" && supportCrossProject {
		return true, func() tea.Msg {
			return ShowCrossProjectLinkerMsg{SourceTask: task, SourceProject: project}
		}
	}

	var linkType LinkType
	switch key {
	case "b":
		linkType = LinkTypeBlocker
	case "d":
		linkType = LinkTypeDependent
	case "s":
		linkType = LinkTypeSubtask
	case "f":
		linkType = LinkTypeFollowUp
	default:
		return false, nil
	}
	return true, func() tea.Msg {
		return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: linkType}
	}
}

// handleTaskPrefixMsgKey handles the message-only subset of the "t" prefix
// dispatch: to (edit content), tc (comments), ta (attachments), tl (checklist),
// te (description editor), tm (move to project), td (decisions).
// These keys produce identical messages across ProjectTree and Outliner.
// Returns (handled, cmd).
func handleTaskPrefixMsgKey(key string, task *models.Task, project *models.Project) (bool, tea.Cmd) {
	switch key {
	case "o":
		return true, func() tea.Msg {
			return EditTaskContentMsg{Task: task, Project: project}
		}
	case "c":
		return true, func() tea.Msg {
			return ShowTaskCommentsMsg{Task: task, Project: project}
		}
	case "a":
		return true, func() tea.Msg {
			return ShowTaskAttachmentsMsg{Task: task, Project: project}
		}
	case "l":
		return true, func() tea.Msg {
			return ShowTaskChecklistMsg{Task: task, Project: project}
		}
	case "e":
		return true, func() tea.Msg {
			return ShowDescriptionEditorMsg{Task: task, Project: project}
		}
	case "m":
		return true, func() tea.Msg {
			return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project}
		}
	case "d":
		return true, func() tea.Msg {
			return ShowTaskDecisionsMsg{Task: task, Project: project}
		}
	}
	return false, nil
}

// handleTaskActionPrefixKey handles the "t" prefix keys that need task/project
// and produce identical messages across ProjectTree and Outliner for single-task
// paths: tn/tN (create), ts (section picker), tt (tag selector), tD (delete).
// focusedTask is the subtree focus task (may be nil), used as parent for tn/tN.
// getMultiTasks returns selected tasks when in multi-select mode (nil if not).
// Returns (handled, cmd).
func handleTaskActionPrefixKey(key string, task *models.Task, project *models.Project, focusedTask *models.Task, getMultiTasks func() []*models.Task) (bool, tea.Cmd) {
	multiTasks := func() []*models.Task {
		if getMultiTasks == nil {
			return nil
		}
		return getMultiTasks()
	}
	switch key {
	case "n":
		parentTask := focusedTask
		if parentTask == nil {
			parentTask = task
		}
		return true, func() tea.Msg {
			return CreateTaskMsg{ParentTask: parentTask, SkipTemplatePicker: true}
		}
	case "N":
		parentTask := focusedTask
		if parentTask == nil {
			parentTask = task
		}
		return true, func() tea.Msg {
			return CreateTaskMsg{ParentTask: parentTask, SkipTemplatePicker: false}
		}
	case "s":
		if tasks := multiTasks(); len(tasks) > 0 {
			return true, func() tea.Msg {
				return ShowBulkSectionPickerMsg{Tasks: tasks, Project: project}
			}
		}
		if task != nil {
			return true, func() tea.Msg {
				return ShowSectionPickerMsg{Task: task, Project: project}
			}
		}
		return true, nil
	case "t":
		if tasks := multiTasks(); len(tasks) > 0 {
			return true, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: project}
			}
		}
		if task != nil {
			tags := append([]string{}, task.Tags...)
			return true, func() tea.Msg {
				return ShowTagSelectorMsg{
					Task:        task,
					Project:     project,
					CurrentTags: tags,
				}
			}
		}
		return true, nil
	}
	return false, nil
}

// handleBlockerPrefixKey handles the "b" prefix dispatch for blocker/dependency
// operations: bb (view dependencies), bm (manage dependencies).
// Returns (handled, cmd).
func handleBlockerPrefixKey(key string, task *models.Task, project *models.Project) (bool, tea.Cmd) {
	switch key {
	case "b":
		return true, func() tea.Msg {
			return ShowDependencyViewerMsg{Task: task, Project: project}
		}
	case "m":
		return true, func() tea.Msg {
			return ManageDependenciesMsg{Task: task, Project: project}
		}
	}
	return false, nil
}
