package projectops

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/projecthub"
)

// Controller is the shell's project cluster: every project act the shell
// itself holds open while it waits for an answer. The archive lifecycle
// (archive and unarchive alike), the delete cascade, the one park prompt the
// tag picker, the bulk tag actions and the project editor all raise, and the
// tag the vocabulary sheet is currently editing.
//
// It owns the waiting, not the writing. Arming a prompt, answering its keys,
// and handing the pending edit back are the whole of this type; every commit
// stays an AppModel method, because the runtime domains, the application
// service and the recovery feedback live there. Nothing here reaches storage,
// so the boundary can add no catalog write of its own.
//
// The three axes stay three axes. No field here names a task lifecycle state:
// a task's workflow state is never read or written by a project act. Archived
// lifecycle and the tag set are separate pendings behind separate dialogs, so
// neither can be reached through the other — archiving preserves tags because
// commitProjectArchive never touches this cluster's tag pending, and parking
// preserves Today/Next references because commitProjectTags commits the catalog
// alone. `parked` remains the one behavioral project tag and its prompt is the
// one project-tag edit that confirms: every other tag edit commits without
// arming anything here.
type Controller struct {
	// pendingArchive is the project whose archive lifecycle change awaits
	// confirmation, and archiving is which direction it was requested in. They
	// are two fields rather than one signed value because unarchiving a project
	// that is not archived is a no-op, not the opposite mutation.
	pendingArchive *models.Project
	archiving      bool
	archiveConfirm chrome.ConfirmDialog

	// pendingDelete is the loaded project copy the delete prompt counted its
	// cascade from, held so the commit reuses exactly what the sentence named.
	// Deleting is the one project act that removes files, so it keeps its own
	// dialog rather than sharing the archive one.
	pendingDelete *models.Project
	deleteConfirm chrome.ConfirmDialog

	// pendingPark is whichever edit raised the shared park confirmation. Only
	// one can be armed at a time because the prompt is modal, and all three
	// routes release through the same take.
	pendingPark ParkPending
	parkConfirm chrome.ConfirmDialog

	// tagActionSubject is the tag the vocabulary sheet edits. It lives here
	// rather than in the sheet's frozen value because a nested value step
	// overwrites that value on its way back to the executor.
	tagActionSubject string
}

// ParkPending is one held park request. Exactly one of its three routes is
// populated: a replacement tag set from the picker, a whole project-editor save
// whose rename and description ride along with the park, or a resolved bulk tag
// edit. Keeping them distinct is what lets a declined prompt leave every field
// of the project as it was rather than committing the part that was not the
// park.
type ParkPending struct {
	Project    *models.Project
	Tags       []string
	Bulk       *BulkTagEdit
	EditorSave *projecthub.ProjectEditorSave
}

// BulkTagUpdate is one project's replacement tag set.
type BulkTagUpdate struct {
	Project *models.Project
	Updated *models.Project
}

// BulkTagEdit is a resolved bulk request: the projects that actually change,
// plus the ones whose change crosses the parking boundary. Resolving the whole
// edit before anything commits is what lets the prompt name the real impact and
// lets a no-op request say so instead of writing an empty transaction. It is
// declared here rather than beside the commit because the park prompt holds one
// while it waits, and the waiting is this cluster's.
type BulkTagEdit struct {
	Tag         string
	Replacement string
	Action      projecthub.ProjectBulkTagAction
	Updates     []BulkTagUpdate
	Parking     []*models.Project
	Unparking   []*models.Project
}

// ArmArchive records the project and the direction, then opens the prompt. The
// verb is derived here so the archive and unarchive routes cannot drift apart.
func (c *Controller) ArmArchive(project *models.Project, archive bool) {
	c.pendingArchive = project
	c.archiving = archive
	verb := "archive"
	if !archive {
		verb = "unarchive"
	}
	c.archiveConfirm.Show(
		projecthub.ProjectArchiveConfirmAction,
		fmt.Sprintf("Are you sure you want to %s this project?", verb),
	)
}

// TakeArchive hands the pending lifecycle change back and disarms in the same
// step, so a dispatched archive can never be prompted for twice.
func (c *Controller) TakeArchive() (*models.Project, bool) {
	project, archive := c.pendingArchive, c.archiving
	c.ClearArchive()
	return project, archive
}

// ClearArchive discards the pending project and dismisses the prompt. It runs
// on cancellation, on a refused request, and after the confirmed message is
// emitted.
func (c *Controller) ClearArchive() {
	c.pendingArchive = nil
	c.archiving = false
	c.archiveConfirm.Cancel()
}

// ResolveArchiveKey answers one key press against the archive confirmation.
func (c *Controller) ResolveArchiveKey(key string) chrome.ConfirmAnswer {
	answer := chrome.ResolveConfirmKey(&c.archiveConfirm, key)
	if answer == chrome.ConfirmAnswerCanceled {
		c.ClearArchive()
	}
	return answer
}

