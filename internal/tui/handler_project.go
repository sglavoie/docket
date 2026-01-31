package tui

import (
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleProjectMessages handles project-related messages (create, edit, archive, delete, restore).
// Returns true if the message was handled.
func (m *AppModel) handleProjectMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case SelectProjectMsg:
		// Pop overlay views if that's where we came from
		if m.currentView() == ViewCommandPalette {
			m.popView()
		}
		if m.currentView() == ViewProjectCycleManager {
			m.popView()
			// Refresh dashboard in case pins were changed
			if m.dashboard != nil {
				m.dashboard.Refresh(m.config)
			}
		}
		m.switchToProject(msg.Project, true)
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil

	case CreateProjectMsg:
		m.showView(ViewProjectCreator, NewProjectCreatorModel(m.config, m.configManager))
		return true, nil

	case ProjectCreatedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ProjectArchivedMsg:
		m.popView()
		// Clear current project if archived
		if msg.Project != nil && m.config != nil && msg.Project.Archived && m.config.CurrentProject == msg.Project.ID {
			m.config.CurrentProject = ""
		}
		if m.configManager != nil {
			if err := m.configManager.Save(m.config); err != nil {
				m.err = err
				return true, nil
			}
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ProjectPinnedToggledMsg:
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		statusMsg := "Pinned"
		if !msg.Project.Pinned {
			statusMsg = "Unpinned"
		}
		m.statusMessage = statusMsg
		return true, defaultStatusTimeout()

	case ProjectDeletedMsg:
		m.config.RemoveProject(msg.Project.ID)
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
			m.dashboard.ResetCursor()
		}
		return true, nil

	case ProjectRestoredMsg:
		if m.configManager != nil {
			if err := m.configManager.Save(m.config); err != nil {
				m.err = err
				return true, nil
			}
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case EditProjectMsg:
		if msg.Project != nil {
			m.showView(ViewProjectEditor, NewProjectEditorModel(msg.Project, m.config, m.configManager))
		}
		return true, nil

	case ProjectUpdatedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		m.refreshProjectTree()
		return true, nil

	case SetCurrentProjectMsg:
		m.config.CurrentProject = msg.Project.ID
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		m.statusMessage = fmt.Sprintf("Set current: %s", msg.Project.Name)
		return true, defaultStatusTimeout()

	case ToggleRecentTasksMsg:
		newValue := !m.config.GetShowRecentTasks()
		m.config.SetShowRecentTasks(newValue)
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		if newValue {
			m.statusMessage = "Recent Tasks shown"
		} else {
			m.statusMessage = "Recent Tasks hidden"
		}
		return true, defaultStatusTimeout()

	case ShowTrashedProjectsMsg:
		m.showView(ViewTrashedProjects, NewTrashedProjectsModel(m.config.DataDir))
		return true, nil

	case CloseTrashedProjectsMsg:
		m.popView()
		return true, nil

	case RestoreFromTrashMsg:
		m.isLoading = true
		m.loadingMessage = "Restoring..."
		return true, tea.Batch(m.spinner.Tick(), m.restoreProjectFromTrash(msg.Slug))

	case projectRestoredFromTrashMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.popView() // Close trashed projects picker
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		m.statusMessage = fmt.Sprintf("Restored: %s", msg.Project.Name)
		return true, defaultStatusTimeout()

	case PurgeFromTrashMsg:
		m.isLoading = true
		m.loadingMessage = "Deleting..."
		return true, tea.Batch(m.spinner.Tick(), m.purgeProjectFromTrash(msg.Slug))

	case projectPurgedFromTrashMsg:
		m.isLoading = false
		m.loadingMessage = ""
		// Refresh the trashed projects list by re-registering the view
		newModel := NewTrashedProjectsModel(m.config.DataDir)
		newModel.SetSize(m.width, m.height-2)
		m.views.Register(ViewTrashedProjects, newModel)
		m.statusMessage = fmt.Sprintf("Permanently deleted: %s", msg.Slug)
		return true, defaultStatusTimeout()
	}
	return false, nil
}

// restoreProjectFromTrash restores a project from the trash directory.
func (m *AppModel) restoreProjectFromTrash(slug string) tea.Cmd {
	return func() tea.Msg {
		// Check if project exists in trash
		if !storage.TrashEntryExists(slug, m.config.DataDir) {
			return ErrorMsg{Err: fmt.Errorf("project '%s' not found in trash", slug)}
		}

		// Check if active project with same slug exists
		if m.config.GetProject(slug) != nil {
			return ErrorMsg{Err: fmt.Errorf("active project '%s' already exists; rename or delete it first", slug)}
		}

		// Backup config before making changes
		if err := m.configManager.BackupConfig(); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create config backup: %w", err)}
		}

		// Restore project directory from trash
		if err := storage.RestoreProjectFromTrash(slug, m.config.DataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to restore project from trash: %w", err)}
		}

		// Reconstruct project metadata
		projectPath := m.config.GetProjectPath(slug)
		project, err := reconstructProjectFromDir(slug, projectPath)
		if err != nil {
			// Move back to trash on failure (best effort)
			_ = storage.MoveProjectToTrash(&models.Project{Slug: slug, Path: projectPath}, m.config.DataDir)
			return ErrorMsg{Err: fmt.Errorf("failed to reconstruct project metadata: %w", err)}
		}

		// Add project to config
		m.config.AddProject(project)

		// Save config
		if err := m.configManager.Save(m.config); err != nil {
			// Move back to trash on failure (best effort)
			_ = storage.MoveProjectToTrash(project, m.config.DataDir)
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}

		return projectRestoredFromTrashMsg{Project: project}
	}
}

// projectRestoredFromTrashMsg is an internal message for trash restoration completion.
type projectRestoredFromTrashMsg struct {
	Project *models.Project
}

// projectPurgedFromTrashMsg is an internal message for trash purge completion.
type projectPurgedFromTrashMsg struct {
	Slug string
}

// purgeProjectFromTrash permanently deletes a project from the trash directory.
func (m *AppModel) purgeProjectFromTrash(slug string) tea.Cmd {
	return func() tea.Msg {
		if err := storage.PurgeFromTrash(slug, m.config.DataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to delete project: %w", err)}
		}
		return projectPurgedFromTrashMsg{Slug: slug}
	}
}

// reconstructProjectFromDir rebuilds project metadata from a restored directory.
func reconstructProjectFromDir(slug, projectPath string) (*models.Project, error) {
	// Create a temporary project to load tasks
	tempProject := &models.Project{
		Path: projectPath,
	}

	// Try to load and parse tasks.md to get project name
	if err := storage.LoadProject(tempProject); err != nil {
		return nil, fmt.Errorf("failed to load project tasks: %w", err)
	}

	// Read the raw file to get the project name from the header
	projectName := slug // Default to slug if we can't find the name
	tasksPath := filepath.Join(projectPath, "tasks.md")
	content, err := storage.ParseProjectNameFromFile(tasksPath)
	if err == nil && content != "" {
		projectName = content
	}

	// Create the project with a new ID (since we don't store ID in tasks.md)
	project := &models.Project{
		ID:          uuid.New().String(),
		Slug:        slug,
		Name:        projectName,
		Path:        projectPath,
		Context:     "personal", // Default context
		CreatedAt:   time.Now(), // We don't preserve original creation time
		UpdatedAt:   time.Now(),
		Tasks:       tempProject.Tasks,
		NextShortID: tempProject.NextShortID,
	}

	return project, nil
}
