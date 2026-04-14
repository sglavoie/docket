package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskCreatorModel) createTask() tea.Cmd {
	// All mutations run synchronously in the Update() goroutine to avoid
	// data races with View() reading project.Tasks concurrently.

	// Replace newlines with spaces since titles should be single logical lines
	title := strings.ReplaceAll(m.titleInput.Value(), "\n", " ")
	title = strings.TrimSpace(title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}

	// Parse due date before saving
	if err := m.parseDueDate(); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}

	now := time.Now()

	// Build type-specific attributes
	bugAttrs, ideaAttrs, taskAttrs, storyAttrs, epicAttrs := m.buildTaskAttrs()

	// Create task
	task := &models.Task{
		ID:          uuid.New().String(),
		ShortID:     m.project.GetNextShortID(),
		Type:        m.taskType,
		Title:       title,
		Description: strings.TrimSpace(m.descValue),
		Status:      models.StatusTodo,
		Priority:    m.priority,
		ParentID:    m.parentID,
		DueDate:     m.dueDate,
		Pinned:      m.pinned,
		Active:      m.active,
		Dimmed:      m.dimmed,
		Recurrence:  m.recurrence,
		BugAttrs:    bugAttrs,
		IdeaAttrs:   ideaAttrs,
		TaskAttrs:   taskAttrs,
		StoryAttrs:  storyAttrs,
		EpicAttrs:   epicAttrs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Route section to correct field (GTD → Section, custom → CustomSection)
	task.SetSection(m.section)

	// Set PinnedAt if pinned
	if m.pinned {
		task.PinnedAt = &now
	}

	// Set tags from the selector
	task.Tags = append([]string{}, m.tags...)

	// Add task to project
	m.project.Tasks = append(m.project.Tasks, task)

	// Build checklist from form-edited checklist items
	for _, item := range m.checklist {
		text := strings.TrimSpace(item.Value())
		if text != "" {
			task.Checklist = append(task.Checklist, models.ChecklistItem{
				Text: text,
				Done: false,
			})
		}
	}

	// Create subtasks from form (Epic/Story only)
	if m.currentTypeCanHaveChildren() {
		allowedTypes := m.getAllowedSubtaskTypes()
		for _, st := range m.subtasks {
			subtaskTitle := strings.TrimSpace(st.titleInput.Value())
			// Skip optional subtasks with empty titles
			if subtaskTitle == "" {
				if st.optional {
					continue
				}
				// Non-optional subtasks with empty titles are skipped too
				continue
			}

			// Get the subtask type, clamping to valid range
			typeCursor := st.typeCursor
			if typeCursor >= len(allowedTypes) {
				typeCursor = 0
			}

			subtask := &models.Task{
				ID:        uuid.New().String(),
				ShortID:   m.project.GetNextShortID(),
				Type:      allowedTypes[typeCursor],
				Title:     subtaskTitle,
				Status:    models.StatusTodo,
				Priority:  m.priority, // Inherit from parent
				ParentID:  &task.ID,
				CreatedAt: now,
				UpdatedAt: now,
			}
			subtask.SetSection(m.section) // Inherit from parent

			// Inherit tags from parent
			subtask.Tags = append([]string{}, m.tags...)

			m.project.Tasks = append(m.project.Tasks, subtask)
		}
	}

	// Increment template use count if from template
	if m.fromTemplate != nil && m.config != nil && m.configManager != nil {
		m.config.IncrementTemplateUseCount(m.fromTemplate.ID)
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
	}

	m.project.BuildTaskTree()
	m.project.ReorderRootTasksByCompletion()

	// Save project with backup for undo support
	if err := saveProjectSync(m.project); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)} }
	}

	return func() tea.Msg { return TaskCreatedMsg{Task: task} }
}
