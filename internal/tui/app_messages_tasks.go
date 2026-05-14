package tui

import "github.com/sglavoie/tt/internal/models"

// =============================================================================
// Task CRUD Messages
// =============================================================================

// CreateTaskMsg opens the task creator, optionally with a parent task.
type CreateTaskMsg struct {
	ParentTask         *models.Task
	SkipTemplatePicker bool // If true, skip template picker even if templates exist
}

// EditTaskMsg opens the task editor for the given task.
type EditTaskMsg struct {
	Task    *models.Task
	Project *models.Project // Optional: when set, uses this project instead of currentProject
}

// TaskCreatedMsg signals a new task was created.
type TaskCreatedMsg struct {
	Task        *models.Task
	Project     *models.Project // target project (for cross-project + open-after)
	OpenContent bool            // open description editor after creation
	OpenDetail  bool            // open task detail view after creation
}

// TaskUpdatedMsg signals a task was modified.
type TaskUpdatedMsg struct{ Task *models.Task }

// TaskDeletedMsg signals a task was deleted.
type TaskDeletedMsg struct{ Task *models.Task }

// TaskStatusChangedMsg signals a task's status changed (complete, start, etc).
type TaskStatusChangedMsg struct{ Task *models.Task }

// TaskPropertyChangedMsg signals an in-place task property change
// (active, pinned, section, etc.) that should refresh without popping.
// When NeedsSave is true, the handler will trigger an async project save.
// This is used by command palette handlers that mutate tasks in-memory
// without going through the persistFn pipeline.
type TaskPropertyChangedMsg struct {
	Task      *models.Task
	NeedsSave bool
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
	Task    *models.Task
	Project *models.Project
}

// =============================================================================
// Snooze Messages
// =============================================================================

// SetSnoozeDateMsg opens the date input view for snoozing a task.
type SetSnoozeDateMsg struct {
	Task    *models.Task
	Project *models.Project
}

// =============================================================================
// Dependency Messages (Show/Close Pattern)
// =============================================================================

// ManageDependenciesMsg opens the dependency selector for a task.
type ManageDependenciesMsg struct {
	Task    *models.Task
	Project *models.Project
}

// ShowDependencyViewerMsg opens read-only dependency visualization.
type ShowDependencyViewerMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseDependencyViewerMsg closes the dependency viewer.
type CloseDependencyViewerMsg struct{ PopViewMsg }

// DependencyViewerSelectMsg navigates to a task from dependency viewer.
type DependencyViewerSelectMsg struct{ Task *models.Task }

// DependencyViewerViewDepsMsg opens dependency viewer for a different task.
type DependencyViewerViewDepsMsg struct {
	Task    *models.Task
	Project *models.Project
}

// =============================================================================
// Undo Messages (Show/Close Pattern)
// =============================================================================

// ShowUndoHistoryMsg opens the undo history view.
type ShowUndoHistoryMsg struct {
	Project *models.Project
}

// CloseUndoHistoryMsg closes the undo history view.
type CloseUndoHistoryMsg struct{ PopViewMsg }

// UndoPerformedMsg signals an undo operation completed.
type UndoPerformedMsg struct {
	Action string
}

// RedoPerformedMsg signals a redo operation completed.
type RedoPerformedMsg struct {
	Action string
}

// UndoHistoryRefreshMsg tells the undo history viewer to reload its entries.
type UndoHistoryRefreshMsg struct{}

// =============================================================================
// Clipboard Messages
// =============================================================================

// CopyTaskTitleMsg copies a single task's title to clipboard.
type CopyTaskTitleMsg struct{ Task *models.Task }

// CopyTaskDetailsMsg copies a single task's full details to clipboard.
type CopyTaskDetailsMsg struct{ Task *models.Task }

// CopyTaskReferenceMsg copies a task reference (e.g., #123) to clipboard.
type CopyTaskReferenceMsg struct{ Task *models.Task }

// CopyTasksTitleMsg copies multiple tasks' titles to clipboard.
type CopyTasksTitleMsg struct{ Tasks []*models.Task }

// CopyTasksDetailsMsg copies multiple tasks' full details to clipboard.
type CopyTasksDetailsMsg struct{ Tasks []*models.Task }

// CopyTaskTitleAndDescMsg copies a single task's title and description to clipboard.
type CopyTaskTitleAndDescMsg struct{ Task *models.Task }

