package tui

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// handleEditTaskContent launches external editor for task title and description.
func (m *AppModel) handleEditTaskContent(msg EditTaskContentMsg) (bool, tea.Cmd) {
	if msg.Task == nil {
		return true, nil
	}

	content := fmt.Sprintf("# %s\n\n<!-- Edit the title above and description below. -->\n\n%s", msg.Task.Title, msg.Task.Description)
	tmpPath, err := external.CreateTempFileWithContent(content, "tt-edit-", ".md")
	if err != nil {
		return true, errorCmd("%s", err)
	}

	editor, editorErr := external.ResolveEditor("")
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return true, errorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

	task := msg.Task
	project := msg.Project
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return TaskContentEditedMsg{Task: task, Project: project, Err: err}
		}
		content, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return TaskContentEditedMsg{Task: task, Project: project, Err: readErr}
		}
		newTitle, newDesc := parseTaskContent(string(content), task)
		return TaskContentEditedMsg{
			Task:           task,
			Project:        project,
			NewTitle:       newTitle,
			NewDescription: newDesc,
		}
	})
}

// handleTaskContentEdited handles result from external editor.
func (m *AppModel) handleTaskContentEdited(msg TaskContentEditedMsg) (bool, tea.Cmd) {
	if msg.Err != nil {
		return true, errorCmd("editor failed: %w", msg.Err)
	}

	if msg.Task != nil {
		titleChanged := msg.NewTitle != msg.Task.Title
		descChanged := msg.NewDescription != msg.Task.Description
		if titleChanged || descChanged {
			// Validate the new title (matches CLI task edit behavior)
			if titleChanged {
				if err := models.ValidateTaskTitle(msg.NewTitle); err != nil {
					return true, errorCmd("invalid title: %w", err)
				}
			}
			if msg.Project != nil {
				req := app.EditRequest{Locator: taskIdentityFor(msg.Task, msg.Project).Locator()}
				if titleChanged {
					req.Title = &msg.NewTitle
				}
				if descChanged {
					req.Description = &msg.NewDescription
				}
				res, err := m.appService.Edit(req)
				if err != nil {
					return true, errorCmd("%s", err)
				}
				m.reloadProjectByID(res.ProjectID)
				return true, m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: "Task updated"})
			}
		}
	}

	return true, nil
}
