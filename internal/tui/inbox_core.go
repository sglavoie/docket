package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// inboxBackend abstracts the differences between local and global inbox processors.
// Each model implements this interface to provide view-specific data access,
// persistence, rendering strings, and behavioral hooks.
type inboxBackend interface {
	// Data access
	taskCount() int
	taskRef(index int) (task *models.Task, project *models.Project)

	// Messages
	closeMsg(processed, skipped int) tea.Msg
	handleSavedMsg(msg tea.Msg) (matched bool, err error)

	// Persistence
	saveCmd() tea.Cmd
	reloadCmd() tea.Cmd
	rebuildTasks()
	deleteFromData(task *models.Task, project *models.Project)

	// Behavioral hooks
	markDeletedProcessed() bool
	shouldProcessOnSectionChange(task *models.Task, project *models.Project) bool
	shouldProcessOnStatusChange(newStatus models.TaskStatus) bool

	// Rendering
	showProjectInHeader() bool
	headerTitle() string
	headerSubtitleExtra() string
	emptyStateText() string
	emptyInboxText() string
	returnHint() string
	viewType() ViewType
	helpViewName() string
}

// inboxCore holds all state and logic shared between InboxProcessorModel and
// GlobalInboxProcessorModel. Each model embeds this struct and sets the backend
// field to itself in its constructor.
type inboxCore struct {
	BaseView
	InteractiveViewState
	config        *config.Config
	configManager *config.Manager

	currentIndex      int
	processedIDs      map[string]bool
	skippedIDs        map[string]bool
	processingHistory []processingAction

	backend inboxBackend
	model   tea.Model // pointer to the outer (embedding) model, for Update returns
}

// processingAction records an action for undo support.
type processingAction struct {
	taskID       string
	projectID    string
	actionType   string
	timestamp    time.Time
	prevSnapshot *inboxSnapshot
}

// initCore initializes the shared core fields. Call from the outer model's constructor.
func (c *inboxCore) initCore(backend inboxBackend, model tea.Model, cfg *config.Config, manager *config.Manager) {
	c.config = cfg
	c.configManager = manager
	c.currentIndex = 0
	c.processedIDs = make(map[string]bool)
	c.skippedIDs = make(map[string]bool)
	c.processingHistory = []processingAction{}
	c.backend = backend
	c.model = model
}

// --- tea.Model implementation ---

// Init initializes the model.
func (c *inboxCore) Init() tea.Cmd {
	return nil
}

// ShouldCaptureEsc returns true when prefix mode or confirm dialog is active.
func (c *inboxCore) ShouldCaptureEsc() bool {
	return c.DefaultShouldCaptureEsc()
}

// Update handles messages for the inbox processor.
func (c *inboxCore) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return c.handleKeyMsg(msg)

	case SectionPickerDoneMsg:
		return c.model, c.handleSectionPickerDone(msg)

	case TaskUpdatedMsg:
		return c.model, inboxHandleTaskUpdated(c)

	case TaskPropertyChangedMsg:
		return c.model, inboxHandleTaskUpdated(c)

	case DueDateChangedMsg:
		return c.model, inboxHandleDueDateChanged(c)

	case DependencySelectorDoneMsg:
		return c.model, inboxHandleDependencySelectorDone(c)

	case TagSelectorDoneMsg:
		_, cmd := inboxHandleTagSelectorDone(c, msg)
		return c.model, cmd

	case TaskCreatedMsg:
		c.statusMessage = "Subtask created"
		return c.model, c.backend.reloadCmd()

	case ErrorMsg:
		c.err = msg.Err
		return c.model, nil

	default:
		if matched, err := c.backend.handleSavedMsg(msg); matched {
			if err != nil {
				c.err = err
				return c.model, nil
			}
			c.backend.rebuildTasks()
			c.clampCurrentIndex()
			return c.model, nil
		}
	}

	return c.model, nil
}

