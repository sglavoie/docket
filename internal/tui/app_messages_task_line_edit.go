package tui

import (
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
