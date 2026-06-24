package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

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

	// Drop the action's task from processed/skipped first so the task
	// re-enters the remaining queue regardless of whether a snapshot exists.
	delete(c.processedIDs, action.taskID)
	delete(c.skippedIDs, action.taskID)

	// Refocus the cursor onto the restored task once the next rebuild lands
	// (saves trigger rebuildTasks, which reorders the queue).
	c.pendingFocusID = action.taskID

	if action.prevSnapshot == nil {
		// Skip undo: nothing to restore on disk. Realign now since no rebuild
		// will fire and surface a clear status line.
		c.realignCurrentIndex()
		c.statusMessage = fmt.Sprintf("Undone: %s", action.actionType)
		return nil
	}

	task, project := c.backend.findTaskByID(action.taskID, action.projectID)
	if task == nil || project == nil {
		// The action's task is no longer reachable (e.g., the project file
		// was removed). The history pop already happened so a second u steps
		// back further.
		c.err = fmt.Errorf("cannot undo: task not found")
		c.pendingFocusID = ""
		return nil
	}
	restoreInboxSnapshot(task, action.prevSnapshot)

	c.statusMessage = fmt.Sprintf("Undone: %s", action.actionType)
	// Save the action's project (not the current cursor's project — those may
	// differ in the global processor after auto-advance).
	return c.backend.saveProjectCmd(project)
}

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
		if project.HasIncompleteDependencies(current) {
			incompleteDeps := project.GetIncompleteDependencies(current)
			depIDs := make([]string, 0, len(incompleteDeps))
			for _, dep := range incompleteDeps {
				depIDs = append(depIDs, fmt.Sprintf("#%d", dep.ShortID))
			}
			// Count cross-project deps not included in GetIncompleteDependencies.
			crossCount := 0
			for _, depID := range current.DependsOn {
				if models.ParseTaskRef(depID).IsCrossProject() {
					crossCount++
				}
			}
			if crossCount > 0 {
				depIDs = append(depIDs, fmt.Sprintf("%d cross-project", crossCount))
			}
			c.err = fmt.Errorf("cannot complete: incomplete dependencies (%s)",
				strings.Join(depIDs, ", "))
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
		t.SetSection(models.SectionWaiting)
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

	saveCmd := c.backend.saveCmd()
	c.backend.deleteFromData(task, project)
	c.statusMessage = "Task deleted"

	if c.backend.markDeletedProcessed() {
		c.processedIDs[task.ID] = true
		return tea.Batch(saveCmd, c.advanceToNext(true))
	}

	c.backend.rebuildTasks()
	c.clampCurrentIndex()

	return saveCmd
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

func (c *inboxCore) changePriority(priority int) tea.Cmd {
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

func (c *inboxCore) setQuickDueDate(days int) tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}
	current := project.GetTaskByID(task.ID)
	if current == nil {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}
	dueDate := util.StartOfDay(time.Now()).AddDate(0, 0, days)
	snapshot := captureInboxSnapshot(current)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = &dueDate
		t.HasTime = false
	})
	c.recordAction("due date set", snapshot)
	c.statusMessage = fmt.Sprintf("Due date set to %s", dueDate.Format("Jan 2"))
	return c.backend.saveCmd()
}

func (c *inboxCore) clearDueDate() tea.Cmd {
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
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = nil
		t.HasTime = false
	})
	c.recordAction("due date cleared", snapshot)
	c.statusMessage = "Due date cleared"
	return c.backend.saveCmd()
}

func (c *inboxCore) setSnooze(snoozeDate *time.Time) tea.Cmd {
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
	project.MutateTask(task.ID, func(t *models.Task) {
		t.SnoozedUntil = snoozeDate
	})
	c.recordAction("snooze updated", snapshot)
	if snoozeDate == nil {
		c.statusMessage = "Snooze cleared"
	} else {
		c.statusMessage = fmt.Sprintf("Snoozed until %s", snoozeDate.Format("Mon Jan 2"))
	}
	return c.backend.saveCmd()
}

func (c *inboxCore) setStage(stage string) tea.Cmd {
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
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Stage = stage
	})
	c.recordAction("stage updated", snapshot)
	if stage == "" {
		c.statusMessage = "Stage cleared"
	} else {
		c.statusMessage = fmt.Sprintf("Stage set to %s", stage)
	}
	return c.backend.saveCmd()
}

func (c *inboxCore) moveToSection(section string) tea.Cmd {
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
		t.SetSection(section)
	}) {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}
	sectionName := section
	if sectionName == "" {
		sectionName = "Active"
	}
	c.recordAction(fmt.Sprintf("moved to %s", sectionName), snapshot)
	c.statusMessage = fmt.Sprintf("Moved to %s", sectionName)
	if c.backend.shouldProcessOnSectionChange(task, project) {
		c.processedIDs[task.ID] = true
		return tea.Batch(c.backend.saveCmd(), c.advanceToNext(true))
	}
	return c.backend.saveCmd()
}

func (c *inboxCore) openSnoozePicker() tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}
	return func() tea.Msg {
		return SetSnoozeDateMsg{Task: task, Project: project}
	}
}
