package coordinator

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The messages below are the coordinator's outward vocabulary beyond the two
// generic transaction results: each one is constructed by a coordinator method
// in this package and by nothing else, so it lives beside its only constructor
// rather than in the shell that happens to handle it.

// TaskCreatedMsg signals a new task was created.
//
// Result, when non-nil, is the committed application-layer outcome from a capture
// coordinator. The shell's handler reloads the committed project by
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

// TaskClipboardProjectedMsg carries application-owned clipboard text back to
// the TUI adapter, which performs the platform clipboard write. Hidden is how
// many of the projected tasks had no row in the projection the selection was
// frozen against; the application layer leaves it at zero because a hidden
// member is a fact about the view, not about the workspace the text came from.
type TaskClipboardProjectedMsg struct {
	Projection app.TaskClipboardProjection
	Hidden     int
}

// TaskLineEditedMsg returns one successful atomic inline edit to the shell.
// Source and Relocation preserve the origin until the committed workspace has
// been reloaded and the result's final qualified identity can be followed.
type TaskLineEditedMsg struct {
	Result     app.TaskLineEditResult
	Source     taskref.TaskIdentity
	Relocation TransactionRelocation
}

// TaskLineEditFailedMsg keeps the row editor open with its original draft when
// application validation or persistence rejects an atomic edit.
type TaskLineEditFailedMsg struct {
	Source taskref.TaskIdentity
	Err    error
}
