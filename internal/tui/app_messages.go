package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
)

// Messages for navigation
type (
	SelectProjectMsg        struct{ Project *models.Project }
	BackMsg                 struct{}
	RefreshMsg              struct{}
	ErrorMsg                struct{ Err error }
	CreateProjectMsg        struct{}
	ProjectCreatedMsg       struct{ Project *models.Project }
	ProjectArchivedMsg      struct{ Project *models.Project }
	ProjectPinnedToggledMsg struct{ Project *models.Project }
	ProjectDeletedMsg       struct{ Project *models.Project }
	ProjectRestoredMsg      struct{ Project *models.Project }
	EditProjectMsg          struct{ Project *models.Project }
	ProjectUpdatedMsg       struct{ Project *models.Project }
	SetCurrentProjectMsg    struct{ Project *models.Project }

	// Task management messages
	CreateTaskMsg        struct{ ParentTask *models.Task }
	CreateNoteMsg        struct{ ParentTask *models.Task }
	EditTaskMsg          struct{ Task *models.Task }
	TaskCreatedMsg       struct{ Task *models.Task }
	TaskUpdatedMsg       struct{ Task *models.Task }
	TaskDeletedMsg       struct{ Task *models.Task }
	TaskStatusChangedMsg struct{ Task *models.Task }

	// External editor messages
	EditTaskContentMsg struct {
		Task    *models.Task
		Project *models.Project
	}
	TaskContentEditedMsg struct {
		Task           *models.Task
		Project        *models.Project
		NewTitle       string
		NewDescription string
		Err            error
	}

	// Glow view message
	GlowViewMsg struct{ Task *models.Task }

	// Scratchpad messages
	ShowScratchpadMsg struct {
		Project *models.Project
	}
	EditScratchpadMsg struct {
		Project *models.Project
	}
	ScratchpadEditedMsg struct {
		Project *models.Project
		Err     error
	}

	// Settings messages
	SettingsMsg      struct{}
	SettingsSavedMsg struct{}

	// Due date messages
	SetDueDateMsg struct {
		Task    *models.Task
		Project *models.Project
	}

	// Dependency management messages
	ManageDependenciesMsg struct {
		Task    *models.Task
		Project *models.Project
	}

	// Dependency viewer messages
	ShowDependencyViewerMsg struct {
		Task    *models.Task
		Project *models.Project
	}
	CloseDependencyViewerMsg    struct{}
	DependencyViewerSelectMsg   struct{ Task *models.Task }
	DependencyViewerViewDepsMsg struct {
		Task    *models.Task
		Project *models.Project
	}

	// Undo messages
	UndoCompletedMsg struct {
		TasksBefore int
		TasksAfter  int
	}
	ShowUndoHistoryMsg struct {
		Project *models.Project
	}
	CloseUndoHistoryMsg struct{}
	UndoPerformedMsg    struct {
		Action string
	}

	// Clipboard messages
	CopyTaskTitleMsg    struct{ Task *models.Task }
	CopyTaskDetailsMsg  struct{ Task *models.Task }
	CopyTasksTitleMsg   struct{ Tasks []*models.Task }
	CopyTasksDetailsMsg struct{ Tasks []*models.Task }
	TaskCopiedMsg       struct {
		Count      int
		FullDetail bool
	}

	// Status message
	StatusClearMsg struct{}

	// Move blocked message (when task can't move due to completion boundary)
	MoveBlockedMsg struct{ Reason string }

	// Parent selector messages
	OpenParentSelectorMsg struct {
		ValidParents []*models.Task
		CurrentID    *string  // Pre-select current parent
		AllowNone    bool     // Whether "None" is valid option
		Source       ViewType // Which view opened it
	}
	ParentSelectedMsg struct {
		ParentID *string  // nil means "None"
		Source   ViewType // Which view to send result to
	}
	CloseParentSelectorMsg struct{}

	// Priority view messages
	ShowPriorityViewMsg struct {
		Project *models.Project
	}
	ClosePriorityViewMsg struct{}

	// Quick capture messages
	ShowQuickCaptureMsg struct {
		Project *models.Project
	}
	CloseQuickCaptureMsg struct{}

	// Bulk capture messages
	ShowBulkCaptureMsg struct {
		Project *models.Project
	}
	CloseBulkCaptureMsg struct{}

	// Task comments messages
	ShowTaskCommentsMsg struct {
		Task    *models.Task
		Project *models.Project
	}
	CloseTaskCommentsMsg struct{}
	CommentAddedMsg      struct{ Task *models.Task }
	CommentDeletedMsg    struct{ Task *models.Task }
	CommentEditedMsg     struct{ Task *models.Task }

	// Project cycling messages
	CycleProjectMsg struct {
		Forward bool // true = next, false = previous
	}

	// Dashboard toggle messages
	ToggleRecentTasksMsg struct{}

	// Application control messages
	QuitMsg struct{} // Request to quit the application
)

// statusTimeout returns a command that clears the status message after the given duration.
func statusTimeout(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return StatusClearMsg{}
	})
}
