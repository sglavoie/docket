package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// RecurrenceEditorModel handles editing a custom recurrence pattern.
type RecurrenceEditorModel struct {
	// Pattern components
	interval   int // 1-99
	unitCursor int // 0=day, 1=week, 2=month
	units      []models.RecurrenceUnit

	// For preview calculation
	dueDate *time.Time

	// Focus: 0=interval, 1=unit.
	focusSection int

	// intervalEditing tracks if user has started typing digits (first digit replaces, subsequent append)
	intervalEditing bool

	width  int
	height int
}

// ShowRecurrenceEditorMsg triggers showing the recurrence editor.
type ShowRecurrenceEditorMsg struct {
	CurrentPattern *models.RecurrencePattern
	DueDate        *time.Time
}

// RecurrenceEditorDoneMsg signals the recurrence editor has finished.
type RecurrenceEditorDoneMsg struct {
	Pattern  *models.RecurrencePattern
	Canceled bool
}
