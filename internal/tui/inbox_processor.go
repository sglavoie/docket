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

// InboxProcessorModel handles sequential inbox task processing for a single project.
// It embeds inboxCore for all shared logic and implements inboxBackend for
// project-specific behavior.
type InboxProcessorModel struct {
	inboxCore
	project    *models.Project
	inboxTasks []*models.Task
}

// NewInboxProcessorModel creates a new inbox processor model.
func NewInboxProcessorModel(project *models.Project, manager *config.Manager, runtime *config.RuntimeState) *InboxProcessorModel {
	m := &InboxProcessorModel{
		project:    project,
		inboxTasks: getInboxTasksForModel(project),
	}
	m.initCore(m, m, manager, runtime)
	return m
}

// getInboxTasksForModel filters and sorts inbox tasks for processing.
func getInboxTasksForModel(project *models.Project) []*models.Task {
	var tasks []*models.Task
	for _, task := range project.Tasks {
		if isProcessableInboxTask(task) {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		if !tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
		}
		return tasks[i].ID < tasks[j].ID
	})
	return tasks
}

// --- inboxBackend implementation ---

// getTaskRef returns the currently focused inbox task and its project.
func (m *InboxProcessorModel) getTaskRef() (*models.Task, *models.Project) {
	return m.currentTaskRef()
}

func (m *InboxProcessorModel) taskCount() int { return len(m.inboxTasks) }

func (m *InboxProcessorModel) taskRef(index int) (*models.Task, *models.Project) {
	return m.inboxTasks[index], m.project
}

func (m *InboxProcessorModel) closeMsg(processed, skipped int) tea.Msg {
	return CloseInboxProcessorMsg{ProcessedCount: processed, SkippedCount: skipped}
}

func (m *InboxProcessorModel) handleSavedMsg(msg tea.Msg) (bool, error) {
	if saved, ok := msg.(InboxProjectSavedMsg); ok {
		return true, saved.Error
	}
	return false, nil
}

func (m *InboxProcessorModel) saveCmd() tea.Cmd {
	return m.saveProjectCmd(m.project)
}

func (m *InboxProcessorModel) saveProjectCmd(project *models.Project) tea.Cmd {
	if project == nil {
		project = m.project
	}
	dataDir := m.runtime.DataDir()
	deps := m.configManager.RuntimeDependencies()
	ledger := workspaceLedgerFromConfig(m.runtime)
	return func() tea.Msg {
		if err := commitInboxProject(dataDir, deps, ledger, m.configManager, m.runtime, project); err != nil {
			return InboxProjectSavedMsg{Error: fmt.Errorf("save failed: %w", err)}
		}
		return InboxProjectSavedMsg{Error: nil}
	}
}

func (m *InboxProcessorModel) reloadCmd() tea.Cmd {
	project := m.project
	return func() tea.Msg {
		if err := storage.LoadProject(project); err != nil {
			return InboxProjectSavedMsg{Error: fmt.Errorf("reload failed: %w", err)}
		}
		return InboxProjectSavedMsg{Error: nil}
	}
}

func (m *InboxProcessorModel) rebuildTasks() {
	m.inboxTasks = getInboxTasksForModel(m.project)
}

func (m *InboxProcessorModel) markDeletedProcessed() bool { return true }

func (m *InboxProcessorModel) shouldProcessOnSectionChange(task *models.Task, project *models.Project) bool {
	current := project.GetTaskByID(task.ID)
	return current != nil && willLeaveInbox(current)
}

func (m *InboxProcessorModel) shouldProcessOnStateChange(newState models.WorkflowState) bool {
	return newState.IsTerminal()
}

func (m *InboxProcessorModel) findTaskByID(taskID, _ string) (*models.Task, *models.Project) {
	if t := m.project.GetTaskByID(taskID); t != nil {
		return t, m.project
	}
	return nil, nil
}

func (m *InboxProcessorModel) projectByID(projectID string) *models.Project {
	if m.project != nil && m.project.ID == projectID {
		return m.project
	}
	return nil
}

func (m *InboxProcessorModel) showProjectInHeader() bool   { return false }
func (m *InboxProcessorModel) headerTitle() string         { return "Inbox" }
func (m *InboxProcessorModel) headerSubtitleExtra() string { return "" }
func (m *InboxProcessorModel) emptyStateText() string {
	return icons.Get("success") + " Inbox is empty! All tasks have been triaged."
}
func (m *InboxProcessorModel) emptyInboxText() string {
	return icons.Get("success") + " Inbox is empty! All tasks have been triaged."
}
func (m *InboxProcessorModel) returnHint() string       { return "return to outliner" }
func (m *InboxProcessorModel) viewType() PresentationID { return ViewInboxProcessor }
func (m *InboxProcessorModel) helpViewName() string     { return "Inbox Processor" }

func (m *InboxProcessorModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *InboxProcessorModel) StatusBarLabel() string { return "Inbox" }
