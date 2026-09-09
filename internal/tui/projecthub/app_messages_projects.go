package projecthub

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// =============================================================================
// Project CRUD Messages
// =============================================================================

// CreateProjectMsg opens the project creator view.
type CreateProjectMsg struct{}

// EditProjectMsg opens the project editor for the given project.
type EditProjectMsg struct{ Project *models.Project }

// ProjectCreatedMsg signals a new project was created.
type ProjectCreatedMsg struct {
	Project       *models.Project
	TransactionID string
}

// ProjectUpdatedMsg signals a project's metadata was updated. Parked reports an
// edit that newly parked the project, which the shell anchors and explains the
// same way the project-tag picker path does.
type ProjectUpdatedMsg struct {
	Project       *models.Project
	Parked        bool
	TransactionID string
}

// ProjectEditorParkRequestMsg hands a resolved project-editor save to the shell
// because its tag edit newly parks the project. Nothing is written until the
// shared park confirmation is answered.
type ProjectEditorParkRequestMsg struct{ Save *ProjectEditorSave }

// ProjectArchivedMsg requests the checked commit of a confirmed archive.
type ProjectArchivedMsg struct{ Project *models.Project }

// ProjectRestoredMsg requests the checked commit of a confirmed unarchive.
type ProjectRestoredMsg struct{ Project *models.Project }

// ProjectArchiveRequestMsg requests a confirmed archive/unarchive action.
type ProjectArchiveRequestMsg struct {
	Project *models.Project
	Archive bool
}

const ProjectArchiveConfirmAction = "project-archive"

type ProjectArchiveConfirmedMsg struct{}

// ProjectDeleteRequestMsg requests a confirmed project delete. Deleting removes
// the project's whole data directory alongside its catalog and planning
// entries, so it always passes through the shell's own confirmation rather than
// the executor's generic destructive prompt.
type ProjectDeleteRequestMsg struct{ Project *models.Project }

// ProjectDeletedMsg requests the checked commit of a confirmed delete.
type ProjectDeletedMsg struct{ Project *models.Project }

const ProjectDeleteConfirmAction = "project-delete"

type ProjectDeleteConfirmedMsg struct{}

// ProjectPinRequestMsg requests a durable project pin-state change.
type ProjectPinRequestMsg struct {
	Project *models.Project
	Pinned  bool
}

// SetProjectTagsMsg replaces a project's own tag set wholesale.
type SetProjectTagsMsg struct {
	Project *models.Project
	Tags    []string
}

// AddProjectCommentMsg appends one comment to the project it names. The project
// is the one the operation froze when the prompt opened and revalidated when it
// was submitted, so the shell commits against that owner and never against
// whatever the cursor happens to rest on by the time this message is handled.
// Request is carried so a refused commit can reopen the buffer on the same
// frozen owner; it is the request the prompt was opened from, not a new one
// built from wherever the cursor has since moved.
type AddProjectCommentMsg struct {
	Request operation.OperationExecutionRequest
	Project *models.Project
	Text    string
}

// ProjectCommentRejectedMsg carries a refused project comment back to the buffer
// it was typed in. It exists because the two ways a quick comment can fail —
// the owner became unavailable while the prompt was open, and the commit itself
// was refused — both happen after the popup closed, and a paragraph someone just
// typed is exactly the thing that must not be thrown away by either.
//
// Unlike the task comment's twin it never degrades to a toast: the request
// carries the frozen project identity, so a retry has somewhere to go even when
// the project is gone, and the text survives long enough for the user to copy it
// out.
type ProjectCommentRejectedMsg struct {
	Request operation.OperationExecutionRequest
	Text    string
	Err     error
}

// ProjectBulkTagAction names the tag edit one bulk request applies to every
// project it names.
type ProjectBulkTagAction string

const (
	ProjectBulkTagAdd    ProjectBulkTagAction = "add"
	ProjectBulkTagRemove ProjectBulkTagAction = "remove"
	ProjectBulkTagRename ProjectBulkTagAction = "rename"
	ProjectBulkTagPark   ProjectBulkTagAction = "park"
	ProjectBulkTagUnpark ProjectBulkTagAction = "unpark"
)

// ProjectBulkTagsMsg edits one tag across a project multi-selection. Tag is
// empty for the park and unpark actions, which name the reserved tag
// themselves. Replacement is the new name a rename writes, or the destination a
// removal falls back to for a project the removal would otherwise leave with no
// tags at all.
type ProjectBulkTagsMsg struct {
	ProjectIDs  []string
	Action      ProjectBulkTagAction
	Tag         string
	Replacement string
}

const ProjectParkConfirmAction = "project-park"

type ProjectParkConfirmedMsg struct{}
