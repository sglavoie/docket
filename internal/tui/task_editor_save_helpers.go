package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// saveTask saves the current task changes to the project.
func (m *TaskEditorModel) saveTask() error {
	title := strings.ReplaceAll(m.titleInput.Value(), "\n", " ")
	title = strings.TrimSpace(title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return err
	}

	if err := m.parseDueDate(); err != nil {
		return err
	}

	bugAttrs, ideaAttrs, taskAttrs, storyAttrs, epicAttrs := m.buildTaskAttrs()

	m.project.MutateTask(m.task.ID, func(task *models.Task) {
		task.Title = title
		task.Type = m.taskType
		task.Priority = m.priority
		task.ParentID = m.parentID
		task.SetSection(m.section)
		task.DueDate = m.dueDate
		task.Tags = append([]string{}, m.tags...)
		task.Active = m.active
		task.Dimmed = m.dimmed
		task.Recurrence = m.recurrence
		task.URL = strings.TrimSpace(m.urlInput)
		task.Source = strings.TrimSpace(m.sourceInput)

		task.BugAttrs = bugAttrs
		task.IdeaAttrs = ideaAttrs
		task.TaskAttrs = taskAttrs
		task.StoryAttrs = storyAttrs
		task.EpicAttrs = epicAttrs

		if m.pinned != task.Pinned {
			task.Pinned = m.pinned
			if m.pinned {
				now := time.Now()
				task.PinnedAt = &now
			} else {
				task.PinnedAt = nil
			}
		}
	})

	m.project.BuildTaskTree()

	if err := saveProjectSync(m.project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	return nil
}
