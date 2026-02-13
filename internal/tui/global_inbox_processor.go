package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// GlobalInboxProcessorModel handles sequential inbox task processing across all projects.
type GlobalInboxProcessorModel struct {
	config        *config.Config
	configManager *config.Manager

	// Task queue and navigation
	inboxTasks   []GlobalInboxTask // Filtered list of root Inbox tasks from all projects
	currentIndex int               // Current position in queue
	processedIDs map[string]bool   // Tasks marked as processed
	skippedIDs   map[string]bool   // Tasks skipped without processing

	// Project cache for efficient lookups
	projectCache map[string]*models.Project

	// Processing history for undo (session-based, not persisted)
	processingHistory []globalProcessingAction

	// UI state
	width         int
	height        int
	err           error
	statusMessage string // Temporary feedback message

	// Confirmation state
	confirm ConfirmDialog

	// Prefix key state (which-key style)
	pendingPrefix string
}

// globalProcessingAction records an action for undo support.
type globalProcessingAction struct {
	taskID       string
	projectID    string
	actionType   string // "section", "priority", "tags", "delete", etc.
	timestamp    time.Time
	prevSnapshot *inboxSnapshot
}

// NewGlobalInboxProcessorModel creates a new global inbox processor model.
func NewGlobalInboxProcessorModel(cfg *config.Config, manager *config.Manager) *GlobalInboxProcessorModel {
	m := &GlobalInboxProcessorModel{
		config:            cfg,
		configManager:     manager,
		currentIndex:      0,
		processedIDs:      make(map[string]bool),
		skippedIDs:        make(map[string]bool),
		projectCache:      make(map[string]*models.Project),
		processingHistory: []globalProcessingAction{},
	}
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

	// Sort by creation date (oldest first - FIFO processing)
	sort.Slice(m.inboxTasks, func(i, j int) bool {
		return m.inboxTasks[i].Task.CreatedAt.Before(m.inboxTasks[j].Task.CreatedAt)
	})
}

// Init initializes the model.
func (m *GlobalInboxProcessorModel) Init() tea.Cmd {
	return nil
}

