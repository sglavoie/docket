package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// =============================================================================
// Navigation Messages
// =============================================================================

// BackMsg requests navigation back to the previous view.
type BackMsg struct{}

// RefreshMsg requests a refresh of the current view's data.
type RefreshMsg struct{}

// QuitMsg requests application termination.
type QuitMsg struct{}

// ErrorMsg reports an error that should be displayed to the user.
type ErrorMsg struct{ Err error }

// StatusClearMsg clears the status message after a timeout.
type StatusClearMsg struct{}

// StatusMsg sets a temporary status message (used by command palette handlers).
type StatusMsg string

// =============================================================================
// Project CRUD Messages
// =============================================================================

// SelectProjectMsg switches the current project view.
type SelectProjectMsg struct{ Project *models.Project }

// CreateProjectMsg opens the project creator view.
type CreateProjectMsg struct{}

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

// ProjectDeletedMsg signals a project was permanently deleted.
type ProjectDeletedMsg struct{ Project *models.Project }

// ProjectPinnedToggledMsg signals a project's pinned state changed.
type ProjectPinnedToggledMsg struct{ Project *models.Project }

// SetCurrentProjectMsg sets the default/current project.
type SetCurrentProjectMsg struct{ Project *models.Project }

// =============================================================================
// Task CRUD Messages
// =============================================================================

// CreateTaskMsg opens the task creator, optionally with a parent task.
type CreateTaskMsg struct{ ParentTask *models.Task }

// EditTaskMsg opens the task editor for the given task.
type EditTaskMsg struct{ Task *models.Task }

// TaskCreatedMsg signals a new task was created.
type TaskCreatedMsg struct{ Task *models.Task }

// TaskUpdatedMsg signals a task was modified.
type TaskUpdatedMsg struct{ Task *models.Task }

// TaskDeletedMsg signals a task was deleted.
type TaskDeletedMsg struct{ Task *models.Task }

// TaskStatusChangedMsg signals a task's status changed (complete, start, etc).
type TaskStatusChangedMsg struct{ Task *models.Task }

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
// Notes Messages (Show/Close Pattern)
// =============================================================================

// ShowNotesMsg opens the notes view.
type ShowNotesMsg struct {
	Project *models.Project
}

// CloseNotesMsg closes the notes view.
type CloseNotesMsg struct{}

// NotesRefreshMsg refreshes the notes list.
type NotesRefreshMsg struct{}

// ShowNoteCreatorMsg opens a quick note creation prompt.
type ShowNoteCreatorMsg struct {
	Project *models.Project
}

// ShowNoteTaskLinkerMsg opens the task picker to link a note to a task.
type ShowNoteTaskLinkerMsg struct {
	Project *models.Project
	Note    *models.Note
}

// BulkNoteTagMsg opens the tag selector for multiple notes.
type BulkNoteTagMsg struct {
	Notes   []*models.Note
	Project *models.Project
}

// BulkNoteTagDoneMsg returns the result from bulk note tagging.
type BulkNoteTagDoneMsg struct {
	Notes        []*models.Note
	Project      *models.Project
	SelectedTags []string
	Canceled     bool
}

// BulkNoteLinkMsg opens the task linker for multiple notes.
type BulkNoteLinkMsg struct {
	Notes   []*models.Note
	Project *models.Project
}

// BulkNoteLinkDoneMsg returns the result from bulk note linking.
type BulkNoteLinkDoneMsg struct {
	Notes    []*models.Note
	Project  *models.Project
	TaskID   string
	Canceled bool
}

// CloseNoteTaskLinkerMsg closes the note task linker.
type CloseNoteTaskLinkerMsg struct{}

// NoteTaskLinkedMsg signals a note was linked to a task.
type NoteTaskLinkedMsg struct {
	TaskID string // Last toggled task ID, used for bulk linking
}

// =============================================================================
// Settings Messages
// =============================================================================

// SettingsMsg opens the settings view.
type SettingsMsg struct{}

// SettingsSavedMsg signals settings were saved.
type SettingsSavedMsg struct{}

// =============================================================================
// Due Date Messages
// =============================================================================

// SetDueDateMsg opens the date input view for a task.
type SetDueDateMsg struct {
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
type CloseDependencyViewerMsg struct{}

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
type CloseUndoHistoryMsg struct{}

// UndoPerformedMsg signals an undo operation completed.
type UndoPerformedMsg struct {
	Action string
}

// UndoCompletedMsg signals tasks were restored from undo.
type UndoCompletedMsg struct {
	TasksBefore int
	TasksAfter  int
}

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
	ValidParents []*models.Task
	CurrentID    *string  // Pre-select current parent
	AllowNone    bool     // Whether "None" is valid option
	Source       ViewType // Which view opened it
}

