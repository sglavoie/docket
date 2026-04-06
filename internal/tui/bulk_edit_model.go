package tui

import (
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// Field indices for navigation.
const (
	fieldStatus = iota
	fieldType
	fieldPriority
	fieldTags
	fieldSection
	fieldDueDate
	fieldRecurrence
	fieldParent
	fieldActions
	fieldCount
)

// BulkEditModel handles bulk editing of multiple tasks.
type BulkEditModel struct {
	BaseView
	tasks   []*models.Task
	project *models.Project

	// Navigation
	focusIndex int // Which field is focused (0-5)

	// Status field
	statusCursor  int                // Cursor within status options (0=no change, 1-7=statuses)
	statusValue   *models.TaskStatus // nil = no change
	statusSummary string             // Summary of current values

	// Type field
	typeCursor  int              // Cursor within type options (0=no change, 1-6=types)
	typeValue   *models.TaskType // nil = no change
	typeSummary string           // Summary of current values

	// Priority field
	priorityCursor  int    // Cursor within priority options (0=no change, 1-5=priorities)
	priorityValue   *int   // nil = no change
	prioritySummary string // Summary of current values

	// Tags field
	tagMode         int             // 0=normal, 1=add input, 2=remove selection
	tagAddInput     textinput.Model // Input for adding tags
	addTags         []string        // Tags to add
	removeTags      map[string]bool // Tags to remove (toggled)
	commonTags      []string        // Tags present on all tasks
	allTags         []string        // All unique tags across tasks
	tagRemoveCursor int             // Cursor for remove tag selection

	// Section field
	sectionCursor  int      // Cursor within section options
	sectionValue   *string  // nil = no change
	sectionSummary string   // Summary of current values
	sectionOptions []string // Available sections

	// Due date field
	dueDateCursor    int             // 0=no change, 1=set date, 2=clear
	dueDateSummary   string          // Summary of current values
	clearDueDate     bool            // Whether to clear due dates
	setDueDate       *time.Time      // Selected date to apply (nil = no change)
	dueDateInputMode bool            // True when typing date
	dueDateInput     textinput.Model // Text input for date entry
	dueDateError     error           // Validation error

	// Recurrence field
	recurrenceCursor  int                       // 0=keep, 1-5=presets, 6=clear
	recurrenceValue   *models.RecurrencePattern // nil = no change
	clearRecurrence   bool                      // Whether to clear recurrence
	recurrenceSummary string                    // Summary of current values

	// Parent field
	parentCursor     int            // 0=Keep, 1=Set parent, 2=Move to root
	parentValue      *string        // Selected parent task ID
	clearParent      bool           // Whether to move to root
	parentSummary    string         // Summary of current parents
	parentCandidates []*models.Task // Valid parent candidates
	parentListMode   bool           // True when browsing candidate list
	parentListCursor int            // Cursor within candidate list
	parentListOffset int            // Scroll offset for candidate list

	// Actions field
	actionCursor int // 0=Apply, 1=Cancel

	// Scroll management
	scrollOffset int // Lines scrolled from top
}

// NewBulkEditModel creates a new bulk edit panel for the given tasks.
func NewBulkEditModel(tasks []*models.Task, project *models.Project) *BulkEditModel {
	m := &BulkEditModel{
		tasks:      tasks,
		project:    project,
		focusIndex: fieldStatus,
		removeTags: make(map[string]bool),
	}

	// Initialize tag input.
	m.tagAddInput = textinput.New()
	m.tagAddInput.Placeholder = "Enter tag name..."
	m.tagAddInput.CharLimit = 50
	m.tagAddInput.SetWidth(30)

	// Initialize due date input.
	m.dueDateInput = textinput.New()
	m.dueDateInput.Placeholder = "e.g., tomorrow, +3d, fri, 2024-01-15"
	m.dueDateInput.CharLimit = 30
	m.dueDateInput.SetWidth(40)

	m.analyzeCurrentValues()
	m.buildSectionOptions()
	m.buildParentCandidates()

	return m
}

// HasTextInput returns true when a text input field is active.
func (m *BulkEditModel) HasTextInput() bool {
	return m.tagMode == 1 || m.dueDateInputMode
}

// ShouldCaptureEsc returns true when the view needs to handle esc internally.
func (m *BulkEditModel) ShouldCaptureEsc() bool {
	return m.tagMode == 1 || m.tagMode == 2 || m.dueDateInputMode
}

// Init initializes the model.
func (m *BulkEditModel) Init() tea.Cmd {
	return nil
}

func (m *BulkEditModel) HandledKeys() []string {
	return []string{
		// Normal navigation
		"tab", "down", "j", "shift+tab", "up", "k",
		// Field interaction
		"enter", "space", "l", "right", "h", "left",
		// Tag operations
		"a", "r",
	}
}

func (m *BulkEditModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewBulkEdit)
}

func (m *BulkEditModel) StatusBarLabel() string { return "Bulk Edit" }
