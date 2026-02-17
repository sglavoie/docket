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

// InboxProcessorModel handles sequential inbox task processing.
type InboxProcessorModel struct {
	project       *models.Project
	config        *config.Config
	configManager *config.Manager

	// Task queue and navigation
	inboxTasks   []*models.Task  // Filtered list of root Inbox tasks
	currentIndex int             // Current position in queue
	processedIDs map[string]bool // Tasks marked as processed
	skippedIDs   map[string]bool // Tasks skipped without processing

	// Processing history for undo (session-based, not persisted)
	processingHistory []processingAction

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

// processingAction records an action for undo support.
type processingAction struct {
	taskID       string
	actionType   string // "section", "priority", "tags", "delete", etc.
	timestamp    time.Time
	prevSnapshot *inboxSnapshot
}

// NewInboxProcessorModel creates a new inbox processor model.
func NewInboxProcessorModel(project *models.Project, cfg *config.Config, manager *config.Manager) *InboxProcessorModel {
	return &InboxProcessorModel{
		project:           project,
		config:            cfg,
		configManager:     manager,
		inboxTasks:        getInboxTasksForModel(project),
		currentIndex:      0,
		processedIDs:      make(map[string]bool),
		skippedIDs:        make(map[string]bool),
		processingHistory: []processingAction{},
	}
}

// getInboxTasksForModel filters and sorts inbox tasks for processing.
func getInboxTasksForModel(project *models.Project) []*models.Task {
	var tasks []*models.Task
	for _, task := range project.Tasks {
		if task.Section == models.SectionInbox && task.IsRoot() && !task.IsComplete() && !task.IsSnoozed() {
			tasks = append(tasks, task)
		}
	}
	// Sort by creation date (oldest first - FIFO processing)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})
	return tasks
}

// Init initializes the model.
func (m *InboxProcessorModel) Init() tea.Cmd {
	return nil
}

// SetSize sets the view dimensions.
func (m *InboxProcessorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// HasTextInput returns false for the inbox processor.
func (m *InboxProcessorModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true because the inbox processor handles esc/q itself.
func (m *InboxProcessorModel) ShouldCaptureEsc() bool {
	return true
}

// Update handles messages.
func (m *InboxProcessorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle normal keys
		return m.handleKeyMsg(msg)

	case SectionPickerDoneMsg:
		return m.handleSectionPickerDone(msg)

	case TaskUpdatedMsg:
		return m.handleTaskUpdated(msg)

	case TaskPropertyChangedMsg:
		return m.handleTaskUpdated(TaskUpdatedMsg(msg))

	case DueDateChangedMsg:
		return m.handleDueDateChanged(msg)

	case DependencySelectorDoneMsg:
		return m.handleDependencySelectorDone(msg)

	case TagSelectorDoneMsg:
		return m.handleTagSelectorDone(msg)

	case TaskCreatedMsg:
		// Subtask created
		m.statusMessage = "Subtask created"
		return m, m.reloadProject()

	case InboxProjectSavedMsg:
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		// Rebuild inbox tasks to get fresh task references after reload
		m.inboxTasks = getInboxTasksForModel(m.project)
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
func (m *InboxProcessorModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			return CloseInboxProcessorMsg{
				ProcessedCount: len(m.processedIDs),
				SkippedCount:   len(m.skippedIDs),
			}
		}

	case "esc":
		// Close the processor (consistent with other views)
		return m, func() tea.Msg {
			return CloseInboxProcessorMsg{
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

	// Note: h/l/left/right removed - not meaningful for vertical queue navigation

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
		// Set due date (capital D to avoid clash with ,d for done)
		return m, m.openDueDateInput()

	case "b":
		return m, m.openDependencySelector()

	case "x":
		return m, m.deleteCurrentTask()

	case "w":
		// Send to Waiting section
		return m, m.sendToWaiting()

	case "u":
		// Undo last action (Vim-style, consistent with project tree)
		return m, m.undoLastAction()

	case "enter":
		if task := m.getCurrentTask(); task != nil {
			project := m.project
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return m, nil

	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewInboxProcessor,
				ViewName:   "Inbox Processor",
				Items:      m.GetHelpItems(),
			}
		}
	}

	return m, nil
}

// View renders the inbox processor.
func (m *InboxProcessorModel) View() string {
	if len(m.inboxTasks) == 0 {
		return m.renderEmptyInbox()
	}

	var b strings.Builder

	// Header with progress
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Current task display
	if task := m.getCurrentTask(); task != nil {
		b.WriteString(m.renderTaskDisplay(task))
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
func (m *InboxProcessorModel) getProgressStats() (processed, skipped, remaining, total int) {
	total = len(m.inboxTasks)
	processed = len(m.processedIDs)
	skipped = len(m.skippedIDs)
	remaining = total - processed - skipped
	return
}

// getCurrentTask returns the current task being processed.
func (m *InboxProcessorModel) getCurrentTask() *models.Task {
	if m.currentIndex >= 0 && m.currentIndex < len(m.inboxTasks) {
		return m.inboxTasks[m.currentIndex]
	}
	return nil
}

// advanceToNext moves to the next unprocessed task.
// If autoClose is true, closes the processor when no tasks remain.
func (m *InboxProcessorModel) advanceToNext(autoClose bool) tea.Cmd {
	// Clear undo history when changing tasks
	m.processingHistory = []processingAction{}

	// Find next unprocessed task
	for i := m.currentIndex + 1; i < len(m.inboxTasks); i++ {
		taskID := m.inboxTasks[i].ID
		if !m.processedIDs[taskID] && !m.skippedIDs[taskID] {
			m.currentIndex = i
			m.statusMessage = ""
			return nil
		}
	}

	// No more unprocessed tasks
	if autoClose {
		// Only close when coming from a processing action
		if len(m.processedIDs) == 0 && len(m.skippedIDs) == 0 {
			m.statusMessage = "No tasks were processed"
		}

		return func() tea.Msg {
			return CloseInboxProcessorMsg{
				ProcessedCount: len(m.processedIDs),
				SkippedCount:   len(m.skippedIDs),
			}
		}
	}

	// Just stay on current task when navigating
	return nil
}

// goToPrevious moves to the previous task.
func (m *InboxProcessorModel) goToPrevious() {
	m.processingHistory = []processingAction{}
	if m.currentIndex > 0 {
		m.currentIndex--
		m.statusMessage = ""
	}
}

// skipCurrent skips current task and advances.
func (m *InboxProcessorModel) skipCurrent() tea.Cmd {
	if task := m.getCurrentTask(); task != nil {
		m.skippedIDs[task.ID] = true
		delete(m.processedIDs, task.ID)
		m.statusMessage = fmt.Sprintf("Skipped task #%d", task.ShortID)
		return m.advanceToNext(true)
	}
	return nil
}

// captureSnapshot creates a snapshot of task state for undo.
func (m *InboxProcessorModel) captureSnapshot(task *models.Task) *inboxSnapshot {
	return captureInboxSnapshot(task)
}

// recordAction records an action in the undo history.
func (m *InboxProcessorModel) recordAction(actionType string, snapshot *inboxSnapshot) {
	task := m.getCurrentTask()
	if task == nil {
		return
	}

	m.processingHistory = append(m.processingHistory, processingAction{
		taskID:       task.ID,
		actionType:   actionType,
		timestamp:    time.Now(),
		prevSnapshot: snapshot,
	})
}

// undoLastAction undoes the last action on the current task.
func (m *InboxProcessorModel) undoLastAction() tea.Cmd {
	if len(m.processingHistory) == 0 {
		m.statusMessage = "Nothing to undo"
		return nil
	}

	// Pop last action
	action := m.processingHistory[len(m.processingHistory)-1]
	m.processingHistory = m.processingHistory[:len(m.processingHistory)-1]

	// Get current task and verify it matches
	task := m.getCurrentTask()
	if task == nil || task.ID != action.taskID {
		m.err = fmt.Errorf("can only undo actions on current task")
		return nil
	}

	// Restore snapshot
	if action.prevSnapshot != nil {
		restoreInboxSnapshot(task, action.prevSnapshot)
	}

	m.statusMessage = fmt.Sprintf("Undone: %s", action.actionType)
	return m.saveProject()
}

// setPriorityQuick sets priority using quick keys 1-5.
func (m *InboxProcessorModel) setPriorityQuick(key string) tea.Cmd {
	priority, _ := strconv.Atoi(key)

	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	snapshot := m.captureSnapshot(task)
	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.Priority = priority
	})

	m.recordAction(fmt.Sprintf("priority set to %d", priority), snapshot)
	m.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return m.saveProject()
}

// saveProject saves the project with backup.
func (m *InboxProcessorModel) saveProject() tea.Cmd {
	return func() tea.Msg {
		if err := saveProjectSync(m.project); err != nil {
			return InboxProjectSavedMsg{Error: fmt.Errorf("save failed: %w", err)}
		}
		return InboxProjectSavedMsg{Error: nil}
	}
}

// reloadProject reloads the project from storage.
func (m *InboxProcessorModel) reloadProject() tea.Cmd {
	return func() tea.Msg {
		if err := storage.LoadProject(m.project); err != nil {
			return InboxProjectSavedMsg{Error: fmt.Errorf("reload failed: %w", err)}
		}
		return InboxProjectSavedMsg{Error: nil}
	}
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *InboxProcessorModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
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

// changeTaskStatus validates and applies a status transition for the current inbox task.
func (m *InboxProcessorModel) changeTaskStatus(newStatus models.TaskStatus, verb string) tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	if msg := task.CanTransitionTo(newStatus); msg != "" {
		m.statusMessage = msg
		return nil
	}

	// Project-level check: incomplete dependencies block completion
	if newStatus == models.StatusDone {
		incompleteDeps := m.project.GetIncompleteDependencies(task)
		if len(incompleteDeps) > 0 {
			depIDs := make([]string, len(incompleteDeps))
			for i, dep := range incompleteDeps {
				depIDs[i] = fmt.Sprintf("#%d", dep.ShortID)
			}
			m.err = fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
				len(incompleteDeps), strings.Join(depIDs, ", "))
			return nil
		}
	}

	snapshot := m.captureSnapshot(task)
	task.ApplyTransition(newStatus)

	m.recordAction(verb+" task", snapshot)
	m.statusMessage = fmt.Sprintf("Task #%d %s", task.ShortID, verb)

	return m.saveProject()
}

// sendToWaiting moves the current task to Waiting section and marks it processed.
func (m *InboxProcessorModel) sendToWaiting() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	snapshot := m.captureSnapshot(task)
	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.Section = models.SectionWaiting
	})

	m.recordAction("sent to Waiting", snapshot)
	m.statusMessage = fmt.Sprintf("Task #%d sent to Waiting", task.ShortID)

	// Mark as processed (it's leaving inbox)
	m.processedIDs[task.ID] = true

	return tea.Batch(m.saveProject(), m.advanceToNext(true))
}

// GetHelpItems returns help items for the inbox processor view.
func (m *InboxProcessorModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewInboxProcessor)
}

// willLeaveInbox returns true if the task will no longer appear in inbox after save.
func (m *InboxProcessorModel) willLeaveInbox(task *models.Task) bool {
	return willLeaveInbox(task)
}
