package tui

import (
	"github.com/sglavoie/tt/internal/app"
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
type ProjectUpdatedMsg struct{ Project *models.Project }

// ProjectArchivedMsg signals a project was archived.
type ProjectArchivedMsg struct{ Project *models.Project }

// ProjectRestoredMsg signals an archived project was restored.
type ProjectRestoredMsg struct{ Project *models.Project }

// ProjectArchiveRequestMsg requests a confirmed archive/unarchive action.
type ProjectArchiveRequestMsg struct {
	Project *models.Project
	Archive bool
}

const projectArchiveConfirmAction = "project-archive"

type projectArchiveConfirmedMsg struct{}

// ProjectDeletedMsg signals a project was permanently deleted.
type ProjectDeletedMsg struct{ Project *models.Project }

// ProjectPinnedToggledMsg signals a project's pinned state changed.
type ProjectPinnedToggledMsg struct{ Project *models.Project }

// =============================================================================
// Project Tree Messages
// =============================================================================

// ShowProjectTreeMsg opens the project tree view for a project.
type ShowProjectTreeMsg struct {
	Project *models.Project
}

// =============================================================================
// Project Cycling Messages
// =============================================================================

// CycleProjectMsg cycles to next/previous project.
type CycleProjectMsg struct {
	Forward bool // true = next, false = previous
}

// ShowProjectCyclePickerMsg opens the numbered project cycle picker popup.
type ShowProjectCyclePickerMsg struct{}

// =============================================================================
// Trash Messages (Show/Close Pattern)
// =============================================================================

// ShowTrashedProjectsMsg opens the trashed projects picker.
type ShowTrashedProjectsMsg struct{}

// CloseTrashedProjectsMsg closes the trashed projects picker.
type CloseTrashedProjectsMsg struct{ PopViewMsg }

// RestoreFromTrashMsg requests restoration of a project from trash.
type RestoreFromTrashMsg struct {
	EntryID string
	Slug    string
}

// PurgeFromTrashMsg requests permanent deletion of a project from trash.
type PurgeFromTrashMsg struct {
	EntryID string
	Slug    string
}

// =============================================================================
// Status Input Messages
// =============================================================================

// ShowStatusInputMsg opens the status input overlay for a project.
type ShowStatusInputMsg struct {
	Project *models.Project
}

// StatusUpdatedMsg signals the project status was updated.
type StatusUpdatedMsg struct{}

// =============================================================================
// Sandbox Messages
// =============================================================================

// SandboxResetRequestMsg requests to reset/clear the sandbox project.
type SandboxResetRequestMsg struct{}

// SandboxResetDoneMsg signals the sandbox was reset.
type SandboxResetDoneMsg struct {
	Result app.SandboxResetResult
}

// SandboxResetFailedMsg reports a rejected reset without changing live caches.
type SandboxResetFailedMsg struct{ Err error }