// ParentSelectedMsg returns the selected parent from parent selector.
type ParentSelectedMsg struct {
	ParentID *string  // nil means "None"
	Source   ViewType // Which view to send result to
}

// CloseParentSelectorMsg closes the parent selector without selection.
type CloseParentSelectorMsg struct{}

// =============================================================================
// Priority View Messages (Show/Close Pattern)
// =============================================================================

// ShowPriorityViewMsg opens the priority overview.
type ShowPriorityViewMsg struct {
	Project *models.Project
}

// ClosePriorityViewMsg closes the priority overview.
type ClosePriorityViewMsg struct{}

// PriorityViewSelectMsg navigates to a task from priority view in the project tree.
type PriorityViewSelectMsg struct {
	Task *models.Task
}

// =============================================================================
// Quick Capture Messages (Show/Close Pattern)
// =============================================================================

// ShowQuickCaptureMsg opens the quick capture input.
type ShowQuickCaptureMsg struct {
	Project     *models.Project
	AllProjects []*models.Project // All non-archived projects for cross-project capture
}

// CloseQuickCaptureMsg closes quick capture.
type CloseQuickCaptureMsg struct{}

// =============================================================================
// Bulk Capture Messages (Show/Close Pattern)
// =============================================================================

// ShowBulkCaptureMsg opens the bulk capture input.
type ShowBulkCaptureMsg struct {
	Project *models.Project
}

// CloseBulkCaptureMsg closes bulk capture.
type CloseBulkCaptureMsg struct{}

// =============================================================================
// Task Comments Messages (Show/Close Pattern)
// =============================================================================

// ShowTaskCommentsMsg opens the comments view for a task.
type ShowTaskCommentsMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskCommentsMsg closes the comments view.
type CloseTaskCommentsMsg struct{}

// CommentAddedMsg signals a comment was added.
type CommentAddedMsg struct{ Task *models.Task }

// CommentDeletedMsg signals a comment was deleted.
type CommentDeletedMsg struct{ Task *models.Task }

// CommentEditedMsg signals a comment was edited.
type CommentEditedMsg struct{ Task *models.Task }

// =============================================================================
// Task Activity Messages (Show/Close Pattern)
// =============================================================================

// OpenTaskActivityMsg opens the activity log view for a task.
type OpenTaskActivityMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskActivityMsg closes the activity log view.
type CloseTaskActivityMsg struct{}

// =============================================================================
// Task Attachments Messages (Show/Close Pattern)
// =============================================================================

// ShowTaskAttachmentsMsg opens the attachments view for a task.
type ShowTaskAttachmentsMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CloseTaskAttachmentsMsg closes the attachments view.
type CloseTaskAttachmentsMsg struct{}

// AttachmentAddedMsg signals an attachment was added.
type AttachmentAddedMsg struct{ Task *models.Task }

// AttachmentDeletedMsg signals an attachment was deleted.
type AttachmentDeletedMsg struct{ Task *models.Task }

// =============================================================================
// Project Cycling Messages
// =============================================================================

// CycleProjectMsg cycles to next/previous project.
type CycleProjectMsg struct {
	Forward bool // true = next, false = previous
}

// =============================================================================
// Dashboard Messages
// =============================================================================

// ToggleRecentTasksMsg toggles recent tasks visibility in dashboard.
type ToggleRecentTasksMsg struct{}

// =============================================================================
// Trash Messages (Show/Close Pattern)
// =============================================================================

// ShowTrashedProjectsMsg opens the trashed projects picker.
type ShowTrashedProjectsMsg struct{}

// CloseTrashedProjectsMsg closes the trashed projects picker.
type CloseTrashedProjectsMsg struct{}

// RestoreFromTrashMsg requests restoration of a project from trash.
type RestoreFromTrashMsg struct{ Slug string }

// PurgeFromTrashMsg requests permanent deletion of a project from trash.
type PurgeFromTrashMsg struct{ Slug string }

// =============================================================================
// Misc Messages
// =============================================================================

// MoveBlockedMsg signals a task move was blocked (e.g., completion boundary).
type MoveBlockedMsg struct{ Reason string }

// OpenDataFolderMsg opens the data folder in the configured editor.
// If Project is nil, opens root data directory. Otherwise opens project directory.
type OpenDataFolderMsg struct {
	Project *models.Project
}

// =============================================================================
// Async Save Messages
// =============================================================================

// SaveStartMsg signals that an async save operation is starting.
type SaveStartMsg struct {
	Message string
}

// SaveCompleteMsg signals that an async save completed successfully.
type SaveCompleteMsg struct {
	SuccessMessage string
}

// SaveErrorMsg signals that an async save failed.
type SaveErrorMsg struct {
	Err error
}

// =============================================================================
// Helper Functions
// =============================================================================

