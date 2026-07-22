package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// GlobalInboxProcessorModel handles sequential inbox task processing across all projects.
// It embeds inboxCore for all shared logic and implements inboxBackend for
// cross-project behavior.
type GlobalInboxProcessorModel struct {
	inboxCore
	inboxTasks   []GlobalInboxTask
	projectCache map[string]*models.Project
}

// NewGlobalInboxProcessorModel creates a new global inbox processor model.
func NewGlobalInboxProcessorModel(manager *config.Manager, runtime *config.RuntimeState) *GlobalInboxProcessorModel {
	m := &GlobalInboxProcessorModel{
		projectCache: make(map[string]*models.Project),
	}
	m.initCore(m, m, manager, runtime)
	m.loadAllInboxTasks()
	return m
}

// loadAllInboxTasks aggregates inbox tasks from all active projects.
func (m *GlobalInboxProcessorModel) loadAllInboxTasks() {
	m.inboxTasks = nil
	m.projectCache = make(map[string]*models.Project)

	projects := m.runtime.ActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}
		m.projectCache[project.ID] = project
		for _, task := range project.Tasks {
			if isProcessableInboxTask(task) {
				m.inboxTasks = append(m.inboxTasks, GlobalInboxTask{
					Task:    task,
					Project: project,
				})
			}
		}
	}

	sort.Slice(m.inboxTasks, func(i, j int) bool {
		ci, cj := m.inboxTasks[i].Task.CreatedAt, m.inboxTasks[j].Task.CreatedAt
		if !ci.Equal(cj) {
			return ci.Before(cj)
		}
		return m.inboxTasks[i].Task.ID < m.inboxTasks[j].Task.ID
	})
}

// HasActiveSearch returns false (no search in this view).
func (m *GlobalInboxProcessorModel) HasActiveSearch() bool {
	return false
}

// --- inboxBackend implementation ---

// getTaskRef returns the currently focused inbox task and its project.
func (m *GlobalInboxProcessorModel) getTaskRef() (*models.Task, *models.Project) {
	return m.currentTaskRef()
}

func (m *GlobalInboxProcessorModel) taskCount() int { return len(m.inboxTasks) }

func (m *GlobalInboxProcessorModel) taskRef(index int) (*models.Task, *models.Project) {
	return m.inboxTasks[index].Task, m.inboxTasks[index].Project
}

func (m *GlobalInboxProcessorModel) closeMsg(processed, skipped int) tea.Msg {
	return CloseGlobalInboxProcessorMsg{ProcessedCount: processed, SkippedCount: skipped}
}

func (m *GlobalInboxProcessorModel) handleSavedMsg(msg tea.Msg) (bool, error) {
	if saved, ok := msg.(GlobalInboxProjectSavedMsg); ok {
		return true, saved.Error
	}
	return false, nil
}

func (m *GlobalInboxProcessorModel) saveCmd() tea.Cmd {
	_, project := m.currentTaskRef()
	if project == nil {
		return nil
	}
	return m.saveProjectCmd(project)
}

func (m *GlobalInboxProcessorModel) saveProjectCmd(project *models.Project) tea.Cmd {
	if project == nil {
		return nil
	}
	dataDir := m.runtime.DataDir()
	deps := m.configManager.RuntimeDependencies()
	ledger := workspaceLedgerFromConfig(m.runtime)
	return func() tea.Msg {
		if err := commitInboxProject(dataDir, deps, ledger, m.configManager, m.runtime, project); err != nil {
			return GlobalInboxProjectSavedMsg{Error: fmt.Errorf("save failed: %w", err)}
		}
		return GlobalInboxProjectSavedMsg{Error: nil}
	}
}

func (m *GlobalInboxProcessorModel) reloadCmd() tea.Cmd {
	task, project := m.currentTaskRef()
	if task == nil {
		return nil
	}
	p := project
	return func() tea.Msg {
		if err := storage.LoadProject(p); err != nil {
			return GlobalInboxProjectSavedMsg{Error: fmt.Errorf("reload failed: %w", err)}
		}
		return GlobalInboxProjectSavedMsg{Error: nil}
	}
}

func (m *GlobalInboxProcessorModel) rebuildTasks() {
	m.loadAllInboxTasks()
}

func (m *GlobalInboxProcessorModel) markDeletedProcessed() bool { return true }

func (m *GlobalInboxProcessorModel) shouldProcessOnSectionChange(task *models.Task, project *models.Project) bool {
	current := project.GetTaskByID(task.ID)
	return current != nil && willLeaveInbox(current)
}

func (m *GlobalInboxProcessorModel) shouldProcessOnStateChange(newState models.WorkflowState) bool {
	return newState.IsTerminal()
}

func (m *GlobalInboxProcessorModel) findTaskByID(taskID, projectID string) (*models.Task, *models.Project) {
	if project, ok := m.projectCache[projectID]; ok && project != nil {
		if t := project.GetTaskByID(taskID); t != nil {
			return t, project
		}
	}
	for _, project := range m.projectCache {
		if project == nil {
			continue
		}
		if t := project.GetTaskByID(taskID); t != nil {
			return t, project
		}
	}
	return nil, nil
}

func (m *GlobalInboxProcessorModel) projectByID(projectID string) *models.Project {
	return m.projectCache[projectID]
}

func (m *GlobalInboxProcessorModel) showProjectInHeader() bool { return true }
func (m *GlobalInboxProcessorModel) headerTitle() string       { return "Global Inbox" }
func (m *GlobalInboxProcessorModel) headerSubtitleExtra() string {
	return fmt.Sprintf(" across %d", len(m.projectCache))
}
func (m *GlobalInboxProcessorModel) emptyStateText() string {
	return icons.Get("success") + " All inboxes are empty! All tasks have been triaged."
}
func (m *GlobalInboxProcessorModel) emptyInboxText() string {
	return icons.Get("success") + " All inboxes are empty! No tasks to process across any project."
}
func (m *GlobalInboxProcessorModel) returnHint() string       { return "return to dashboard" }
func (m *GlobalInboxProcessorModel) viewType() PresentationID { return ViewGlobalInboxProcessor }
func (m *GlobalInboxProcessorModel) helpViewName() string     { return "Global Inbox Processor" }

func (m *GlobalInboxProcessorModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *GlobalInboxProcessorModel) StatusBarLabel() string { return "Global Inbox" }