// ArmDelete holds the loaded project copy and opens the prompt whose cascade
// note was computed from that same copy.
func (c *Controller) ArmDelete(project *models.Project, prompt string, detail *chrome.ConfirmDetail) {
	c.pendingDelete = project
	c.deleteConfirm.ShowWithDetail(projecthub.ProjectDeleteConfirmAction, prompt, detail)
}

// TakeDelete hands the pending project back and disarms.
func (c *Controller) TakeDelete() *models.Project {
	project := c.pendingDelete
	c.ClearDelete()
	return project
}

func (c *Controller) ClearDelete() {
	c.pendingDelete = nil
	c.deleteConfirm.Cancel()
}

// ResolveDeleteKey answers one key press against the delete confirmation.
func (c *Controller) ResolveDeleteKey(key string) chrome.ConfirmAnswer {
	answer := chrome.ResolveConfirmKey(&c.deleteConfirm, key)
	if answer == chrome.ConfirmAnswerCanceled {
		c.ClearDelete()
	}
	return answer
}

// ArmPark holds whichever edit is parking and raises the one park prompt. All
// three routes come through here, so the sentence cannot drift between the
// picker, the bulk actions, and the editor.
func (c *Controller) ArmPark(pending ParkPending, prompt string) {
	c.pendingPark = pending
	c.parkConfirm.Show(projecthub.ProjectParkConfirmAction, prompt)
}

// TakePark hands the held edit back and disarms, releasing whichever of the
// three routes raised the prompt.
func (c *Controller) TakePark() ParkPending {
	pending := c.pendingPark
	c.ClearPark()
	return pending
}

// ClearPark is the single cancel path for the shared park confirmation, so it
// releases every route at once.
func (c *Controller) ClearPark() {
	c.pendingPark = ParkPending{}
	c.parkConfirm.Cancel()
}

// ResolveParkKey answers one key press against the park confirmation.
func (c *Controller) ResolveParkKey(key string) chrome.ConfirmAnswer {
	answer := chrome.ResolveConfirmKey(&c.parkConfirm, key)
	if answer == chrome.ConfirmAnswerCanceled {
		c.ClearPark()
	}
	return answer
}

// ConfirmationArmed reports whether any project confirmation is standing. It is
// the reading without the rendering, so the shell can resolve who owns guidance
// without composing an overlay box it may not draw.
func (c *Controller) ConfirmationArmed() bool {
	return c.archiveConfirm.IsActive() || c.deleteConfirm.IsActive() || c.parkConfirm.IsActive()
}

// ConfirmOverlay renders whichever project confirmation is armed, in the order
// the shell has always checked them. All three are modal, so at most one can
// answer here.
func (c *Controller) ConfirmOverlay(width int) (string, bool) {
	if c.archiveConfirm.IsActive() {
		return c.archiveConfirm.RenderOverlay(width), true
	}
	if c.deleteConfirm.IsActive() {
		return c.deleteConfirm.RenderOverlay(width), true
	}
	if c.parkConfirm.IsActive() {
		return c.parkConfirm.RenderOverlay(width), true
	}
	return "", false
}

// ConfirmKey projects one project-confirmation reading onto the shell's
// (handled, cmd) key contract. The three prompts differ only in which message a
// confirmation emits: an armed prompt consumes every key, and a cancellation
// has already released its pending edit by the time the answer arrives here.
func ConfirmKey(answer chrome.ConfirmAnswer, confirmed tea.Msg) (bool, tea.Cmd) {
	switch answer {
	case chrome.ConfirmAnswerConfirmed:
		return true, func() tea.Msg { return confirmed }
	case chrome.ConfirmAnswerSwallowed, chrome.ConfirmAnswerCanceled:
		return true, nil
	case chrome.ConfirmAnswerUnarmed:
	}
	return false, nil
}

// SetTagActionSubject names the tag the vocabulary sheet is editing. The sheet
// opens through this cluster, so the only caller outside it is a test standing
// in for that open.
func (c *Controller) SetTagActionSubject(tag string) {
	c.tagActionSubject = tag
}

// The observers below are how the shell — and the shell's tests — read what a
// prompt is standing in front of without reaching into the pendings. Arming
// stays this cluster's, so nothing here writes.

// ArchiveArmed reports whether the archive confirmation is open.
func (c *Controller) ArchiveArmed() bool { return c.archiveConfirm.IsActive() }

// PendingArchive is the project the archive prompt is holding and the direction
// it was requested in.
func (c *Controller) PendingArchive() (*models.Project, bool) {
	return c.pendingArchive, c.archiving
}

// DeleteArmed reports whether the delete confirmation is open.
func (c *Controller) DeleteArmed() bool { return c.deleteConfirm.IsActive() }

// ParkArmed reports whether the shared park confirmation is open, and
// ParkPrompt is the sentence it is asking.
func (c *Controller) ParkArmed() bool    { return c.parkConfirm.IsActive() }
func (c *Controller) ParkPrompt() string { return c.parkConfirm.Prompt() }

// PendingPark is the edit the park prompt is holding, left armed.
func (c *Controller) PendingPark() ParkPending { return c.pendingPark }