// statusTimeout returns a command that clears the status message after the given duration.
func statusTimeout(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return StatusClearMsg{}
	})
}

// =============================================================================
// Cross-Project Move Messages (Show/Close/Done Pattern)
// =============================================================================

// ShowProjectSelectorForMoveMsg opens the project selector for moving a task.
type ShowProjectSelectorForMoveMsg struct {
	Task          *models.Task
	SourceProject *models.Project
}

// CloseProjectSelectorMsg closes the project selector without selection.
type CloseProjectSelectorMsg struct{}

// MoveTaskToProjectMsg requests moving a task to another project.
type MoveTaskToProjectMsg struct {
	Task          *models.Task
	SourceProject *models.Project
	DestProjectID string
}

// TaskMovedToProjectMsg signals a task was moved to another project.
type TaskMovedToProjectMsg struct {
	Task          *models.Task
	SourceProject *models.Project
	DestProject   *models.Project
	MovedCount    int // Number of tasks moved (including descendants)
	ClearedDeps   int // Number of dependencies that were cleared
	OldShortID    int // Original short ID before move
}

// =============================================================================
// Daily Review Messages (Show/Close Pattern)
// =============================================================================

// ShowDailyReviewMsg opens the daily review view.
type ShowDailyReviewMsg struct{}

// CloseDailyReviewMsg closes the daily review view.
type CloseDailyReviewMsg struct{}

// DailyReviewRefreshMsg requests a refresh of the daily review data.
type DailyReviewRefreshMsg struct{}

// DailyReviewSelectMsg navigates to a task from daily review.
type DailyReviewSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// =============================================================================
// Focus View Messages (Show/Close Pattern)
// =============================================================================

// ShowFocusViewMsg opens the focus view.
type ShowFocusViewMsg struct{}

// CloseFocusViewMsg closes the focus view.
type CloseFocusViewMsg struct{}

// FocusViewRefreshMsg requests a refresh of the focus view data.
type FocusViewRefreshMsg struct{}

// FocusViewSelectMsg navigates to a task from focus view.
type FocusViewSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// =============================================================================
// Recent Activity Messages (Show/Close Pattern)
// =============================================================================

// ShowRecentActivityMsg opens the recent activity view.
type ShowRecentActivityMsg struct{}

// CloseRecentActivityMsg closes the recent activity view.
type CloseRecentActivityMsg struct{}

// RecentActivitySelectMsg navigates to a task from recent activity.
type RecentActivitySelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// =============================================================================
// Project Focus View Messages (Show/Close Pattern)
// =============================================================================

// ShowProjectFocusViewMsg opens the project-scoped focus view.
type ShowProjectFocusViewMsg struct {
	Project *models.Project
}

// CloseProjectFocusViewMsg closes the project focus view.
type CloseProjectFocusViewMsg struct{}

// =============================================================================
// Archive Messages
// =============================================================================

// ArchiveCompletedTasksMsg requests archiving all completed tasks in the current project.
type ArchiveCompletedTasksMsg struct{}

// TasksArchivedMsg signals that completed tasks were archived.
type TasksArchivedMsg struct {
	Count int // Number of tasks archived
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
	SourceTask *models.Task
	NewTask    *models.Task
	Project    *models.Project
	LinkType   LinkType
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
type CloseLinkedTaskSelectorMsg struct{}

// LinkedTaskSelectedMsg signals a linked task was selected for navigation.
type LinkedTaskSelectedMsg struct {
	Task    *models.Task
	Project *models.Project
	TaskRef string // Optional: qualified ref for cross-project links (projectID:taskID)
}

// =============================================================================
// Kanban View Messages (Show/Close Pattern)
// =============================================================================

// ShowKanbanMsg opens the kanban view.
type ShowKanbanMsg struct {
	Project *models.Project
}

// CloseKanbanMsg closes the kanban view.
type CloseKanbanMsg struct{}

// =============================================================================
// Cross-Project Linking Messages (Show/Close Pattern)
// =============================================================================

// ShowCrossProjectLinkerMsg opens the cross-project linker for a task.
type ShowCrossProjectLinkerMsg struct {
	SourceTask    *models.Task
	SourceProject *models.Project
}

// CloseCrossProjectLinkerMsg closes the cross-project linker.
type CloseCrossProjectLinkerMsg struct{}

// CrossProjectLinkCreatedMsg signals a cross-project link was created.
type CrossProjectLinkCreatedMsg struct {
	SourceTask    *models.Task
	SourceProject *models.Project
	TargetTask    *models.Task
	TargetProject *models.Project
	LinkType      LinkType
}

// LinkRemovedMsg signals a task link was removed from the sidebar.
type LinkRemovedMsg struct {
	SourceTask *models.Task
	Section    int // 0=blocked by, 1=blocking, 2=related
}
