package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
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
func NewGlobalInboxProcessorModel(cfg *config.Config, manager *config.Manager) *GlobalInboxProcessorModel {
	m := &GlobalInboxProcessorModel{
		projectCache: make(map[string]*models.Project),
	}
	m.initCore(m, m, cfg, manager)
	m.loadAllInboxTasks()
	return m
}

// loadAllInboxTasks aggregates inbox tasks from all active projects.
func (m *GlobalInboxProcessorModel) loadAllInboxTasks() {
	m.inboxTasks = nil
	m.projectCache = make(map[string]*models.Project)

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}
		m.projectCache[project.ID] = project
		for _, task := range project.Tasks {
			if task.Section == models.SectionInbox && task.IsRoot() && !task.IsComplete() && !task.IsSnoozed() {
				m.inboxTasks = append(m.inboxTasks, GlobalInboxTask{
					Task:    task,
					Project: project,
				})
			}
		}
	}

	sort.Slice(m.inboxTasks, func(i, j int) bool {
		return m.inboxTasks[i].Task.CreatedAt.Before(m.inboxTasks[j].Task.CreatedAt)
	})
}

// HasActiveSearch returns false (no search in this view).
func (m *GlobalInboxProcessorModel) HasActiveSearch() bool {
	return false
}

// --- inboxBackend implementation ---

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
	task, project := m.currentTaskRef()
	if task == nil {
		return nil
	}
	p := project
	return func() tea.Msg {
		if err := saveProjectSync(p); err != nil {
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

func (m *GlobalInboxProcessorModel) deleteFromData(task *models.Task, project *models.Project) {
	for i, t := range project.Tasks {
		if t.ID == task.ID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			return
		}
	}
}

func (m *GlobalInboxProcessorModel) markDeletedProcessed() bool { return true }

func (m *GlobalInboxProcessorModel) shouldProcessOnSectionChange(task *models.Task, project *models.Project) bool {
	current := project.GetTaskByID(task.ID)
	return current != nil && willLeaveInbox(current)
}

func (m *GlobalInboxProcessorModel) shouldProcessOnStatusChange(newStatus models.TaskStatus) bool {
	return newStatus.IsComplete()
}

func (m *GlobalInboxProcessorModel) showProjectInHeader() bool { return true }
func (m *GlobalInboxProcessorModel) headerTitle() string       { return "Global Inbox Processing" }
func (m *GlobalInboxProcessorModel) headerSubtitleExtra() string {
	return fmt.Sprintf(" from %d projects", len(m.projectCache))
}
func (m *GlobalInboxProcessorModel) emptyStateText() string {
	return "✓ All inboxes are empty! All tasks have been triaged."
}
func (m *GlobalInboxProcessorModel) emptyInboxText() string {
	return "✓ All inboxes are empty! No tasks to process across any project."
}
func (m *GlobalInboxProcessorModel) returnHint() string   { return "return to dashboard" }
func (m *GlobalInboxProcessorModel) viewType() ViewType   { return ViewGlobalInboxProcessor }
func (m *GlobalInboxProcessorModel) helpViewName() string { return "Global Inbox Processor" }

func (m *GlobalInboxProcessorModel) HandledKeys() []string {
	return inboxProcessorKeys()
}

func (m *GlobalInboxProcessorModel) StatusBarLabel() string { return "Global Inbox" }
