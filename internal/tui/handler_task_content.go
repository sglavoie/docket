package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// handleEditTaskContent launches external editor for task title and description.
func (m *AppModel) handleEditTaskContent(msg EditTaskContentMsg) (bool, tea.Cmd) {
	if msg.Task == nil {
		return true, nil
	}

	tmpFile, err := os.CreateTemp("", "tt-edit-*.md")
	if err != nil {
		return true, errorCmd("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	content := fmt.Sprintf("# %s\n\n<!-- Edit the title above and description below. -->\n\n%s", msg.Task.Title, msg.Task.Description)
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	editor := external.ResolveEditor(m.config.Editor)
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
			if msg.Project != nil {
				// Re-lookup task in project to avoid stale pointer after concurrent reload
				if !msg.Project.MutateTask(msg.Task.ID, func(t *models.Task) {
					t.Title = msg.NewTitle
					t.Description = msg.NewDescription
				}) {
					return true, errorCmd("task no longer exists in project")
				}

				// Refresh task editor view first if it's open
				if m.currentView() == ViewTaskEditor {
					current := msg.Project.GetTaskByID(msg.Task.ID)
					taskEditor := NewTaskEditorModel(msg.Project, current, m.config, m.configManager)
					taskEditor.SetSize(m.width, m.height-2)
					m.views.Register(ViewTaskEditor, taskEditor)
				}
				return true, tea.Batch(
					startLoading("Saving..."),
					SaveProjectAsync(msg.Project, "Task updated"),
				)
			}
		}
	}

	if m.currentView() == ViewTaskEditor && msg.Task != nil {
		current := msg.Project.GetTaskByID(msg.Task.ID)
		if current != nil {
			taskEditor := NewTaskEditorModel(msg.Project, current, m.config, m.configManager)
			taskEditor.SetSize(m.width, m.height-2)
			m.views.Register(ViewTaskEditor, taskEditor)
		}
	}
	return true, nil
}

// handleDescriptionEditorDone handles result from the description text editor popup.
func (m *AppModel) handleDescriptionEditorDone(msg DescriptionEditorDoneMsg) (bool, tea.Cmd) {
	if msg.Canceled {
		return true, nil
	}

	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}

	// Check if description actually changed
	if msg.NewDescription == msg.Task.Description {
		return true, nil
	}

	// Update the task description
	if !msg.Project.MutateTask(msg.Task.ID, func(t *models.Task) {
		t.Description = msg.NewDescription
	}) {
		return true, errorCmd("task no longer exists in project")
	}

	// Refresh project tree and save
	m.refreshProjectTree()
	if m.outliner != nil {
		m.outliner.rebuildRows()
	}
	return true, tea.Batch(
		startLoading("Saving..."),
		SaveProjectAsync(msg.Project, "Description updated"),
	)
}
