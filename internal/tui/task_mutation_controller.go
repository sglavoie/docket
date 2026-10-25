package tui

import (
	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

// taskMutationController is the shell's task-mutation cluster: the two
// destructive task edits the shell itself holds — the canonical delete request
// and the outline buffer whose removals need confirming — each with the
// ConfirmDialog standing in front of it.
//
// It owns the waiting, not the writing. Arming a prompt, answering its keys,
// and handing the pending value back are the whole of this type; the
// transaction stays on AppModel, where the application service and the
// coordinators live. That split is what keeps the mutation contract intact: a
// confirmed delete is still one BulkCoordinator.DeleteLocators over targets
// resolved from one workspace snapshot, and a confirmed outline apply is still
// one Outline.Apply over the buffer that was parsed before the prompt opened.
// Nothing here reaches storage, so the boundary can add no SaveProject call of
// its own.
//
// The two pendings are deliberately separate values rather than one union: a
// delete carries resolved task refs and an outline apply carries a parsed
// buffer with the revision it was dumped at, and only one of the two can be
// armed at a time because each prompt is modal.
type taskMutationController struct {
	// pendingDelete holds the canonicalized roots and the originating selection
	// for an in-flight shell-owned task deletion awaiting confirmation.
	pendingDelete taskDeletePending
	deleteConfirm chrome.ConfirmDialog

	// pendingOutline holds one parsed outline buffer while its deletions await
	// confirmation. It is parsed and its removals counted before the prompt
	// opens, so the numbers on screen are the ones the transaction acts on.
	pendingOutline outlineApplyPending
	outlineConfirm chrome.ConfirmDialog
}

// taskDeletePending holds the canonicalized roots and origin for an in-flight
// shell-owned task deletion awaiting confirmation.
type taskDeletePending struct {
	roots    []TaskProjectRef
	selected []TaskProjectRef
}

// outlineApplyPending is one parsed buffer held while its deletions await
// confirmation. It carries the path so a declined apply can still name the file.
type outlineApplyPending struct {
	project      *models.Project
	revision     string
	path         string
	scopeTaskIDs []string
	lines        []app.OutlinePlanLine
	deletions    []*models.Task
}

const (
	taskDeleteConfirmAction    = "task-delete"
	outlineDeleteConfirmAction = "outline-delete"
)

// armDelete records the effective roots and the selection they came from, then
// opens the prompt describing the impact already calculated for them.
func (c *taskMutationController) armDelete(roots, selected []TaskProjectRef, impact taskDeleteImpact) {
	c.pendingDelete = taskDeletePending{roots: roots, selected: selected}
	c.deleteConfirm.ShowWithDetail(
		taskDeleteConfirmAction,
		taskDeletePrompt(roots),
		taskDeleteConfirmDetail(roots, impact),
	)
}

// takeDelete hands the pending deletion to its caller and disarms in the same
// step, so a dispatched transaction can never be prompted for twice.
func (c *taskMutationController) takeDelete() taskDeletePending {
	pending := c.pendingDelete
	c.clearDelete()
	return pending
}

// clearDelete discards the pending roots and dismisses the confirm dialog. It
// runs on cancellation and after the transaction is dispatched so the shell
// never re-prompts for a stale selection.
func (c *taskMutationController) clearDelete() {
	c.pendingDelete = taskDeletePending{}
	c.deleteConfirm.Cancel()
}

// resolveDeleteKey answers one key press against the delete confirmation.
func (c *taskMutationController) resolveDeleteKey(key string) chrome.ConfirmAnswer {
	answer := chrome.ResolveConfirmKey(&c.deleteConfirm, key)
	if answer == chrome.ConfirmAnswerCanceled {
		c.clearDelete()
	}
	return answer
}

// armOutlineApply holds the parsed buffer and opens the prompt naming the tasks
// its lines no longer claim.
func (c *taskMutationController) armOutlineApply(pending outlineApplyPending) {
	c.pendingOutline = pending
	c.outlineConfirm.ShowWithDetail(
		outlineDeleteConfirmAction,
		outlineDeletePrompt(pending.project, pending.deletions),
		outlineDeleteConfirmDetail(pending.deletions),
	)
}

// takeOutlineApply hands the pending buffer back and disarms. Both answers need
// it — confirming applies it, declining names its path — so neither route reads
// the field directly.
func (c *taskMutationController) takeOutlineApply() outlineApplyPending {
	pending := c.pendingOutline
	c.clearOutlineApply()
	return pending
}

func (c *taskMutationController) clearOutlineApply() {
	c.pendingOutline = outlineApplyPending{}
	c.outlineConfirm.Cancel()
}

// resolveOutlineKey answers one key press against the outline confirmation. It
// does not clear on cancellation: declining still owes the user the buffer
// path, so that route takes the pending value instead.
func (c *taskMutationController) resolveOutlineKey(key string) chrome.ConfirmAnswer {
	return chrome.ResolveConfirmKey(&c.outlineConfirm, key)
}

// confirmationArmed is confirmOverlay's reading half, so guidance ownership can
// be resolved without rendering a box the frame may not use.
func (c *taskMutationController) confirmationArmed() bool {
	return c.deleteConfirm.IsActive() || c.outlineConfirm.IsActive()
}

// confirmOverlay renders whichever task-mutation confirmation is armed, delete
// first. Both dialogs are modal, so at most one can answer here.
func (c *taskMutationController) confirmOverlay(width int) (string, bool) {
	if c.deleteConfirm.IsActive() {
		return c.deleteConfirm.RenderOverlay(width), true
	}
	if c.outlineConfirm.IsActive() {
		return c.outlineConfirm.RenderOverlay(width), true
	}
	return "", false
}
