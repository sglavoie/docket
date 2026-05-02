package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// RecurrenceEditorModel handles editing a custom recurrence pattern.
type RecurrenceEditorModel struct {
	// Pattern components
	interval   int          // 1-99
	unitCursor int          // 0=day, 1=week, 2=month, 3=year
	weekdays   map[int]bool // 0=Sun, 1=Mon, ..., 6=Sat (only for weekly)
	units      []models.RecurrenceUnit
	mode       models.RecurMode // from-due (default) or from-completion

	// For preview calculation
	dueDate *time.Time
	streak  int // Current streak from existing pattern (display only)

	// Focus: 0=interval, 1=unit, 2=weekdays (weekday cursor when focused)
	focusSection  int
	weekdayCursor int // 0-6 when in weekdays section

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