// View renders the inbox processor.
func (c *inboxCore) View() string {
	if c.backend.taskCount() == 0 {
		return c.renderEmptyInbox()
	}

	var b strings.Builder

	b.WriteString(c.renderHeader())
	b.WriteString("\n")

	task, _ := c.currentTaskRef()
	if task != nil {
		b.WriteString(c.renderTaskDisplay())
	} else {
		b.WriteString(c.renderEmptyState())
	}

	b.WriteString("\n")
	b.WriteString(renderInboxOperationsPanel())
	b.WriteString("\n\n")
	b.WriteString(renderInboxFooter(c.width))

	if c.statusMessage != "" {
		b.WriteString("\n")
		b.WriteString(SuccessStyle.Render(c.statusMessage))
	}

	if c.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(c.err.Error()))
	}

	return b.String()
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (c *inboxCore) GetPrefixPopupInfo() *PrefixPopupInfo {
	if c.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: c.pendingPrefix,
		Hints: PrefixHints{
			",": StatusPrefixHints,
			"g": NavigationPrefixHints,
		},
	}
}

// GetHelpItems returns help items for this view.
func (c *inboxCore) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(c.backend.viewType())
}

// --- Key handling ---

func (c *inboxCore) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if confirmed, handled := c.confirm.HandleKey(key); handled {
		if confirmed {
			return c.model, c.executeDelete()
		}
		c.statusMessage = ""
		return c.model, nil
	}

	if c.pendingPrefix != "" {
		return c.handlePrefixKey(key)
	}

	c.err = nil

	if key == "," || key == "g" {
		c.pendingPrefix = key
		return c.model, nil
	}

	switch key {
	case "q", "esc":
		return c.model, func() tea.Msg {
			return c.backend.closeMsg(len(c.processedIDs), len(c.skippedIDs))
		}

	case "n":
		return c.model, c.skipCurrent()

	case "j", "down":
		return c.model, c.advanceToNext(false)

	case "k", "up":
		c.goToPrevious()
		return c.model, nil

	case "G":
		if c.backend.taskCount() > 0 {
			c.currentIndex = c.backend.taskCount() - 1
		}
		return c.model, nil

	case "s":
		return c.model, inboxOpenSectionPicker(c)

	case "1", "2", "3", "4", "5":
		return c.model, c.setPriorityQuick(key)

	case "t":
		return c.model, inboxOpenTagSelector(c)

	case "e":
		return c.model, inboxOpenTaskEditor(c)

	case "c":
		return c.model, inboxOpenSubtaskCreator(c)

	case "d":
		return c.model, inboxOpenDueDateInput(c)

	case "D":
		inboxDeletePrompt(c, &c.confirm)
		return c.model, nil

	case "b":
		return c.model, inboxOpenDependencySelector(c)

	case "w":
		return c.model, c.sendToWaiting()

	case "u":
		return c.model, c.undoLastAction()

	case "enter":
		task, project := c.currentTaskRef()
		if task != nil {
			return c.model, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return c.model, nil

	case "?":
		return c.model, helpCmd(c.backend.viewType(), c.backend.helpViewName(), c.GetHelpItems())
	}

	return c.model, nil
}

func (c *inboxCore) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := c.pendingPrefix
	c.pendingPrefix = ""

	if key == "esc" {
		return c.model, nil
	}

	switch prefix {
	case ",":
		if status, ok := StatusForPrefixKey(key); ok {
			return c.model, c.changeTaskStatus(status, StatusVerb(status))
		}
	case "g":
		if key == "g" {
			c.currentIndex = 0
			return c.model, nil
		}
	}

	return c.model, nil
}

// --- Navigation ---

func (c *inboxCore) currentTaskRef() (task *models.Task, project *models.Project) {
	if c.currentIndex >= 0 && c.currentIndex < c.backend.taskCount() {
		return c.backend.taskRef(c.currentIndex)
	}
	return nil, nil
}

func (c *inboxCore) clampCurrentIndex() {
	count := c.backend.taskCount()
	if c.currentIndex >= count {
		c.currentIndex = count - 1
	}
	if c.currentIndex < 0 {
		c.currentIndex = 0
	}
}

