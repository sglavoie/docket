package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleProjectMessages handles project-related messages (create, edit, archive, delete, restore).
// Returns true if the message was handled.
func (m *AppModel) handleProjectMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case SelectProjectMsg:
		// Pop overlay views if that's where we came from
		if m.currentView() == ViewCommandPalette {
			m.popView()
		}
		m.saveCurrentProjectView()
		m.switchToProject(msg.Project, true)
		targetView := m.getProjectLastView(msg.Project.ID)
		switch targetView {
		case ViewProjectTree:
			m.showView(ViewProjectTree, m.projectTree)
		default:
			m.saveOutlinerToggleState()
			m.saveOutlinerFilterState()
			m.outliner = NewOutlinerModel(msg.Project, m.config, m.configManager)
			m.outliner.queueIDs = m.focusQueueIDs
			m.outliner.dailyPlanIDs = m.dailyPlanIDs
			m.restoreOutlinerToggleState()
			m.restoreOutlinerFilterState()
			m.showView(ViewOutliner, m.outliner)
		}
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

	case ProjectArchiveRequestMsg:
		return true, m.showProjectArchiveConfirm(msg.Project, msg.Archive)

	case projectArchiveConfirmedMsg:
		return true, m.confirmProjectArchive()

	case ProjectArchivedMsg:
		popCmd := m.popView()
		// Clear current project if archived
		if msg.Project != nil && m.config != nil && msg.Project.Archived && m.config.CurrentProject == msg.Project.ID {
			m.config.CurrentProject = ""
		}
		if msg.Project != nil && msg.Project.Archived && m.currentProject != nil && m.currentProject.ID == msg.Project.ID {
			m.currentProject = nil
		}
		// Clean up config references for tasks in the archived project
		if msg.Project != nil {
			taskIDs := make(map[string]bool, len(msg.Project.Tasks))
			for _, t := range msg.Project.Tasks {
				taskIDs[t.ID] = true
			}
			m.config.RemoveTaskRefs(taskIDs)
		}
		m.pruneDeletedTaskRefs()
		if m.configManager != nil {
			if err := m.configManager.Save(m.config); err != nil {
				m.err = err
				return true, nil
			}
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		if msg.Project != nil {
			m.setSuccessStatus(fmt.Sprintf("Archived %s", msg.Project.Name))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		}
		return true, popCmd

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
		m.setSuccessStatus(statusMsg)
		return true, defaultStatusTimeout()

	case ProjectDeletedMsg:
		m.config.RemoveProject(msg.Project.ID)
		if m.currentProject != nil && m.currentProject.ID == msg.Project.ID {
			m.currentProject = nil
		}
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
		}
		m.pruneDeletedTaskRefs()
		// Pop back to dashboard since the project no longer exists
		m.viewStack = m.viewStack[:1]
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
		if msg.Project != nil {
			m.setSuccessStatus(fmt.Sprintf("Unarchived %s", msg.Project.Name))
			return true, defaultStatusTimeout()
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

	case ToggleInfoPanelMsg:
		mode := m.config.CycleInfoPanelMode()
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		switch mode {
		case config.InfoPanelOff:
			m.setInfoStatus("Info panel hidden")
		case config.InfoPanelInfoCards:
			m.setInfoStatus("Info panel: info cards")
		case config.InfoPanelProjectSummary:
			m.setInfoStatus("Info panel: project summary")
		}
		return true, defaultStatusTimeout()

	case TutorDismissedMsg:
		m.config.SetTutorDismissed(true)
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		m.setSuccessStatus("Tutor prompt dismissed")
		return true, defaultStatusTimeout()

	case ShowTrashedProjectsMsg:
		m.showView(ViewTrashedProjects, NewTrashedProjectsModel(m.config.DataDir))
		return true, nil

	case RestoreFromTrashMsg:
		m.isLoading = true
		m.loadingMessage = "Restoring..."
		return true, tea.Batch(
			m.spinner.Tick(),
			m.restoreProjectFromTrash(msg.Slug),
			tea.Tick(10*time.Second, func(time.Time) tea.Msg { return SaveTimeoutMsg{} }),
		)

	case projectRestoredFromTrashMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.popView() // Close trashed projects picker
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		m.setSuccessStatus(fmt.Sprintf("Restored: %s", msg.Project.Name))
		return true, defaultStatusTimeout()

	case PurgeFromTrashMsg:
		m.isLoading = true
		m.loadingMessage = "Deleting..."
		return true, tea.Batch(
			m.spinner.Tick(),
			m.purgeProjectFromTrash(msg.Slug),
			tea.Tick(10*time.Second, func(time.Time) tea.Msg { return SaveTimeoutMsg{} }),
		)

	case projectPurgedFromTrashMsg:
		m.isLoading = false
		m.loadingMessage = ""
		// Refresh the trashed projects list by re-registering the view
		newModel := NewTrashedProjectsModel(m.config.DataDir)
		newModel.SetSize(m.width, m.height-2)
		m.views.Register(ViewTrashedProjects, newModel)
		m.setSuccessStatus(fmt.Sprintf("Permanently deleted: %s", msg.Slug))
		return true, defaultStatusTimeout()

	case ShowProjectPickerMsg:
		projects := projectDiscoveryProjects(m.config)
		for _, p := range projects {
			if err := storage.LoadProject(p); err != nil {
				logging.Warn("failed to load project %s: %v", p.Name, err)
			}
		}
		return m.showViewInit(ViewProjectPicker, NewProjectPickerModel(projects))

	case ProjectPickerDoneMsg:
		m.popView() // Close picker
		if !msg.Canceled && msg.Project != nil {
			// Save current project's active view before switching away
			m.saveCurrentProjectView()
			// Save current project ID so we can restore it when navigating back
			if m.currentProject != nil {
				m.projectStack = append(m.projectStack, m.currentProject.ID)
			}
			m.switchToProject(msg.Project, true)
			targetView := m.getProjectLastView(msg.Project.ID)
			switch targetView {
			case ViewOutliner:
				m.saveOutlinerToggleState()
				m.saveOutlinerFilterState()
				m.outliner = NewOutlinerModel(msg.Project, m.config, m.configManager)
				m.outliner.queueIDs = m.focusQueueIDs
				m.outliner.dailyPlanIDs = m.dailyPlanIDs
				m.restoreOutlinerToggleState()
				m.restoreOutlinerFilterState()
				m.showView(ViewOutliner, m.outliner)
			default:
				m.pushView(ViewProjectTree)
			}
		}
		return true, nil

	case SandboxResetRequestMsg:
		sandbox := m.config.GetSandboxProject()
		if sandbox == nil {
			m.setInfoStatus("Sandbox project not found")
			return true, defaultStatusTimeout()
		}
		if err := storage.LoadProject(sandbox); err != nil {
			m.err = fmt.Errorf("failed to load sandbox: %w", err)
			return true, nil
		}
		taskCount := len(sandbox.Tasks)
		notes, _ := storage.ListNotes(sandbox)
		noteCount := len(notes)
		tagCount := len(sandbox.AllTags())
		pinnedTagCount := len(sandbox.PinnedTagList())

		if taskCount == 0 && noteCount == 0 && tagCount == 0 && pinnedTagCount == 0 {
			m.setInfoStatus("Sandbox is already empty")
			return true, defaultStatusTimeout()
		}

		// Build item count for confirmation
		totalItems := taskCount + noteCount + tagCount + pinnedTagCount
		m.pendingSandboxReset = true
		m.pendingSandboxTaskCount = totalItems
		m.setInfoStatus(fmt.Sprintf("Reset sandbox? Delete %d item(s)?", totalItems))
		return true, nil

	case SandboxResetDoneMsg:
		m.pendingSandboxReset = false
		m.pendingSandboxTaskCount = 0
		m.setSuccessStatus(fmt.Sprintf("Cleared %d item(s) from sandbox", msg.TaskCount))
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

func (m *AppModel) showProjectArchiveConfirm(project *models.Project, archive bool) tea.Cmd {
	if project == nil {
		m.clearPendingProjectArchive()
		return nil
	}
	if project.Sandbox {
		m.clearPendingProjectArchive()
		return errorCmd("cannot archive the sandbox project")
	}

	m.pendingProjectArchive = project
	m.pendingProjectArchiveArchive = archive

	verb := "archive"
	if !archive {
		verb = "unarchive"
	}
	m.projectArchiveConfirm.Show(projectArchiveConfirmAction, fmt.Sprintf("Are you sure you want to %s this project?", verb))
	return nil
}

func (m *AppModel) clearPendingProjectArchive() {
	m.pendingProjectArchive = nil
	m.pendingProjectArchiveArchive = false
	m.projectArchiveConfirm.Cancel()
}

func (m *AppModel) confirmProjectArchive() tea.Cmd {
	project := m.pendingProjectArchive
	archive := m.pendingProjectArchiveArchive
	m.clearPendingProjectArchive()

	if project == nil {
		return nil
	}
	if project.Sandbox {
		return errorCmd("cannot archive the sandbox project")
	}

	if archive {
		project.Archive()
		return func() tea.Msg {
			return ProjectArchivedMsg{Project: project}
		}
	}

	project.Unarchive()
	return func() tea.Msg {
		return ProjectRestoredMsg{Project: project}
	}
}

// restoreProjectFromTrash restores a project from the trash directory.
func (m *AppModel) restoreProjectFromTrash(slug string) tea.Cmd {
	cfg := m.config
	mgr := m.configManager
	dataDir := cfg.DataDir
	return func() tea.Msg {
		// Check if project exists in trash
		if !storage.TrashEntryExists(slug, dataDir) {
			return ErrorMsg{Err: fmt.Errorf("project '%s' not found in trash", slug)}
		}

		// Check if active project with same slug exists
		if cfg.GetProject(slug) != nil {
			return ErrorMsg{Err: fmt.Errorf("active project '%s' already exists; rename or delete it first", slug)}
		}

		// Backup config before making changes
		if err := mgr.BackupConfig(); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create config backup: %w", err)}
		}

		// Restore project directory from trash
		if err := storage.RestoreProjectFromTrash(slug, dataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to restore project from trash: %w", err)}
		}

		// Reconstruct project metadata
		projectPath := cfg.GetProjectPath(slug)
		project, err := reconstructProjectFromDir(slug, projectPath)
		if err != nil {
			// Move back to trash on failure (best effort)
			_ = storage.MoveProjectToTrash(&models.Project{Slug: slug, Path: projectPath}, dataDir)
			return ErrorMsg{Err: fmt.Errorf("failed to reconstruct project metadata: %w", err)}
		}

		// Add project to config
		cfg.AddProject(project)

		// Save config
		if err := mgr.Save(cfg); err != nil {
			// Move back to trash on failure (best effort)
			_ = storage.MoveProjectToTrash(project, dataDir)
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
	dataDir := m.config.DataDir
	return func() tea.Msg {
		if err := storage.PurgeFromTrash(slug, dataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to delete project: %w", err)}
		}
		return projectPurgedFromTrashMsg{Slug: slug}
	}
}

// executeSandboxReset performs the actual sandbox reset operation.
func (m *AppModel) executeSandboxReset() tea.Cmd {
	cfg := m.config
	mgr := m.configManager
	sandbox := cfg.GetSandboxProject()
	undoDepth := 0
	if cfg.UndoHistoryDepth != nil {
		undoDepth = *cfg.UndoHistoryDepth
	}
	return func() tea.Msg {
		if sandbox == nil {
			return ErrorMsg{Err: fmt.Errorf("sandbox project not found")}
		}

		if err := storage.LoadProject(sandbox); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to load sandbox: %w", err)}
		}

		taskCount := len(sandbox.Tasks)
		notes, _ := storage.ListNotes(sandbox)
		noteCount := len(notes)
		tagCount := len(sandbox.AllTags())
		pinnedTagCount := len(sandbox.PinnedTagList())
		totalItems := taskCount + noteCount + tagCount + pinnedTagCount

		if totalItems == 0 {
			return SandboxResetDoneMsg{TaskCount: 0}
		}

		// Record undo snapshot before clearing (only captures tasks, not notes)
		if err := storage.RecordUndoSnapshot(sandbox, "reset sandbox", "", undoDepth, nil); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to record undo snapshot: %w", err)}
		}

		// Clear tasks and reset NextShortID
		sandbox.Tasks = nil
		sandbox.NextShortID = 1

		// Reset sections to default
		sandbox.Sections = []string{models.SectionNext}
		sandbox.SectionColors = make(map[string]string)
		sandbox.Tags = []string{}
		sandbox.PinnedTags = []string{}

		if err := storage.SaveProjectLocked(sandbox); err != nil {
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
		if err := mgr.Save(cfg); err != nil {
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
		ID:            uuid.New().String(),
		Slug:          slug,
		Name:          projectName,
		Path:          projectPath,
		Context:       "personal", // Default context
		CreatedAt:     time.Now(), // We don't preserve original creation time
		UpdatedAt:     time.Now(),
		Tasks:         tempProject.Tasks,
		NextShortID:   tempProject.NextShortID,
		Sections:      tempProject.Sections,
		SectionColors: tempProject.SectionColors,
	}

	return project, nil
}
