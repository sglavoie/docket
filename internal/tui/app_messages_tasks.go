package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// =============================================================================
// Task CRUD Messages
// =============================================================================

// CreateTaskMsg opens the task creator, optionally with a parent task.
type CreateTaskMsg struct {
	ParentTask *models.Task
	Project    *models.Project // Optional: when set, creates in this project instead of currentProject
}

// TaskCreatedMsg signals a new task was created.
//
// Result, when non-nil, is the committed application-layer outcome from a capture
// coordinator. handleTaskCreated reloads the committed project by
// Result.ProjectID and re-resolves the task by Result.TaskID before running the
// shared navigation/status logic, so the legacy Task/Project fields (still set by
// the not-yet-migrated task form) and the coordinator path share one handler.
type TaskCreatedMsg struct {
	Task            *models.Task
	Project         *models.Project // target project (for cross-project + open-after)
	Result          *app.TaskResult // committed capture-coordinator outcome (nil on the legacy path)
	CreatedCount    int             // bulk composer count; zero means infer one from Task/Result
	PlanningChanged bool            // commit added Today/Next membership; refresh the runtime planning snapshot
	OpenTaskID      string          // when set, navigate/open this task instead of Result.TaskID (task-form open-after-subtask)
	OpenContent     bool            // open description editor after creation
}

// =============================================================================
// Parent Selector Messages (Show/Close/Done Pattern)
// =============================================================================

// OpenParentSelectorMsg opens the parent task selector.
type OpenParentSelectorMsg struct {
	ValidParents []*models.Task
	CurrentID    *string                     // Pre-select current parent
	AllowNone    bool                        // Whether "None" is valid option
	Source       presentation.PresentationID // Which view opened it
	ReparentTask taskref.TaskIdentity        // When valid, reparent this qualified task instead of updating a form field
}

// ParentSelectedMsg returns the selected parent from parent selector.
type ParentSelectedMsg struct {
	ParentID     *string                     // nil means "None"
	Source       presentation.PresentationID // Which view to send result to
	ReparentTask taskref.TaskIdentity        // Forwarded from OpenParentSelectorMsg for delayed reparent operations
}

// CloseParentSelectorMsg closes the parent selector without selection.
type CloseParentSelectorMsg struct{ PopViewMsg }

// ShowTaskComposerMsg opens the canonical typed composer preset directly.
type ShowTaskComposerMsg struct {
	Preset TaskComposerPreset
}