func (c *inboxCore) advanceToNext(autoClose bool) tea.Cmd {
	c.processingHistory = []processingAction{}

	for i := c.currentIndex + 1; i < c.backend.taskCount(); i++ {
		task, _ := c.backend.taskRef(i)
		if !c.processedIDs[task.ID] && !c.skippedIDs[task.ID] {
			c.currentIndex = i
			c.statusMessage = ""
			return nil
		}
	}

	if autoClose {
		if len(c.processedIDs) == 0 && len(c.skippedIDs) == 0 {
			c.statusMessage = "No tasks were processed"
		}
		return func() tea.Msg {
			return c.backend.closeMsg(len(c.processedIDs), len(c.skippedIDs))
		}
	}

	return nil
}

func (c *inboxCore) goToPrevious() {
	c.processingHistory = []processingAction{}
	if c.currentIndex > 0 {
		c.currentIndex--
		c.statusMessage = ""
	}
}

func (c *inboxCore) skipCurrent() tea.Cmd {
	task, _ := c.currentTaskRef()
	if task != nil {
		c.skippedIDs[task.ID] = true
		delete(c.processedIDs, task.ID)
		c.statusMessage = fmt.Sprintf("Skipped task #%d", task.ShortID)
		return c.advanceToNext(true)
	}
	return nil
}

func (c *inboxCore) getProgressStats() (processed, skipped, remaining, total int) {
	total = c.backend.taskCount()
	processed = len(c.processedIDs)
	skipped = len(c.skippedIDs)
	remaining = total - processed - skipped
	return
}

// --- Undo ---

func (c *inboxCore) recordAction(actionType string, snapshot *inboxSnapshot) {
	task, project := c.currentTaskRef()
	if task == nil {
		return
	}
	projectID := ""
	if project != nil {
		projectID = project.ID
	}
	c.processingHistory = append(c.processingHistory, processingAction{
		taskID:       task.ID,
		projectID:    projectID,
		actionType:   actionType,
		timestamp:    time.Now(),
		prevSnapshot: snapshot,
	})
}

func (c *inboxCore) undoLastAction() tea.Cmd {
	if len(c.processingHistory) == 0 {
		c.statusMessage = "Nothing to undo"
		return nil
	}

	action := c.processingHistory[len(c.processingHistory)-1]
	c.processingHistory = c.processingHistory[:len(c.processingHistory)-1]

	task, _ := c.currentTaskRef()
	if task == nil || task.ID != action.taskID {
		c.err = fmt.Errorf("can only undo actions on current task")
		return nil
	}

	if action.prevSnapshot != nil {
		restoreInboxSnapshot(task, action.prevSnapshot)
	}

	c.statusMessage = fmt.Sprintf("Undone: %s", action.actionType)
	return c.backend.saveCmd()
}

// --- Operations ---

func (c *inboxCore) setPriorityQuick(key string) tea.Cmd {
	priority, _ := strconv.Atoi(key)

	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}

	snapshot := captureInboxSnapshot(task)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Priority = priority
	})

	c.recordAction(fmt.Sprintf("priority set to %d", priority), snapshot)
	c.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return c.backend.saveCmd()
}

func (c *inboxCore) changeTaskStatus(newStatus models.TaskStatus, verb string) tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}

	current := project.GetTaskByID(task.ID)
	if current == nil {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	if msg := current.CanTransitionTo(newStatus); msg != "" {
		c.statusMessage = msg
		return nil
	}

	if newStatus == models.StatusDone {
		incompleteDeps := project.GetIncompleteDependencies(current)
		if len(incompleteDeps) > 0 {
			depIDs := make([]string, len(incompleteDeps))
			for i, dep := range incompleteDeps {
				depIDs[i] = fmt.Sprintf("#%d", dep.ShortID)
			}
			c.err = fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
				len(incompleteDeps), strings.Join(depIDs, ", "))
			return nil
		}
	}

	snapshot := captureInboxSnapshot(current)
	current.ApplyTransition(newStatus)

	c.recordAction(verb+" task", snapshot)
	c.statusMessage = fmt.Sprintf("Task #%d %s", current.ShortID, verb)

	if c.backend.shouldProcessOnStatusChange(newStatus) {
		c.processedIDs[current.ID] = true
		return tea.Batch(c.backend.saveCmd(), c.advanceToNext(true))
	}

	return c.backend.saveCmd()
}