// CopyTasksTitleAndDescMsg copies multiple tasks' title and description to clipboard.
type CopyTasksTitleAndDescMsg struct{ Tasks []*models.Task }

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
	ValidParents   []*models.Task
	CurrentID      *string  // Pre-select current parent
	AllowNone      bool     // Whether "None" is valid option
	Source         ViewType // Which view opened it
	ReparentTaskID string   // When non-empty, reparent this task instead of updating a form field
}

// ParentSelectedMsg returns the selected parent from parent selector.
type ParentSelectedMsg struct {
	ParentID       *string  // nil means "None"
	Source         ViewType // Which view to send result to
	ReparentTaskID string   // Forwarded from OpenParentSelectorMsg; non-empty means reparent operation
}

// CloseParentSelectorMsg closes the parent selector without selection.
type CloseParentSelectorMsg struct{ PopViewMsg }

// =============================================================================
// Quick Capture Messages (Show/Close Pattern)
// =============================================================================

// ShowQuickCaptureMsg opens the quick capture input.
type ShowQuickCaptureMsg struct {
	Project           *models.Project
	AllProjects       []*models.Project // All non-archived projects for cross-project capture
	FromGlobalCapture bool              // True when opened via ;i (project already selected)
	SuggestedTag      string            // Pre-fill with #tag when single active tag filter
	SuggestedSection  string            // Pre-fill with /section when in a specific section view
}

// CloseQuickCaptureMsg closes quick capture.
type CloseQuickCaptureMsg struct{ PopViewMsg }

// =============================================================================
// Bulk Capture Messages (Show/Close Pattern)
// =============================================================================

// ShowBulkCaptureMsg opens the bulk capture input.
type ShowBulkCaptureMsg struct {
	Project *models.Project
}

// CloseBulkCaptureMsg closes bulk capture.
type CloseBulkCaptureMsg struct{ PopViewMsg }

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

// OpenQuickNoteMsg opens the quick note overlay for the given task.
type OpenQuickNoteMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CommentAddedMsg signals a comment was added.
type CommentAddedMsg struct{ Task *models.Task }

// CommentDeletedMsg signals a comment was deleted.
type CommentDeletedMsg struct{ Task *models.Task }

// CommentEditedMsg signals a comment was edited.
type CommentEditedMsg struct{ Task *models.Task }

// CommentResolvedMsg signals a comment's resolved state was toggled.
type CommentResolvedMsg struct{ Task *models.Task }

// =============================================================================
// Task Decisions Messages (Show/Close Pattern)
// =============================================================================

// ShowTaskDecisionsMsg opens the decisions view for a task.
type ShowTaskDecisionsMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskDecisionsMsg closes the decisions view.
type CloseTaskDecisionsMsg struct{ PopViewMsg }

// DecisionAddedMsg signals a decision was added.
type DecisionAddedMsg struct{ Task *models.Task }

// DecisionDeletedMsg signals a decision was deleted.
type DecisionDeletedMsg struct{ Task *models.Task }

// DecisionEditedMsg signals a decision was edited.
type DecisionEditedMsg struct{ Task *models.Task }

// DecisionResolvedMsg signals a decision's resolved state was toggled.
type DecisionResolvedMsg struct{ Task *models.Task }

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
type ChecklistItemAddedMsg struct{ Task *models.Task }

// ChecklistItemEditedMsg signals a checklist item was edited.
type ChecklistItemEditedMsg struct{ Task *models.Task }

// ChecklistItemDeletedMsg signals a checklist item was deleted.
type ChecklistItemDeletedMsg struct{ Task *models.Task }

// ChecklistItemToggledMsg signals a checklist item was toggled.
type ChecklistItemToggledMsg struct{ Task *models.Task }

// =============================================================================
// Task Activity Messages (Show/Close Pattern)
// =============================================================================

// OpenTaskActivityMsg opens the activity log view for a task.
type OpenTaskActivityMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskActivityMsg closes the activity log view.
type CloseTaskActivityMsg struct{ PopViewMsg }

// =============================================================================
// Task Detail Messages (Show/Close Pattern)
// =============================================================================

// ShowTaskDetailMsg opens the full-screen task detail view.
type ShowTaskDetailMsg struct {
	Task    *models.Task
	Project *models.Project
}

