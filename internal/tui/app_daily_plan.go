package tui

import (
	"cmp"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// toggleToday adds or removes the focused task from the daily plan.
// Returns a status timeout cmd on success, or nil if no task is available.
func (m *AppModel) toggleToday(capture *outliner.OutlinerRelocationCapture) tea.Cmd {
	view := m.activeView()
	if view == nil {
		return nil
	}
	qt, ok := view.(QueueToggleable)
	if !ok {
		return nil
	}
	task, project := qt.GetTaskRef()
	if task == nil || project == nil {
		return nil
	}
	taskID, projectID := task.ID, project.ID
	identity := taskref.For(task, project)
	add := !m.todayIDs[identity]
	fresh, _, transactionID, err := m.todayLedgerTransition(taskID, projectID, add)
	if err != nil {
		m.feedback.err = err
		return status.ErrorCmd("%s", err)
	}
	successMessage := fmt.Sprintf("Added to today's plan: %s", taskTitleOrFallback(fresh, task))
	if !add {
		successMessage = fmt.Sprintf("Removed from today's plan: %s", taskTitleOrFallback(fresh, task))
	}
	m.lastToggleUndoKind = toggleUndoToday
	m.lastTodayToggleAt = time.Now()
	m.clearToggleRedoHistory()
	return m.finishSynchronousOutlinerRelocation(m.queueToggleResultCmd(), capture, successMessage, transactionID)
}

// todayLedgerTransition delegates both membership and any Inbox/Next
// promotion to the application service.
func (m *AppModel) todayLedgerTransition(taskID, projectID string, add bool) (*models.Task, *models.Project, string, error) {
	action := app.PlanningRemoveToday
	if add {
		action = app.PlanningAddToday
	}
	return m.applyTaskPlanningAction(taskID, projectID, action)
}

// undoTodayToggle restores the last Today transaction through the same
// durable workspace ledger used by Next.
func (m *AppModel) undoTodayToggle() tea.Cmd {
	m.lastTodayToggleAt = time.Time{}
	m.lastTodayRedoAt = time.Now()
	m.lastToggleRedoKind = toggleUndoToday
	m.lastToggleUndoKind = toggleUndoToday
	return m.planningLedgerRestore(false)
}

// redoTodayToggle reapplies the last Today transaction through the durable
// workspace ledger.
func (m *AppModel) redoTodayToggle() tea.Cmd {
	m.lastTodayRedoAt = time.Time{}
	m.lastToggleRedoKind = toggleUndoToday
	m.lastToggleUndoKind = toggleUndoToday
	m.lastTodayToggleAt = time.Now()
	return m.planningLedgerRestore(true)
}

// QueueToggleable is implemented by views that can provide a task for queue toggling.
type QueueToggleable interface {
	GetTaskRef() (*models.Task, *models.Project)
}

// taskIsNext reports explicit project-qualified Next membership.
func (m *AppModel) taskIsNext(task *models.Task, project *models.Project) bool {
	if task == nil || project == nil {
		return false
	}
	return m.nextOrderIDs[taskref.For(task, project)]
}

// queueToggleResultCmd fans the state change out to every task view (mirroring a
// committed save) and schedules the status-line timeout, without clearing the
// toggle-undo history the toggle just recorded.
func (m *AppModel) queueToggleResultCmd() tea.Cmd {
	// Planning actions commit synchronously before returning here. Rebuild the
	// loaded Outliner immediately so Inbox/Next/Backlog rows disappear and the
	// cursor advances before the next render, without waiting for an async reload.
	m.refreshLoadedTaskWorkspace()
	cmds := m.refreshTaskViewsAfterPlanningMutation()
	cmds = append(cmds, status.DefaultStatusTimeout())
	return tea.Batch(cmds...)
}

// toggleQueue adds or removes the focused task from explicit Next membership.
// Adding drops Today and normalizes nonterminal lifecycle state to Backlog;
// removing changes planning only. Because the whole move is a single durable
// ledger entry, undo/redo route through the ledger
// (undoQueueToggle/redoQueueToggle) rather than a bespoke in-memory compensation,
// so they survive a restart and honor LIFO ordering. Returns a status timeout cmd
// on success, or nil if no task is available.
func (m *AppModel) toggleQueue(capture *outliner.OutlinerRelocationCapture) tea.Cmd {
	view := m.activeView()
	if view == nil {
		return nil
	}
	qt, ok := view.(QueueToggleable)
	if !ok {
		return nil
	}
	task, project := qt.GetTaskRef()
	if task == nil || project == nil {
		return nil
	}
	taskID, projectID := task.ID, project.ID

	if m.taskIsNext(task, project) {
		// Removing from Next preserves lifecycle state.
		fresh, _, transactionID, err := m.queueLedgerTransition(taskID, projectID, false)
		if err != nil {
			m.feedback.err = err
			return status.ErrorCmd("%s", err)
		}
		title := taskTitleOrFallback(fresh, task)
		successMessage := fmt.Sprintf("Removed from Next: %s", title)
		m.lastToggleUndoKind = toggleUndoQueue
		m.lastQueueToggleAt = time.Now()
		m.clearToggleRedoHistory()
		return m.finishSynchronousOutlinerRelocation(m.queueToggleResultCmd(), capture, successMessage, transactionID)
	} else {
		// Adding to Next drops Today and appends one ordered membership.
		fresh, _, transactionID, err := m.queueLedgerTransition(taskID, projectID, true)
		if err != nil {
			m.feedback.err = err
			return status.ErrorCmd("%s", err)
		}
		title := taskTitleOrFallback(fresh, task)
		successMessage := fmt.Sprintf("Added to Next: %s", title)
		m.lastToggleUndoKind = toggleUndoQueue
		m.lastQueueToggleAt = time.Now()
		m.clearToggleRedoHistory()
		return m.finishSynchronousOutlinerRelocation(m.queueToggleResultCmd(), capture, successMessage, transactionID)
	}
}

// queueLedgerTransition delegates the whole Next mutation to the application
// service, then reloads the task and planning snapshots from the committed state.
func (m *AppModel) queueLedgerTransition(taskID, projectID string, appendOrder bool) (*models.Task, *models.Project, string, error) {
	action := app.PlanningRemoveNext
	if appendOrder {
		action = app.PlanningAddNext
	}
	return m.applyTaskPlanningAction(taskID, projectID, action)
}

// undoQueueToggle reverses the last Next-queue toggle by undoing its durable
// workspace-ledger transaction, restoring the workflow state, Today membership,
// and Next ordering together. Unlike the retired in-memory compensation, it reads
// the durable ledger, so it works after a restart and always undoes the newest
// transaction first.
func (m *AppModel) undoQueueToggle() tea.Cmd {
	m.lastQueueToggleAt = time.Time{}
	// The workspace ledger is the durable LIFO history: keep routing u/ctrl+r
	// through it so consecutive presses walk the stack (newest first) and a
	// following ctrl+r replays, mirroring the persistent undo contract.
	m.lastToggleRedoKind = toggleUndoQueue
	m.lastQueueRedoAt = time.Now()
	m.lastToggleUndoKind = toggleUndoQueue
	return m.planningLedgerRestore(false)
}

// redoQueueToggle replays the last undone Next-queue toggle through the workspace
// ledger's redo stack.
func (m *AppModel) redoQueueToggle() tea.Cmd {
	m.lastQueueRedoAt = time.Now()
	m.lastToggleRedoKind = toggleUndoQueue
	m.lastToggleUndoKind = toggleUndoQueue
	m.lastQueueToggleAt = time.Time{}
	return m.planningLedgerRestore(true)
}

// taskTitleOrFallback returns the fresh task title, falling back to the
// pre-reload pointer when the reload could not resolve the task.
func taskTitleOrFallback(fresh, fallback *models.Task) string {
	if fresh != nil {
		return fresh.Title
	}
	if fallback != nil {
		return fallback.Title
	}
	return ""
}

type toggleUndoKind uint8

const (
	toggleUndoNone toggleUndoKind = iota
	toggleUndoToday
	toggleUndoQueue
)

func (m *AppModel) clearToggleRedoHistory() {
	m.lastTodayRedoAt = time.Time{}
	m.lastQueueRedoAt = time.Time{}
	m.lastToggleRedoKind = toggleUndoNone
}

func (m *AppModel) clearToggleHistory() {
	m.lastTodayToggleAt = time.Time{}
	m.lastQueueToggleAt = time.Time{}
	m.lastToggleUndoKind = toggleUndoNone
	m.clearToggleRedoHistory()
}

func (m *AppModel) toggleUndoTimestamp(kind toggleUndoKind) time.Time {
	switch kind {
	case toggleUndoToday:
		return m.lastTodayToggleAt
	case toggleUndoQueue:
		return m.lastQueueToggleAt
	default:
		return time.Time{}
	}
}

func (m *AppModel) toggleRedoTimestamp(kind toggleUndoKind) time.Time {
	switch kind {
	case toggleUndoToday:
		return m.lastTodayRedoAt
	case toggleUndoQueue:
		return m.lastQueueRedoAt
	default:
		return time.Time{}
	}
}

type pinnedTaskInfo struct {
	Title       string
	ShortID     int
	ProjectName string
	PinnedAt    time.Time
	Task        *models.Task
}

func (m *AppModel) collectPinnedTasks() []pinnedTaskInfo {
	projects := m.taskWorkspaceProjects()
	var pinned []pinnedTaskInfo
	now := time.Now()

	for _, project := range projects {
		if project.Tasks == nil {
			if err := storage.LoadProject(project); err != nil {
				continue
			}
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			if task.Pinned && !task.IsTerminalState() && !task.IsDeferredAt(now) {
				info := pinnedTaskInfo{
					Title:       task.Title,
					ShortID:     task.ShortID,
					ProjectName: project.Name,
					Task:        task,
				}
				if task.PinnedAt != nil {
					info.PinnedAt = *task.PinnedAt
				}
				pinned = append(pinned, info)
			}
		}
	}

	sort.SliceStable(pinned, func(i, j int) bool {
		switch {
		case !pinned[i].PinnedAt.IsZero() && pinned[j].PinnedAt.IsZero():
			return true
		case pinned[i].PinnedAt.IsZero() && !pinned[j].PinnedAt.IsZero():
			return false
		case !pinned[i].PinnedAt.Equal(pinned[j].PinnedAt):
			return pinned[i].PinnedAt.After(pinned[j].PinnedAt)
		}
		if c := cmp.Compare(strings.ToLower(pinned[i].ProjectName), strings.ToLower(pinned[j].ProjectName)); c != 0 {
			return c < 0
		}
		if c := cmp.Compare(pinned[i].ShortID, pinned[j].ShortID); c != 0 {
			return c < 0
		}
		return pinned[i].Task.ID < pinned[j].Task.ID
	})

	return pinned
}
