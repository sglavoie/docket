package tui

import (
	"fmt"
	"os"
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

	// Delete confirmation state
	confirmDelete bool
}

// processingAction records an action for undo support.
type processingAction struct {
	taskID       string
	actionType   string // "section", "priority", "tags", "delete", etc.
	timestamp    time.Time
	prevSnapshot *taskSnapshot
}

// taskSnapshot stores task state for undo.
type taskSnapshot struct {
	section   string
	priority  int
	tags      []string
	dueDate   *time.Time
	dependsOn []string
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
		if task.Section == models.SectionInbox && task.IsRoot() {
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
		// DEBUG: Log current task after rebuild
		if task := m.getCurrentTask(); task != nil {
			fmt.Fprintf(os.Stderr, "[InboxProcessor] After reload, current task %s tags: %v\n", task.ID, task.Tags)
		}
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

	// Handle delete confirmation
	if m.confirmDelete {
		switch key {
		case "y", "Y":
			m.confirmDelete = false
			return m, m.executeDelete()
		case "n", "N", "esc":
			m.confirmDelete = false
			m.statusMessage = ""
			return m, nil
		}
		return m, nil
	}

	// Clear previous error
	m.err = nil

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
		// Only handle escape in confirmation dialogs
		// User must use 'q' to exit the processor
		return m, nil

	case "n":
		return m, m.skipCurrent()

	case "left", "h":
		m.goToPrevious()
		return m, nil

	case "right", "l", "j":
		return m, m.advanceToNext(false)

	case "up", "k":
		m.goToPrevious()
		return m, nil

	case "down":
		return m, m.advanceToNext(false)

	// Operations
	case "s":
		return m, m.openSectionPicker()

	case "1", "2", "3", "4", "5":
		return m, m.setPriorityQuick(key)

	case "t":
		return m, m.openTagSelector()

	case "e":
		return m, m.openTaskEditor()

	case "c":
		return m, m.openSubtaskCreator()

	case "u":
		return m, m.openDueDateInput()

	case "b":
		return m, m.openDependencySelector()

	case "x":
		return m, m.deleteCurrentTask()

	case "z":
		return m, m.undoLastAction()
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
	b.WriteString("\n\n")

	// Current task display
	if task := m.getCurrentTask(); task != nil {
		b.WriteString(m.renderTaskDisplay(task))
	} else {
		b.WriteString(m.renderEmptyState())
	}

	b.WriteString("\n\n")

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

// markProcessed marks current task as processed and advances.
func (m *InboxProcessorModel) markProcessed() tea.Cmd {
	if task := m.getCurrentTask(); task != nil {
		m.processedIDs[task.ID] = true
		delete(m.skippedIDs, task.ID)
		m.statusMessage = fmt.Sprintf("Processed task #%d", task.ShortID)
		return m.advanceToNext(true)
	}
	return nil
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
func (m *InboxProcessorModel) captureSnapshot(task *models.Task) *taskSnapshot {
	var dueDate *time.Time
	if task.DueDate != nil {
		d := *task.DueDate
		dueDate = &d
	}

	return &taskSnapshot{
		section:   task.Section,
		priority:  task.Priority,
		tags:      append([]string{}, task.Tags...),
		dueDate:   dueDate,
		dependsOn: append([]string{}, task.DependsOn...),
	}
}

// recordAction records an action in the undo history.
func (m *InboxProcessorModel) recordAction(actionType string, snapshot *taskSnapshot) {
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
		task.Section = action.prevSnapshot.section
		task.Priority = action.prevSnapshot.priority
		task.Tags = append([]string{}, action.prevSnapshot.tags...)
		task.DependsOn = append([]string{}, action.prevSnapshot.dependsOn...)
		if action.prevSnapshot.dueDate != nil {
			d := *action.prevSnapshot.dueDate
			task.DueDate = &d
		} else {
			task.DueDate = nil
		}
		task.UpdatedAt = time.Now()
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
	task.Priority = priority
	task.UpdatedAt = time.Now()

	m.recordAction(fmt.Sprintf("priority set to %d", priority), snapshot)
	m.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return m.saveProject()
}

// applyTags applies comma-separated tags to current task.
func (m *InboxProcessorModel) applyTags(tagInput string) tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	snapshot := m.captureSnapshot(task)

	// Parse comma-separated tags
	tags := strings.Split(tagInput, ",")
	task.Tags = []string{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			task.AddTag(tag)
		}
	}

	m.recordAction("tags updated", snapshot)
	m.statusMessage = fmt.Sprintf("Tags set to: %s", strings.Join(task.Tags, ", "))

	return m.saveProject()
}

// saveProject saves the project with backup.
func (m *InboxProcessorModel) saveProject() tea.Cmd {
	return func() tea.Msg {
		if err := storage.BackupProject(m.project); err != nil {
			return InboxProjectSavedMsg{Error: fmt.Errorf("backup failed: %w", err)}
		}
		if err := storage.SaveProject(m.project); err != nil {
			return InboxProjectSavedMsg{Error: fmt.Errorf("save failed: %w", err)}
		}
		if err := storage.LoadProject(m.project); err != nil {
			return InboxProjectSavedMsg{Error: fmt.Errorf("reload failed: %w", err)}
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

// GetHelpItems returns help items for the inbox processor view.
func (m *InboxProcessorModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "n", Description: "Skip task (move to next without processing)", Category: "Navigation"},
		{Key: "←/→ or h/l", Description: "Navigate to previous/next task", Category: "Navigation"},
		{Key: "q", Description: "Quit inbox processing", Category: "Navigation"},
		{Key: "s", Description: "Assign section", Category: "Operations"},
		{Key: "1-5", Description: "Set priority (quick)", Category: "Operations"},
		{Key: "t", Description: "Manage tags", Category: "Operations"},
		{Key: "c", Description: "Create subtask", Category: "Operations"},
		{Key: "e", Description: "Edit task (title/description)", Category: "Operations"},
		{Key: "u", Description: "Set due date", Category: "Operations"},
		{Key: "b", Description: "Manage dependencies (blockers)", Category: "Operations"},
		{Key: "x", Description: "Delete task (with confirmation)", Category: "Operations"},
		{Key: "z", Description: "Undo last operation on current task", Category: "Operations"},
	}
}
