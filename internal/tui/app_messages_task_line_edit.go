package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// =============================================================================
// Outliner Row Editor Messages
// =============================================================================

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
