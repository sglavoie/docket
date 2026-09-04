package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/projecthub"
)

// projectController is the shell's project cluster: every project act the shell
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
type projectController struct {
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
	pendingPark projectParkPending
	parkConfirm chrome.ConfirmDialog

	// tagActionSubject is the tag the vocabulary sheet edits. It lives here
	// rather than in the sheet's frozen value because a nested value step
	// overwrites that value on its way back to the executor.
	tagActionSubject string
}

// projectParkPending is one held park request. Exactly one of its three routes
// is populated: a replacement tag set from the picker, a whole project-editor
// save whose rename and description ride along with the park, or a resolved
// bulk tag edit. Keeping them distinct is what lets a declined prompt leave
// every field of the project as it was rather than committing the part that
// was not the park.
type projectParkPending struct {
	project    *models.Project
	tags       []string
	bulk       *projectBulkTagEdit
	editorSave *projecthub.ProjectEditorSave
}

// armArchive records the project and the direction, then opens the prompt. The
// verb is derived here so the archive and unarchive routes cannot drift apart.
func (c *projectController) armArchive(project *models.Project, archive bool) {
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

// takeArchive hands the pending lifecycle change back and disarms in the same
// step, so a dispatched archive can never be prompted for twice.
func (c *projectController) takeArchive() (*models.Project, bool) {
	project, archive := c.pendingArchive, c.archiving
	c.clearArchive()
	return project, archive
}

// clearArchive discards the pending project and dismisses the prompt. It runs
// on cancellation, on a refused request, and after the confirmed message is
// emitted.
func (c *projectController) clearArchive() {
	c.pendingArchive = nil
	c.archiving = false
	c.archiveConfirm.Cancel()
}

// resolveArchiveKey answers one key press against the archive confirmation.
func (c *projectController) resolveArchiveKey(key string) confirmAnswer {
	answer := resolveConfirmKey(&c.archiveConfirm, key)
	if answer == confirmAnswerCanceled {
		c.clearArchive()
	}
	return answer
}

// armDelete holds the loaded project copy and opens the prompt whose cascade
// note was computed from that same copy.
func (c *projectController) armDelete(project *models.Project, prompt string, detail *chrome.ConfirmDetail) {
	c.pendingDelete = project
	c.deleteConfirm.ShowWithDetail(projecthub.ProjectDeleteConfirmAction, prompt, detail)
}

// takeDelete hands the pending project back and disarms.
func (c *projectController) takeDelete() *models.Project {
	project := c.pendingDelete
	c.clearDelete()
	return project
}

func (c *projectController) clearDelete() {
	c.pendingDelete = nil
	c.deleteConfirm.Cancel()
}

// resolveDeleteKey answers one key press against the delete confirmation.
func (c *projectController) resolveDeleteKey(key string) confirmAnswer {
	answer := resolveConfirmKey(&c.deleteConfirm, key)
	if answer == confirmAnswerCanceled {
		c.clearDelete()
	}
	return answer
}

// armPark holds whichever edit is parking and raises the one park prompt. All
// three routes come through here, so the sentence cannot drift between the
// picker, the bulk actions, and the editor.
func (c *projectController) armPark(pending projectParkPending, prompt string) {
	c.pendingPark = pending
	c.parkConfirm.Show(projecthub.ProjectParkConfirmAction, prompt)
}

// takePark hands the held edit back and disarms, releasing whichever of the
// three routes raised the prompt.
func (c *projectController) takePark() projectParkPending {
	pending := c.pendingPark
	c.clearPark()
	return pending
}

// clearPark is the single cancel path for the shared park confirmation, so it
// releases every route at once.
func (c *projectController) clearPark() {
	c.pendingPark = projectParkPending{}
	c.parkConfirm.Cancel()
}

// resolveParkKey answers one key press against the park confirmation.
func (c *projectController) resolveParkKey(key string) confirmAnswer {
	answer := resolveConfirmKey(&c.parkConfirm, key)
	if answer == confirmAnswerCanceled {
		c.clearPark()
	}
	return answer
}

// confirmOverlay renders whichever project confirmation is armed, in the order
// the shell has always checked them. All three are modal, so at most one can
// answer here.
func (c *projectController) confirmOverlay(width int) (string, bool) {
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

// projectConfirmKey projects one project-confirmation reading onto the shell's
// (handled, cmd) key contract. The three prompts differ only in which message a
// confirmation emits: an armed prompt consumes every key, and a cancellation
// has already released its pending edit by the time the answer arrives here.
func projectConfirmKey(answer confirmAnswer, confirmed tea.Msg) (bool, tea.Cmd) {
	switch answer {
	case confirmAnswerConfirmed:
		return true, func() tea.Msg { return confirmed }
	case confirmAnswerSwallowed, confirmAnswerCanceled:
		return true, nil
	case confirmAnswerUnarmed:
	}
	return false, nil
}
