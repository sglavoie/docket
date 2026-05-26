package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

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

	// Also search backward so that processing/skipping the last item does not
	// close the session while earlier unprocessed items remain.
	for i := 0; i < c.currentIndex; i++ {
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
