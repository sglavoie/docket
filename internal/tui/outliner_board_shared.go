package tui

import (
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
)

// FocusTask is an alias for focus.FocusTask — the cross-project ordered task the
// daily plan / Next order index are built from. Retained after the standalone
// Planner (Focus Board) model was retired; the Outliner's Today section and
// Next mode build their ordering from these.
type FocusTask = focus.FocusTask

// Planner tab identities retained for daily-plan / Next reconciliation
// (planning_reconcile.go, task_composer.go).
const (
	boardTabToday   = 0
	boardTabNext    = 1
	boardTabBacklog = 2
)

// boardRow is the minimal compatibility shape still used by grouped title undo.
type boardRow struct {
	Task    *models.Task
	Project *models.Project
}
