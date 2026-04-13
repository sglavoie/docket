package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
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