// AlertCenterShowDetailMsg requests task detail view from the alert center.
// Resolved at the app level since the notification center doesn't hold config.
type AlertCenterShowDetailMsg struct {
	TaskID    string
	ProjectID string
}

// CloseTaskDetailMsg closes the task detail view.
type CloseTaskDetailMsg struct{ PopViewMsg }

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
type AttachmentAddedMsg struct{ Task *models.Task }

// AttachmentDeletedMsg signals an attachment was deleted.
type AttachmentDeletedMsg struct{ Task *models.Task }

// AttachmentReorderedMsg signals an attachment was reordered.
type AttachmentReorderedMsg struct{ Task *models.Task }

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
	FromGlobal    bool // true when triggered from the global ;tm workflow (skip popView)
}

// TaskMovedToProjectMsg signals a task was moved to another project.
type TaskMovedToProjectMsg struct {
	Task          *models.Task
	SourceProject *models.Project
	DestProject   *models.Project
	MovedCount    int  // Number of tasks moved (including descendants)
	ClearedDeps   int  // Number of dependencies that were cleared
	OldShortID    int  // Original short ID before move
	FromGlobal    bool // true when triggered from the global ;tm workflow (skip popView)
}

// =============================================================================
// Archive Messages
// =============================================================================

// ArchiveCompletedTasksMsg requests archiving all completed tasks in the current project.
type ArchiveCompletedTasksMsg struct{}

// ArchiveAllCompletedTasksMsg requests archiving completed tasks across all active projects.
type ArchiveAllCompletedTasksMsg struct{}

// TasksArchivedMsg signals that completed tasks were archived.
type TasksArchivedMsg struct {
	Count    int // Number of tasks archived
	Projects int // Number of projects affected (0 means single-project archive)
}

// =============================================================================
// Linked Task Creation Messages
// =============================================================================

// LinkType represents the type of link between tasks.
type LinkType int

const (
	LinkTypeBlocker   LinkType = iota // New task blocks source
	LinkTypeDependent                 // New task depends on source
	LinkTypeSubtask                   // New task is a subtask of source
	LinkTypeFollowUp                  // New task is a follow-up (bidirectional related)
)

// CreateLinkedTaskMsg opens the linked task creator.
type CreateLinkedTaskMsg struct {
	SourceTask *models.Task
	Project    *models.Project
	LinkType   LinkType
}

// LinkedTaskCreatedMsg signals a linked task was created.
type LinkedTaskCreatedMsg struct {
	SourceTask  *models.Task
	NewTask     *models.Task
	Project     *models.Project
	LinkType    LinkType
	OpenContent bool
	OpenDetail  bool
}

// =============================================================================
// Linked Task Navigation Messages (Show/Close Pattern)
// =============================================================================

// ShowLinkedTaskSelectorMsg opens the linked task selector for a task.
type ShowLinkedTaskSelectorMsg struct {
	Task      *models.Task
	Project   *models.Project
	ForUnlink bool // If true, always show selector (don't jump on single task)
}

// CloseLinkedTaskSelectorMsg closes the linked task selector.
type CloseLinkedTaskSelectorMsg struct{ PopViewMsg }

// LinkedTaskSelectedMsg signals a linked task was selected for navigation.
type LinkedTaskSelectedMsg struct {
	Task    *models.Task
	Project *models.Project
	TaskRef string // Optional: qualified ref for cross-project links (projectID:taskID)
}

// LinkRemovedMsg signals a task link was removed from the sidebar.
type LinkRemovedMsg struct {
	SourceTask *models.Task
	Section    int // 0=blocked by, 1=blocking, 2=related
}

// =============================================================================
// Cross-Project Linking Messages (Show/Close Pattern)
// =============================================================================

// ShowCrossProjectLinkerMsg opens the cross-project linker for a task.
type ShowCrossProjectLinkerMsg struct {
	SourceTask    *models.Task
	SourceProject *models.Project
}

// CloseCrossProjectLinkerMsg closes the cross-project linker.
type CloseCrossProjectLinkerMsg struct{ PopViewMsg }

// CrossProjectLinkCreatedMsg signals a cross-project link was created.
type CrossProjectLinkCreatedMsg struct {
	SourceTask    *models.Task
	SourceProject *models.Project
	TargetTask    *models.Task
	TargetProject *models.Project
	LinkType      LinkType
}
