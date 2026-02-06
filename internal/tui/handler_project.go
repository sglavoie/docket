package tui

import (
	"fmt"
	"os"
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

	case ToggleInfoPanelMsg:
		newValue := !m.config.GetShowInfoPanel()
		m.config.SetShowInfoPanel(newValue)
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		if newValue {
			m.statusMessage = "Info panel shown"
		} else {
			m.statusMessage = "Info panel hidden"
		}
		return true, defaultStatusTimeout()

	case TutorDismissedMsg:
		m.config.SetTutorDismissed(true)
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		m.statusMessage = "Tutor prompt dismissed"
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

	case ShowProjectPickerMsg:
		projects := m.config.GetActiveProjects()
		picker := NewProjectPickerModel(projects)
		m.showView(ViewProjectPicker, picker)
		return true, picker.Init()

	case ProjectPickerDoneMsg:
		m.popView() // Close picker
		if !msg.Canceled && msg.Project != nil {
			// Save current project ID so we can restore it when navigating back
			if m.currentProject != nil {
				m.projectStack = append(m.projectStack, m.currentProject.ID)
			}
			m.switchToProject(msg.Project, true)
			m.pushView(ViewProjectTree)
		}
		return true, nil

	case SandboxResetRequestMsg:
		sandbox := m.config.GetSandboxProject()
		if sandbox == nil {
			m.statusMessage = "Sandbox project not found"
			return true, defaultStatusTimeout()
		}
		if err := storage.LoadProject(sandbox); err != nil {
			m.err = fmt.Errorf("failed to load sandbox: %w", err)
			return true, nil
		}
		taskCount := len(sandbox.Tasks)
		notes, _ := storage.ListNotes(sandbox)
		noteCount := len(notes)
		customSections := sandbox.GetCustomSections()
		sectionCount := len(customSections)

		if taskCount == 0 && noteCount == 0 && sectionCount == 0 {
			m.statusMessage = "Sandbox is already empty"
			return true, defaultStatusTimeout()
		}

		// Build item count for confirmation
		totalItems := taskCount + noteCount + sectionCount
		m.pendingSandboxReset = true
		m.pendingSandboxTaskCount = totalItems
		m.statusMessage = fmt.Sprintf("Reset sandbox? Delete %d item(s)? (y/n)", totalItems)
		return true, nil

	case SandboxResetDoneMsg:
		m.pendingSandboxReset = false
		m.pendingSandboxTaskCount = 0
		m.statusMessage = fmt.Sprintf("Cleared %d item(s) from sandbox", msg.TaskCount)
		// Refresh views if sandbox is currently displayed
		if m.currentProject != nil && m.currentProject.Sandbox {
			m.refreshProjectTree()
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
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

// executeSandboxReset performs the actual sandbox reset operation.
func (m *AppModel) executeSandboxReset() tea.Cmd {
	return func() tea.Msg {
		sandbox := m.config.GetSandboxProject()
		if sandbox == nil {
			return ErrorMsg{Err: fmt.Errorf("sandbox project not found")}
		}

		if err := storage.LoadProject(sandbox); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to load sandbox: %w", err)}
		}

		taskCount := len(sandbox.Tasks)
		notes, _ := storage.ListNotes(sandbox)
		noteCount := len(notes)
		customSections := sandbox.GetCustomSections()
		sectionCount := len(customSections)
		totalItems := taskCount + noteCount + sectionCount

		if totalItems == 0 {
			return SandboxResetDoneMsg{TaskCount: 0}
		}

		// Record undo snapshot before clearing (only captures tasks, not notes)
		undoDepth := 0
		if m.config.UndoHistoryDepth != nil {
			undoDepth = *m.config.UndoHistoryDepth
		}
		if err := storage.RecordUndoSnapshot(sandbox, "reset sandbox", "", undoDepth); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to record undo snapshot: %w", err)}
		}

		// Clear tasks and reset NextShortID
		sandbox.Tasks = nil
		sandbox.NextShortID = 1

		// Reset sections to default
		sandbox.Sections = []string{models.SectionNext}
		sandbox.SectionColors = make(map[string]string)

		if err := storage.SaveProject(sandbox); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save sandbox: %w", err)}
		}

		// Clear notes directory
		if noteCount > 0 {
			notesDir := storage.NotesDir(sandbox)
			if err := os.RemoveAll(notesDir); err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to clear notes: %w", err)}
			}
		}

		// Save config for NextShortID persistence
		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}

		return SandboxResetDoneMsg{TaskCount: totalItems}
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