func (c *inboxCore) sendToWaiting() tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}

	current := project.GetTaskByID(task.ID)
	if current == nil {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	snapshot := captureInboxSnapshot(current)
	if !project.MutateTask(task.ID, func(t *models.Task) {
		t.Section = models.SectionWaiting
	}) {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	c.recordAction("sent to Waiting", snapshot)
	c.statusMessage = fmt.Sprintf("Task #%d sent to Waiting", current.ShortID)
	c.processedIDs[task.ID] = true

	return tea.Batch(c.backend.saveCmd(), c.advanceToNext(true))
}

func (c *inboxCore) handleSectionPickerDone(msg SectionPickerDoneMsg) tea.Cmd {
	if !inboxHandleSectionPickerCore(c, msg) {
		return nil
	}
	task, project := c.currentTaskRef()
	if task != nil && c.backend.shouldProcessOnSectionChange(task, project) {
		c.processedIDs[task.ID] = true
		return tea.Batch(c.backend.saveCmd(), c.advanceToNext(true))
	}
	return c.backend.saveCmd()
}

func (c *inboxCore) executeDelete() tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}

	c.backend.deleteFromData(task, project)

	if c.backend.markDeletedProcessed() {
		c.processedIDs[task.ID] = true
	}

	c.backend.rebuildTasks()
	c.clampCurrentIndex()
	c.statusMessage = "Task deleted"

	return c.backend.saveCmd()
}

// --- inboxOpsView implementation (used by inbox_operations_shared.go) ---

func (c *inboxCore) getInboxTaskRef() (*models.Task, *models.Project) {
	return c.currentTaskRef()
}

func (c *inboxCore) inboxSave() tea.Cmd        { return c.backend.saveCmd() }
func (c *inboxCore) inboxReload() tea.Cmd      { return c.backend.reloadCmd() }
func (c *inboxCore) setInboxStatus(msg string) { c.statusMessage = msg }
func (c *inboxCore) inboxCaptureSnapshot(task *models.Task) *inboxSnapshot {
	return captureInboxSnapshot(task)
}
func (c *inboxCore) inboxRecordAction(desc string, snapshot *inboxSnapshot) {
	c.recordAction(desc, snapshot)
}

// --- Rendering ---

func (c *inboxCore) renderHeader() string {
	processed, skipped, remaining, total := c.getProgressStats()
	subtitle := fmt.Sprintf("%d/%d tasks%s (%d processed, %d skipped, %d remaining)",
		c.currentIndex+1, total, c.backend.headerSubtitleExtra(), processed, skipped, remaining)
	title := renderFullscreenTitle(c.backend.headerTitle(), subtitle)

	var percent float64
	if total > 0 {
		percent = float64(c.currentIndex+1) / float64(total) * 100
	}
	barWidth := 30
	bar := RenderProgressBar(percent, barWidth)
	progressBar := fmt.Sprintf("%s %.0f%%", bar, percent)

	return fmt.Sprintf("%s\n%s", title, progressBar)
}

func (c *inboxCore) renderTaskDisplay() string {
	task, project := c.currentTaskRef()
	projectName := ""
	if c.backend.showProjectInHeader() {
		projectName = project.Name
	}
	return renderInboxTaskDisplay(task, c.width, willLeaveInbox(task), projectName, c.config)
}

func (c *inboxCore) renderEmptyState() string {
	content := SuccessStyle.Render(c.backend.emptyStateText())
	return BoxStyle.Render(content)
}

func (c *inboxCore) renderEmptyInbox() string {
	var b strings.Builder
	b.WriteString(renderFullscreenTitle(c.backend.headerTitle(), ""))
	b.WriteString("\n\n")
	content := SuccessStyle.Render(c.backend.emptyInboxText())
	b.WriteString(BoxStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(HelpKeyStyle.Render("q") + MutedStyle.Render(": "+c.backend.returnHint()))
	return b.String()
}
