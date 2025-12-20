package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
)

// Messages for navigation
type (
	SelectProjectMsg   struct{ Project *models.Project }
	SelectTaskMsg      struct{ Task *models.Task }
	BackMsg            struct{}
	RefreshMsg         struct{}
	ErrorMsg           struct{ Err error }
	CreateProjectMsg   struct{}
	ProjectCreatedMsg  struct{ Project *models.Project }
	ProjectArchivedMsg struct{ Project *models.Project }
	ProjectDeletedMsg  struct{ Project *models.Project }
	ProjectRestoredMsg struct{ Project *models.Project }
	EditProjectMsg     struct{ Project *models.Project }
	ProjectUpdatedMsg  struct{ Project *models.Project }

	// Task management messages
	CreateTaskMsg        struct{ ParentTask *models.Task }
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

	// Undo message
	UndoCompletedMsg struct {
		TasksBefore int
		TasksAfter  int
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
)

// statusTimeout returns a command that clears the status message after the given duration.
func statusTimeout(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return StatusClearMsg{}
	})
}
