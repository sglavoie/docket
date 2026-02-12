package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

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
// td (description editor), tm (move to project).
// These keys produce identical messages across ProjectTree and Kanban.
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
	case "d":
		return true, func() tea.Msg {
			return ShowDescriptionEditorMsg{Task: task, Project: project}
		}
	case "m":
		return true, func() tea.Msg {
			return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project}
		}
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