// SetSize sets the view dimensions.
func (m *GlobalInboxProcessorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages.
func (m *GlobalInboxProcessorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case SectionPickerDoneMsg:
		return m.handleSectionPickerDone(msg)

	case TaskUpdatedMsg:
		return m.handleTaskUpdated(msg)

	case TaskPropertyChangedMsg:
		return m.handleTaskUpdated(TaskUpdatedMsg{Task: msg.Task})

	case DueDateChangedMsg:
		return m.handleDueDateChanged(msg)

	case DependencySelectorDoneMsg:
		return m.handleDependencySelectorDone(msg)

	case TagSelectorDoneMsg:
		return m.handleTagSelectorDone(msg)

	case TaskCreatedMsg:
		// Subtask created
		m.statusMessage = "Subtask created"
		return m, m.reloadCurrentProject()

	case GlobalInboxProjectSavedMsg:
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		// Rebuild inbox tasks to get fresh task references after reload
		m.loadAllInboxTasks()
		// Verify currentIndex is still valid
		if m.currentIndex >= len(m.inboxTasks) {
			m.currentIndex = len(m.inboxTasks) - 1
		}
		if m.currentIndex < 0 {
			m.currentIndex = 0
		}
		return m, nil

	case ErrorMsg:
		m.err = msg.Err
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *GlobalInboxProcessorModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle confirmation
	if confirmed, handled := m.confirm.HandleKey(key); handled {
		if confirmed {
			return m, m.executeDelete()
		}
		m.statusMessage = ""
		return m, nil
	}

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error
	m.err = nil

	// Activate prefix mode
	if key == "," || key == "g" {
		m.pendingPrefix = key
		return m, nil
	}

	switch key {
	// Navigation
	case "q":
		return m, func() tea.Msg {
			return CloseGlobalInboxProcessorMsg{
				ProcessedCount: len(m.processedIDs),
				SkippedCount:   len(m.skippedIDs),
			}
		}

	case "esc":
		return m, func() tea.Msg {
			return CloseGlobalInboxProcessorMsg{
				ProcessedCount: len(m.processedIDs),
				SkippedCount:   len(m.skippedIDs),
			}
		}

	case "n":
		// Skip current task and advance to next (workflow: "next")
		return m, m.skipCurrent()

	case "j", "down":
		// Move to next task in queue
		return m, m.advanceToNext(false)

	case "k", "up":
		// Move to previous task in queue
		m.goToPrevious()
		return m, nil

	case "G":
		// Go to last task (Vim-style)
		if len(m.inboxTasks) > 0 {
			m.currentIndex = len(m.inboxTasks) - 1
		}
		return m, nil

	// Operations
	case "s":
		// Assign section
		return m, m.openSectionPicker()

	case "1", "2", "3", "4", "5":
		return m, m.setPriorityQuick(key)

	case "t":
		return m, m.openTagSelector()

	case "e":
		return m, m.openTaskEditor()

	case "c":
		// Create subtask
		return m, m.openSubtaskCreator()

	case "D":
		// Set due date
		return m, m.openDueDateInput()

	case "b":
		return m, m.openDependencySelector()

	case "x":
		return m, m.deleteCurrentTask()

	case "w":
		// Send to Waiting section
		return m, m.sendToWaiting()

	case "u":
		// Undo last action
		return m, m.undoLastAction()

	case "enter":
		if git := m.getCurrentTask(); git != nil {
			task := git.Task
			project := git.Project
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return m, nil

	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewGlobalInboxProcessor,
				ViewName:   "Global Inbox Processor",
				Items:      m.GetHelpItems(),
			}
		}
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *GlobalInboxProcessorModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	switch prefix {
	case ",":
		if status, ok := StatusForPrefixKey(key); ok {
			return m, m.changeTaskStatus(status, StatusVerb(status))
		}
	case "g":
		// Navigation operations
		if key == "g" {
			// gg: Go to first task
			m.currentIndex = 0
			return m, nil
		}
	}

	// Invalid prefix sequence - silently ignore
	return m, nil
}

// View renders the global inbox processor.
func (m *GlobalInboxProcessorModel) View() string {
	if len(m.inboxTasks) == 0 {
		return m.renderEmptyInbox()
	}

	var b strings.Builder

	// Header with progress
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Current task display
	if git := m.getCurrentTask(); git != nil {
		b.WriteString(m.renderTaskDisplay(git))
	} else {
		b.WriteString(m.renderEmptyState())
	}

	b.WriteString("\n")

	// Operations panel
	b.WriteString(m.renderOperationsPanel())

	b.WriteString("\n\n")

	// Footer with controls
	b.WriteString(m.renderFooter())

	// Status message
	if m.statusMessage != "" {
		b.WriteString("\n")
		b.WriteString(SuccessStyle.Render(m.statusMessage))
	}

	// Error message
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	return b.String()
}

// getProgressStats returns processing statistics.
func (m *GlobalInboxProcessorModel) getProgressStats() (processed, skipped, remaining, total int) {
	total = len(m.inboxTasks)
	processed = len(m.processedIDs)
	skipped = len(m.skippedIDs)
	remaining = total - processed - skipped
	return
}

// getCurrentTask returns the current task being processed.
func (m *GlobalInboxProcessorModel) getCurrentTask() *GlobalInboxTask {
	if m.currentIndex >= 0 && m.currentIndex < len(m.inboxTasks) {
		return &m.inboxTasks[m.currentIndex]
	}
	return nil
}

// advanceToNext moves to the next unprocessed task.
func (m *GlobalInboxProcessorModel) advanceToNext(autoClose bool) tea.Cmd {
	// Clear undo history when changing tasks
	m.processingHistory = []globalProcessingAction{}

	// Find next unprocessed task
	for i := m.currentIndex + 1; i < len(m.inboxTasks); i++ {
		taskID := m.inboxTasks[i].Task.ID
		if !m.processedIDs[taskID] && !m.skippedIDs[taskID] {
			m.currentIndex = i
			m.statusMessage = ""
			return nil
		}
	}

	// No more unprocessed tasks
	if autoClose {
		if len(m.processedIDs) == 0 && len(m.skippedIDs) == 0 {
			m.statusMessage = "No tasks were processed"
		}

		return func() tea.Msg {
			return CloseGlobalInboxProcessorMsg{
				ProcessedCount: len(m.processedIDs),
				SkippedCount:   len(m.skippedIDs),
			}
		}
	}

	return nil
}

// goToPrevious moves to the previous task.
func (m *GlobalInboxProcessorModel) goToPrevious() {
	m.processingHistory = []globalProcessingAction{}
	if m.currentIndex > 0 {
		m.currentIndex--
		m.statusMessage = ""
	}
}

// skipCurrent skips current task and advances.
func (m *GlobalInboxProcessorModel) skipCurrent() tea.Cmd {
	if git := m.getCurrentTask(); git != nil {
		m.skippedIDs[git.Task.ID] = true
		delete(m.processedIDs, git.Task.ID)
		m.statusMessage = fmt.Sprintf("Skipped task #%d", git.Task.ShortID)
		return m.advanceToNext(true)
	}
	return nil
}

// captureSnapshot creates a snapshot of task state for undo.
func (m *GlobalInboxProcessorModel) captureSnapshot(task *models.Task) *inboxSnapshot {
	return captureInboxSnapshot(task)
}

// recordAction records an action in the undo history.
func (m *GlobalInboxProcessorModel) recordAction(actionType string, snapshot *inboxSnapshot) {
	git := m.getCurrentTask()
	if git == nil {
		return
	}

	m.processingHistory = append(m.processingHistory, globalProcessingAction{
		taskID:       git.Task.ID,
		projectID:    git.Project.ID,
		actionType:   actionType,
		timestamp:    time.Now(),
		prevSnapshot: snapshot,
	})
}

// undoLastAction undoes the last action on the current task.
func (m *GlobalInboxProcessorModel) undoLastAction() tea.Cmd {
	if len(m.processingHistory) == 0 {
		m.statusMessage = "Nothing to undo"
		return nil
	}

	// Pop last action
	action := m.processingHistory[len(m.processingHistory)-1]
	m.processingHistory = m.processingHistory[:len(m.processingHistory)-1]

	// Get current task and verify it matches
	git := m.getCurrentTask()
	if git == nil || git.Task.ID != action.taskID {
		m.err = fmt.Errorf("can only undo actions on current task")
		return nil
	}

	// Restore snapshot
	if action.prevSnapshot != nil {
		restoreInboxSnapshot(git.Task, action.prevSnapshot)
	}

	m.statusMessage = fmt.Sprintf("Undone: %s", action.actionType)
	return m.saveCurrentProject()
}

// setPriorityQuick sets priority using quick keys 1-5.
func (m *GlobalInboxProcessorModel) setPriorityQuick(key string) tea.Cmd {
	priority, _ := strconv.Atoi(key)

	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	snapshot := m.captureSnapshot(git.Task)
	git.Project.MutateTask(git.Task.ID, func(t *models.Task) {
		t.Priority = priority
	})

	m.recordAction(fmt.Sprintf("priority set to %d", priority), snapshot)
	m.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return m.saveCurrentProject()
}

// saveCurrentProject saves the project for the current task.
func (m *GlobalInboxProcessorModel) saveCurrentProject() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	project := git.Project
	return func() tea.Msg {
		if err := saveProjectSync(project); err != nil {
			return GlobalInboxProjectSavedMsg{Error: fmt.Errorf("save failed: %w", err)}
		}
		return GlobalInboxProjectSavedMsg{Error: nil}
	}
}

// reloadCurrentProject reloads the project from storage.
func (m *GlobalInboxProcessorModel) reloadCurrentProject() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	project := git.Project
	return func() tea.Msg {
		if err := storage.LoadProject(project); err != nil {
			return GlobalInboxProjectSavedMsg{Error: fmt.Errorf("reload failed: %w", err)}
		}
		return GlobalInboxProjectSavedMsg{Error: nil}
	}
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *GlobalInboxProcessorModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

// HasTextInput returns false for the global inbox processor.
func (m *GlobalInboxProcessorModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true when prefix mode is active.
func (m *GlobalInboxProcessorModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix()
}

// HasActiveSearch returns false (no search in this view).
func (m *GlobalInboxProcessorModel) HasActiveSearch() bool {
	return false
}

// GetHelpItems returns help items for the global inbox processor view.
func (m *GlobalInboxProcessorModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewGlobalInboxProcessor)
}

// willLeaveInbox returns true if the task will no longer appear in inbox after save.
func (m *GlobalInboxProcessorModel) willLeaveInbox(task *models.Task) bool {
	return willLeaveInbox(task)
}
