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

// EditTaskMsg opens the task editor for the given task.
type EditTaskMsg struct {
	Task    *models.Task
	Project *models.Project // Optional: when set, uses this project instead of currentProject
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

// TaskUpdatedMsg signals a task was modified.
//
// Contract: Task points at the post-reload pointer in Project.Tasks at emit
// time; consumers may use it directly without re-resolving via GetTaskByID.
// May be nil only if the task was concurrently deleted (rare); handlers must
// nil-check before dereferencing. Empty TaskUpdatedMsg{} (Task=nil) is used
// in tests to exercise the no-op path.
//
// Result, when non-nil, is the committed application-layer outcome from the
// task-form edit coordinator. handleTaskUpdated reloads the committed project by
// Result.ProjectID and re-resolves the live Task pointer before delegating, so
// the coordinator path and the legacy in-place path share one handler.
type TaskUpdatedMsg struct {
	Result *app.TaskResult
}

// TaskDeletedMsg signals a task was deleted. Project carries the project
// the task was deleted from so the post-delete handler can wire an undo
// toast that targets the correct project's snapshot history.
type TaskDeletedMsg struct {
	Project *models.Project
}

// TaskStatusChangedMsg signals a task's status changed (complete, start, etc).
type TaskStatusChangedMsg struct{ Task *models.Task }

// TaskPropertyChangedMsg signals an in-place task property change
// (active, pinned, section, etc.) that should refresh without popping.
// When NeedsSave is true, the handler will trigger an async project save.
// This is used by adapters that mutate tasks in memory without going through
// the coordinator pipeline.
type TaskPropertyChangedMsg struct {
	NeedsSave   bool
	UndoAction  string
	UndoContext string
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

// GlowViewMsg opens glow viewer for task markdown.
type GlowViewMsg struct{ Task *models.Task }

// =============================================================================
// Due Date Messages
// =============================================================================

// SetDueDateMsg opens the date input view for a task.
type SetDueDateMsg struct {
	Task     *models.Task
	Project  *models.Project
	TaskRefs []TaskProjectRef
}

// =============================================================================
// Snooze Messages
// =============================================================================

// SetSnoozeDateMsg opens the date input view for snoozing a task.
type SetSnoozeDateMsg struct {
	Task     *models.Task
	Project  *models.Project
	TaskRefs []TaskProjectRef
}

// =============================================================================
// Undo Messages
// =============================================================================

// UndoPerformedMsg signals an undo operation completed.
type UndoPerformedMsg struct {
	Action string
}

// RedoPerformedMsg signals a redo operation completed.
type RedoPerformedMsg struct {
	Action string
}

// =============================================================================
// Clipboard Messages
// =============================================================================

// CopyTaskTitleMsg copies a single task's title to clipboard.
type CopyTaskTitleMsg struct{ Task *models.Task }

// CopyTaskDetailsMsg copies a single task's full details to clipboard.
type CopyTaskDetailsMsg struct{ Task *models.Task }

// CopyTasksTitleMsg copies multiple tasks' titles to clipboard.
type CopyTasksTitleMsg struct{ Tasks []*models.Task }

// CopyTasksDetailsMsg copies multiple tasks' full details to clipboard.
type CopyTasksDetailsMsg struct{ Tasks []*models.Task }

// CopyTaskTitleAndDescMsg copies a single task's title and description to clipboard.
type CopyTaskTitleAndDescMsg struct{ Task *models.Task }

// CopyTasksTitleAndDescMsg copies multiple tasks' title and description to clipboard.
type CopyTasksTitleAndDescMsg struct{ Tasks []*models.Task }

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
type CommentAddedMsg struct{}

// CommentDeletedMsg signals a comment was deleted.
type CommentDeletedMsg struct{}

// CommentEditedMsg signals a comment was edited.
type CommentEditedMsg struct{}

// =============================================================================
// Task Checklist Messages (Show/Close Pattern)
// =============================================================================

// ShowTaskChecklistMsg opens the checklist view for a task.
type ShowTaskChecklistMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskChecklistMsg closes the checklist view.
type CloseTaskChecklistMsg struct{ PopViewMsg }

// ChecklistItemAddedMsg signals a checklist item was added.
type ChecklistItemAddedMsg struct{}

// ChecklistItemEditedMsg signals a checklist item was edited.
type ChecklistItemEditedMsg struct{}

// ChecklistItemDeletedMsg signals a checklist item was deleted.
type ChecklistItemDeletedMsg struct{}

// ChecklistItemToggledMsg signals a checklist item was toggled.
type ChecklistItemToggledMsg struct{}

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

// AttachmentAddedMsg signals an attachment was added.
type AttachmentAddedMsg struct{}

// AttachmentDeletedMsg signals an attachment was deleted.
type AttachmentDeletedMsg struct{}

// =============================================================================
// Cross-Project Move Messages (Show/Close/Done Pattern)
// =============================================================================

// ShowProjectSelectorForMoveMsg opens the project selector for moving a task.
type ShowProjectSelectorForMoveMsg struct {
	Task          *models.Task
	SourceProject *models.Project
}

// CloseProjectSelectorMsg closes the project selector without selection.
type CloseProjectSelectorMsg struct{ PopViewMsg }

// MoveTaskToProjectMsg requests moving a task to another project.
type MoveTaskToProjectMsg struct {
	Task          *models.Task
	SourceProject *models.Project
	DestProjectID string
	FromGlobal    bool // true when triggered from a global move workflow (skip popView)
}

// TaskMovedToProjectMsg signals a task was moved to another project.
type TaskMovedToProjectMsg struct {
	Task        *models.Task
	DestProject *models.Project
	MovedCount  int  // Number of tasks moved (including descendants)
	OldShortID  int  // Original short ID before move
	FromGlobal  bool // true when triggered from a global move workflow (skip popView)
}
