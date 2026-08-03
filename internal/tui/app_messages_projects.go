package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// =============================================================================
// Project CRUD Messages
// =============================================================================

// SelectProjectMsg switches the current project view.
type SelectProjectMsg struct{ Project *models.Project }

// CreateProjectMsg opens the project creator view. MoveTask/SourceProject are
// optional triage continuation data: after creation the selected Inbox item is
// moved into the new project without a second picker step.
type CreateProjectMsg struct {
	MoveTask      *models.Task
	SourceProject *models.Project
}

// EditProjectMsg opens the project editor for the given project.
type EditProjectMsg struct{ Project *models.Project }

// ProjectCreatedMsg signals a new project was created.
type ProjectCreatedMsg struct{ Project *models.Project }

// ProjectUpdatedMsg signals a project's metadata was updated.
type ProjectUpdatedMsg struct{}

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

// ProjectPinRequestMsg requests a durable project pin-state change.
type ProjectPinRequestMsg struct {
	Project *models.Project
	Pinned  bool
}
