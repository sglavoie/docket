package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleAddInput handles keyboard input in add mode.
func (m *FocusBoardModel) handleAddInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Let autocomplete handle keys first when active
	if m.handleBoardAutocompleteKey(key) {
		return m, nil
	}

	switch key {
	case "esc":
		m.addMode = false
		m.addInput.Blur()
		m.ac.Clear()
		return m, nil
	case "enter":
		if m.ac.IsActive() {
			return m, nil
		}
		value := strings.TrimSpace(m.addInput.Value())
		if value == "" {
			m.addMode = false
			m.addInput.Blur()
			m.ac.Clear()
			return m, nil
		}
		return m.createItemFromInput(value)
	default:
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		m.refreshBoardAutocomplete()
		return m, cmd
	}
}

// handleEditInput handles keyboard input in edit mode.
func (m *FocusBoardModel) handleEditInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editMode = false
		m.editInput.Blur()
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.editInput.Value())
		m.editMode = false
		m.editInput.Blur()
		if value == "" {
			return m, nil
		}
		if err := models.ValidateTaskTitle(value); err != nil {
			m.setStatus(err.Error())
			return m, nil
		}
		if m.editIdx >= 0 && m.editIdx < len(m.filtered) {
			row := m.filtered[m.editIdx]
			row.Project.MutateTask(row.Task.ID, func(t *models.Task) {
				t.Title = value
			})
			opts := BuildWorkflowOpts(m.projectService, row.Project, m.getConfig(), m.configManager, nil, "Edit Title", row.Task.Title)
			if err := persistWithWorkflow(opts); err != nil {
				m.setStatus("Failed to save: " + err.Error())
				return m, nil
			}
			return m, func() tea.Msg {
				return SaveCompleteMsg{SuccessMessage: "Title updated"}
			}
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
}

// createItemFromInput parses the add input and creates a task.
func (m *FocusBoardModel) createItemFromInput(input string) (tea.Model, tea.Cmd) {
	m.addMode = false
	m.addInput.Blur()
	m.ac.Clear()

	result := parseBoardAddInput(input)

	// Determine target project
	var targetProject *models.Project

	if result.projectSlug != "" {
		for _, p := range m.acProjectCache {
			if p.Slug == result.projectSlug {
				targetProject = p
				break
			}
		}
	}

	if targetProject == nil {
		// Use action inbox as default
		project, created := m.config.EnsureActionInboxProject()
		targetProject = project
		if created {
			// Ensure project directory exists and save config
			if err := storage.CreateProjectDir(targetProject); err != nil {
				m.setStatus("Failed to create action inbox: " + err.Error())
				return m, nil
			}
			if err := storage.SaveProjectLocked(targetProject); err != nil {
				m.setStatus("Failed to save action inbox: " + err.Error())
				return m, nil
			}
			if m.configManager != nil {
				if err := m.configManager.Save(m.config); err != nil {
					logging.Warn("failed to save config: %v", err)
				}
			}
		}
	}

	if err := m.projectService.Load(targetProject); err != nil {
		m.setStatus("Failed to load project: " + err.Error())
		return m, nil
	}

	task, err := models.NewTask(uuid.New().String(), targetProject.GetNextShortID(), models.TaskTypeTask, result.title)
	if err != nil {
		m.setStatus("Invalid task: " + err.Error())
		return m, nil
	}
	if result.source != "" {
		task.Source = result.source
	}
	if result.url != "" {
		task.URL = result.url
	}
	if result.priority >= 1 && result.priority <= 5 {
		task.Priority = result.priority
	}
	if len(result.tags) > 0 {
		task.Tags = result.tags
	}

	targetProject.Tasks = append(targetProject.Tasks, task)
	targetProject.BuildTaskTree()

	opts := BuildWorkflowOpts(m.projectService, targetProject, m.getConfig(), m.configManager, nil, "Add Task", task.Title)
	if err := persistWithWorkflow(opts); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return m, nil
	}

	return m, func() tea.Msg {
		return SaveCompleteMsg{SuccessMessage: fmt.Sprintf("Added #%d: %s", task.ShortID, task.Title)}
	}
}
