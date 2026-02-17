package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleAddInput handles keyboard input in add mode.
func (m *FocusBoardModel) handleAddInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Let autocomplete handle keys first when active
	if m.handleBoardAutocompleteKey(key) {
		return m, nil
	}

	switch key {
	case "esc":
		m.addMode = false
		m.addInput.Blur()
		m.resetBoardAutocomplete()
		return m, nil
	case "enter":
		if m.acMode != acModeNone {
			return m, nil
		}
		value := strings.TrimSpace(m.addInput.Value())
		if value == "" {
			m.addMode = false
			m.addInput.Blur()
			m.resetBoardAutocomplete()
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
func (m *FocusBoardModel) handleEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if m.editIdx >= 0 && m.editIdx < len(m.filtered) {
			row := m.filtered[m.editIdx]
			row.Project.MutateTask(row.Task.ID, func(t *models.Task) {
				t.Title = value
			})
			if err := saveProjectSync(row.Project); err != nil {
				m.statusMessage = "Failed to save: " + err.Error()
			} else {
				m.statusMessage = "Title updated"
				m.loadItems()
				m.clampCursor()
			}
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
}

// handleSearchInput processes keyboard input while in search mode.
func (m *FocusBoardModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.searchInput.Blur()
		m.searchQuery = ""
		m.searchInput.SetValue("")
		m.applyFilter()
		m.clampCursor()
		return m, nil

	case "enter":
		m.searchMode = false
		m.searchInput.Blur()
		m.searchQuery = strings.TrimSpace(m.searchInput.Value())
		m.applyFilter()
		m.clampCursor()
		return m, nil

	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.searchQuery = strings.TrimSpace(m.searchInput.Value())
		m.applyFilter()
		m.clampCursor()
		return m, cmd
	}
}

// createItemFromInput parses the add input and creates an Action task.
func (m *FocusBoardModel) createItemFromInput(input string) (tea.Model, tea.Cmd) {
	m.addMode = false
	m.addInput.Blur()
	m.resetBoardAutocomplete()

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
				m.statusMessage = "Failed to create action inbox: " + err.Error()
				return m, nil
			}
			if err := storage.SaveProject(targetProject); err != nil {
				m.statusMessage = "Failed to save action inbox: " + err.Error()
				return m, nil
			}
			if m.configManager != nil {
				_ = m.configManager.Save(m.config)
			}
		}
	}

	if err := m.projectService.Load(targetProject); err != nil {
		m.statusMessage = "Failed to load project: " + err.Error()
		return m, nil
	}

	task := models.NewTask(uuid.New().String(), targetProject.GetNextShortID(), models.TaskTypeAction, result.title)
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

	if err := saveProjectSync(targetProject); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
		return m, nil
	}

	m.statusMessage = fmt.Sprintf("Added #%d: %s", task.ShortID, task.Title)
	m.loadItems()
	m.clampCursor()
	return m, nil
}
