package coordinator

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// AttachmentCoordinator backs the task attachment overlay.
type AttachmentCoordinator struct {
	attach AttachmentMutator
}

// NewAttachmentCoordinator binds an attachment mutator. It is the only way to
// populate the coordinator from outside this package, which is what lets the
// attachment-panel tests drive the overlay against a recording double rather
// than a whole application service.
func NewAttachmentCoordinator(mutator AttachmentMutator) AttachmentCoordinator {
	return AttachmentCoordinator{attach: mutator}
}

// AttachURL records a URL attachment on a task.
func (c AttachmentCoordinator) AttachURL(task taskref.TaskIdentity, url string) tea.Cmd {
	return runTask("Attach URL", func() (app.TaskResult, error) {
		return c.attach.AttachURL(task.Locator(), url)
	})
}

// AttachCodeReference records a file:line code reference on a task.
func (c AttachmentCoordinator) AttachCodeReference(task taskref.TaskIdentity, path string, line int) tea.Cmd {
	return runTask("Attach Code Ref", func() (app.TaskResult, error) {
		return c.attach.AttachCodeReference(task.Locator(), path, line)
	})
}

// AttachFile ingests an external file into a task's attachments as one transaction.
func (c AttachmentCoordinator) AttachFile(task taskref.TaskIdentity, sourcePath string) tea.Cmd {
	return runTask("Attach File", func() (app.TaskResult, error) {
		res, err := c.attach.AttachFile(task.Locator(), sourcePath)
		return res.Task, err
	})
}

// RemoveAttachment drops an attachment (and optionally its bytes) as one transaction.
func (c AttachmentCoordinator) RemoveAttachment(task taskref.TaskIdentity, index int, deleteFile bool) tea.Cmd {
	return runTask("Remove Attachment", func() (app.TaskResult, error) {
		res, err := c.attach.RemoveAttachment(task.Locator(), index, deleteFile)
		return res.Task, err
	})
}

// Reorder swaps two attachments' positions as one transaction.
func (c AttachmentCoordinator) Reorder(task taskref.TaskIdentity, from, to int) tea.Cmd {
	return runTask("Reorder Attachments", func() (app.TaskResult, error) {
		return c.attach.ReorderAttachment(task.Locator(), from, to)
	})
}

// SubviewCoordinator backs the task subview panels (comments and checklist).
// Every method resolves to the shared transaction messages; the
// panels adapt them back to their own message vocabulary with their own
// adapter so their existing toasts and refresh behavior survive the
// move onto the application layer.
type SubviewCoordinator struct {
	sub SubviewMutator
}

// AddComment appends a comment to a task.
func (c SubviewCoordinator) AddComment(task taskref.TaskIdentity, text string) tea.Cmd {
	return runTask("Add Comment", func() (app.TaskResult, error) {
		res, err := c.sub.AddComment(task.Locator(), text)
		return res.Task, err
	})
}

// EditComment replaces a comment's text, preserving its timestamp.
func (c SubviewCoordinator) EditComment(task taskref.TaskIdentity, index int, text string) tea.Cmd {
	return runTask("Edit Comment", func() (app.TaskResult, error) {
		res, err := c.sub.EditComment(task.Locator(), index, text)
		return res.Task, err
	})
}

// RemoveComment removes the comment at index.
func (c SubviewCoordinator) RemoveComment(task taskref.TaskIdentity, index int) tea.Cmd {
	return runTask("Remove Comment", func() (app.TaskResult, error) {
		res, err := c.sub.RemoveComment(task.Locator(), index)
		return res.Task, err
	})
}

// AddChecklistItem appends a checklist item to a task.
func (c SubviewCoordinator) AddChecklistItem(task taskref.TaskIdentity, text string) tea.Cmd {
	return runTask("Add Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.AddChecklistItem(task.Locator(), text)
		return res.Task, err
	})
}

// EditChecklistItem replaces a checklist item's text, preserving done state.
func (c SubviewCoordinator) EditChecklistItem(task taskref.TaskIdentity, index int, text string) tea.Cmd {
	return runTask("Edit Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.EditChecklistItem(task.Locator(), index, text)
		return res.Task, err
	})
}

// RemoveChecklistItem removes the checklist item at index.
func (c SubviewCoordinator) RemoveChecklistItem(task taskref.TaskIdentity, index int) tea.Cmd {
	return runTask("Remove Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.RemoveChecklistItem(task.Locator(), index)
		return res.Task, err
	})
}

// ToggleChecklistItem flips a checklist item's done state.
func (c SubviewCoordinator) ToggleChecklistItem(task taskref.TaskIdentity, index int) tea.Cmd {
	return runTask("Toggle Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.ToggleChecklistItem(task.Locator(), index)
		return res.Task, err
	})
}

// ReorderChecklistItem swaps the checklist items at from and to.
func (c SubviewCoordinator) ReorderChecklistItem(task taskref.TaskIdentity, from, to int) tea.Cmd {
	return runTask("Reorder Checklist", func() (app.TaskResult, error) {
		res, err := c.sub.ReorderChecklistItem(task.Locator(), from, to)
		return res.Task, err
	})
}

// MoveChecklistItem splices the checklist item at from out and reinserts it at
// to, so an edge move is one transaction rather than one per adjacent swap.
func (c SubviewCoordinator) MoveChecklistItem(task taskref.TaskIdentity, from, to int) tea.Cmd {
	return runTask("Move Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.MoveChecklistItem(task.Locator(), from, to)
		return res.Task, err
	})
}

// PromoteChecklistItem converts the checklist item at index into a child task of
// its owner as one transaction. checklist is the complete ordered list the
// caller read when the gesture was made: it is frozen here rather than
// re-resolved at commit time, so a checklist someone else edited, reordered, or
// shortened in between refuses the promotion instead of consuming a different
// item.
//
// It does not go through runTask because the committed child is the point. The
// owner's own result drives the ordinary reload, and the minted identity rides
// back on RevealTaskFor, which is the only place a view can learn it: the row
// the gesture was made on no longer exists once the transaction lands.
func (c SubviewCoordinator) PromoteChecklistItem(
	task taskref.TaskIdentity, index int, checklist []models.ChecklistItem,
) tea.Cmd {
	return func() tea.Msg {
		res, err := c.sub.PromoteChecklistItem(app.ChecklistPromotionRequest{
			Owner:     task.Locator(),
			Index:     index,
			Checklist: checklist,
		})
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		return TransactionResultMsg{
			Action: "Make Subtask",
			Result: res.Owner,
			RevealTaskFor: taskref.TaskIdentity{
				ProjectID: res.Child.ProjectID,
				TaskID:    res.Child.TaskID,
			},
		}
	}
}
