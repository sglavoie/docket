package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
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

// StartTaskLineEditMsg activates the selected Outliner row editor. Task and
// Project are the frozen operation target so shell dispatch cannot retarget the
// edit if the cursor moves before the message is delivered.
type StartTaskLineEditMsg struct {
	Task    *models.Task
	Project *models.Project
}

// TaskLineEditedMsg returns one successful atomic inline edit to the shell.
// Source and relocation preserve the origin until the committed workspace has
// been reloaded and the result's final qualified identity can be followed.
type TaskLineEditedMsg struct {
	Result     app.TaskLineEditResult
	Source     TaskIdentity
	Relocation *outlinerRelocationCapture
}

// TaskLineEditFailedMsg keeps the row editor open with its original draft when
// application validation or persistence rejects an atomic edit.
type TaskLineEditFailedMsg struct {
	Source TaskIdentity
	Err    error
}

// =============================================================================
// External Editor Messages
// =============================================================================

// EditTaskContentMsg launches external editor for task title/description.
type EditTaskContentMsg struct {
	Task    *models.Task
	Project *models.Project
}

// TaskContentEditedMsg returns results from external editor.
type TaskContentEditedMsg struct {
	Task           *models.Task
	Project        *models.Project
	NewTitle       string
	NewDescription string
	Err            error
}

// OpenOutlineEditorMsg dumps one project's task tree to an external editor for
// bulk restructuring. The Outliner emits it; the shell owns the round trip.
// Project names which project to outline: the shell reloads it from an
// application snapshot before dumping, so the cursor's own copy is never what
// reaches the buffer.
type OpenOutlineEditorMsg struct {
	Project *models.Project
}

// OutlineEditedMsg returns one edited outline buffer. Project and Revision are
// the snapshot pair the buffer was dumped from, and the revision travels into
// the apply so a workspace that moved while the editor was open is refused
// instead of merged. ScopeTaskIDs are the tasks the dump actually printed, which
// is what makes absence readable as deletion. Path is kept so a refused buffer
// can be named in the status rather than silently discarded.
type OutlineEditedMsg struct {
	Project      *models.Project
	Revision     string
	ScopeTaskIDs []string
	Path         string
	Buffer       string
	Err          error
}

// =============================================================================
// Clipboard Messages
// =============================================================================

// CopyTaskTitleMsg copies a single task's title to clipboard.
type CopyTaskTitleMsg struct{ Task *models.Task }

// CopyTaskDetailsMsg copies a single task's full details to clipboard.
type CopyTaskDetailsMsg struct{ Task *models.Task }

// CopyTaskTitleAndDescMsg copies a single task's title and description to clipboard.
type CopyTaskTitleAndDescMsg struct{ Task *models.Task }

// TaskClipboardProjectedMsg carries application-owned clipboard text back to
// the TUI adapter, which performs the platform clipboard write.
type TaskClipboardProjectedMsg struct {
	Projection app.TaskClipboardProjection
}

// TaskCopiedMsg confirms task(s) were copied to clipboard.
type TaskCopiedMsg struct {
	Count        int
	FullDetail   bool
	TitleAndDesc bool
}

// =============================================================================
// Parent Selector Messages (Show/Close/Done Pattern)
// =============================================================================

// OpenParentSelectorMsg opens the parent task selector.
type OpenParentSelectorMsg struct {
	ValidParents []*models.Task
	CurrentID    *string        // Pre-select current parent
	AllowNone    bool           // Whether "None" is valid option
	Source       PresentationID // Which view opened it
	ReparentTask TaskIdentity   // When valid, reparent this qualified task instead of updating a form field
}

// ParentSelectedMsg returns the selected parent from parent selector.
type ParentSelectedMsg struct {
	ParentID     *string        // nil means "None"
	Source       PresentationID // Which view to send result to
	ReparentTask TaskIdentity   // Forwarded from OpenParentSelectorMsg for delayed reparent operations
}

// CloseParentSelectorMsg closes the parent selector without selection.
type CloseParentSelectorMsg struct{ PopViewMsg }

// ShowTaskComposerMsg opens the canonical typed composer preset directly.
type ShowTaskComposerMsg struct {
	Preset TaskComposerPreset
}

// =============================================================================
// Task Comments Messages (Show/Close Pattern)
// =============================================================================

// ShowTaskCommentsMsg opens the comments view for a task.
type ShowTaskCommentsMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskCommentsMsg closes the comments view.
type CloseTaskCommentsMsg struct{ PopViewMsg }

// CommentAddedMsg signals a comment was added.
type CommentAddedMsg struct{ TransactionID string }

// CommentDeletedMsg signals a comment was deleted.
type CommentDeletedMsg struct{ TransactionID string }

// CommentEditedMsg signals a comment was edited.
type CommentEditedMsg struct{ TransactionID string }

// =============================================================================
// Task Attachments Messages (Show/Close Pattern)
// =============================================================================

// ShowTaskAttachmentsMsg opens the attachments view for a task.
type ShowTaskAttachmentsMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskAttachmentsMsg closes the attachments view.
type CloseTaskAttachmentsMsg struct{ PopViewMsg }
