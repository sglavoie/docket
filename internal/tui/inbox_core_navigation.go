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
	// Note: processingHistory is intentionally preserved so undo works across
	// auto-advance (a single press of u undoes the action that just moved the
	// previous task out of the current slot).

	for i := c.currentIndex + 1; i < c.backend.taskCount(); i++ {
		task, project := c.backend.taskRef(i)
		identity := taskIdentityFor(task, project)
		if !c.processedIDs[identity] && !c.skippedIDs[identity] {
			c.currentIndex = i
			c.pendingFocus = identity
			c.statusMessage = ""
			return nil
		}
	}

	// Also search backward so that processing/skipping the last item does not
	// close the session while earlier unprocessed items remain.
	for i := 0; i < c.currentIndex; i++ {
		task, project := c.backend.taskRef(i)
		identity := taskIdentityFor(task, project)
		if !c.processedIDs[identity] && !c.skippedIDs[identity] {
			c.currentIndex = i
			c.pendingFocus = identity
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
	if c.currentIndex > 0 {
		c.currentIndex--
		if task, project := c.currentTaskRef(); task != nil {
			c.pendingFocus = taskIdentityFor(task, project)
		}
		c.statusMessage = ""
	}
}

// realignCurrentIndex repositions the cursor onto pendingFocus after a
// rebuild. The task list shifts whenever a save triggers rebuildTasks (deleted
// or section-changed tasks leave the queue), so the raw currentIndex no
// longer points at the task the user expects. If pendingFocus is valid and
// matches a current task, we land on it; otherwise we clamp to the nearest
// valid slot.
func (c *inboxCore) realignCurrentIndex() {
	if c.pendingFocus.Valid() {
		for i := 0; i < c.backend.taskCount(); i++ {
			task, project := c.backend.taskRef(i)
			if taskIdentityFor(task, project) == c.pendingFocus {
				c.currentIndex = i
				c.pendingFocus = TaskIdentity{}
				return
			}
		}
		c.pendingFocus = TaskIdentity{}
	}
	c.clampCurrentIndex()
}

func (c *inboxCore) skipCurrent() tea.Cmd {
	task, project := c.currentTaskRef()
	if task != nil {
		identity := taskIdentityFor(task, project)
		c.skippedIDs[identity] = true
		delete(c.processedIDs, identity)
		// Record without a snapshot — skip mutates no task state, so undo only
		// needs to remove the identity from skippedIDs and refocus.
		c.recordAction("skip", nil)
		c.statusMessage = fmt.Sprintf("Skipped task #%d", task.ShortID)
		return c.advanceToNext(true)
	}
	return nil
}

// getProgressStats reports counts that satisfy the invariant
// `processed + skipped + remaining = total`. `remaining` counts tasks still in
// the queue that are neither processed nor skipped; `total` is derived from
// the three buckets rather than from taskCount so the figure stays stable when
// processed tasks leave the inbox via rebuildTasks (delete, section change,
// status completion).
func (c *inboxCore) getProgressStats() (processed, skipped, remaining, total int) {
	processed = len(c.processedIDs)
	skipped = len(c.skippedIDs)
	for i := 0; i < c.backend.taskCount(); i++ {
		task, project := c.backend.taskRef(i)
		identity := taskIdentityFor(task, project)
		if !c.processedIDs[identity] && !c.skippedIDs[identity] {
			remaining++
		}
	}
	total = processed + skipped + remaining
	return
}
