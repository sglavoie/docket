package tui

import (
	"github.com/sglavoie/tt/internal/models"
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

// projectEditorParkRequestMsg hands a resolved project-editor save to the shell
// because its tag edit newly parks the project. Nothing is written until the
// shared park confirmation is answered.
type projectEditorParkRequestMsg struct{ save *projectEditorSave }

// ProjectArchivedMsg requests the checked commit of a confirmed archive.
type ProjectArchivedMsg struct{ Project *models.Project }

// ProjectRestoredMsg requests the checked commit of a confirmed unarchive.
type ProjectRestoredMsg struct{ Project *models.Project }

// ProjectArchiveRequestMsg requests a confirmed archive/unarchive action.
type ProjectArchiveRequestMsg struct {
	Project *models.Project
	Archive bool
}

const projectArchiveConfirmAction = "project-archive"

type projectArchiveConfirmedMsg struct{}

// ProjectDeleteRequestMsg requests a confirmed project delete. Deleting removes
// the project's whole data directory alongside its catalog and planning
// entries, so it always passes through the shell's own confirmation rather than
// the executor's generic destructive prompt.
type ProjectDeleteRequestMsg struct{ Project *models.Project }

// ProjectDeletedMsg requests the checked commit of a confirmed delete.
type ProjectDeletedMsg struct{ Project *models.Project }

const projectDeleteConfirmAction = "project-delete"

type projectDeleteConfirmedMsg struct{}

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

const projectParkConfirmAction = "project-park"

type projectParkConfirmedMsg struct{}
