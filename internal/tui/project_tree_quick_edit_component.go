package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// QuickEditComponent encapsulates quick edit state and functionality.
type QuickEditComponent struct {
	mode     bool             // True when in quick edit mode
	rename   bool             // True if rename-only mode (no modifier parsing)
	input    textinput.Model  // Text input for editing
	task     *models.Task     // Task being edited
	preview  *quickEditResult // Live preview of parsed changes
	original *quickEditResult // Original task state for comparison
}

// NewQuickEditComponent creates a new quick edit component.
func NewQuickEditComponent() *QuickEditComponent {
	quickEditInput := textinput.New()
	quickEditInput.Placeholder = "Edit task..."
	quickEditInput.CharLimit = 200
	quickEditInput.SetWidth(60)

	return &QuickEditComponent{
		mode:     false,
		rename:   false,
		input:    quickEditInput,
		task:     nil,
		preview:  nil,
		original: nil,
	}
}

// IsActive returns whether quick edit mode is active.
func (q *QuickEditComponent) IsActive() bool {
	return q.mode
}

// IsRenameOnly returns whether quick edit is in rename-only mode.
func (q *QuickEditComponent) IsRenameOnly() bool {
	return q.rename
}

// GetInput returns the text input model.
func (q *QuickEditComponent) GetInput() textinput.Model {
	return q.input
}

// GetTask returns the task being edited.
func (q *QuickEditComponent) GetTask() *models.Task {
	return q.task
}

// GetPreview returns the live preview of parsed changes.
func (q *QuickEditComponent) GetPreview() *quickEditResult {
	return q.preview
}

// GetOriginal returns the original task state.
func (q *QuickEditComponent) GetOriginal() *quickEditResult {
	return q.original
}

// SetPreview updates the preview of parsed changes.
func (q *QuickEditComponent) SetPreview(preview *quickEditResult) {
	q.preview = preview
}

// Enter enters quick edit mode for the given task.
func (q *QuickEditComponent) Enter(task *models.Task, renameOnly bool) tea.Cmd {
	q.mode = true
	q.rename = renameOnly
	q.task = task
	q.input.SetValue(task.Title)
	q.input.Focus()
	q.input.CursorEnd()

	// Store original state
	q.original = &quickEditResult{
		title:       task.Title,
		titleSet:    true,
		priority:    task.Priority,
		prioritySet: true,
		dueDate:     task.DueDate,
		dueDateSet:  task.DueDate != nil,
		section:     task.Section,
		sectionSet:  true,
		status:      task.Status,
		statusSet:   true,
	}

	// Initialize preview with original values
	q.preview = &quickEditResult{
		title:       task.Title,
		titleSet:    true,
		priority:    task.Priority,
		prioritySet: true,
		dueDate:     task.DueDate,
		dueDateSet:  task.DueDate != nil,
		section:     task.Section,
		sectionSet:  true,
		status:      task.Status,
		statusSet:   true,
	}

	return textinput.Blink
}

// Exit exits quick edit mode.
func (q *QuickEditComponent) Exit() {
	q.mode = false
	q.rename = false
	q.task = nil
	q.preview = nil
	q.original = nil
	q.input.SetValue("")
	q.input.Blur()
}

// Update updates the quick edit component with a key message.
func (q *QuickEditComponent) Update(msg tea.KeyPressMsg) (textinput.Model, tea.Cmd) {
	return q.input.Update(msg)
}

// UpdateInput updates the input model directly.
func (q *QuickEditComponent) UpdateInput(input textinput.Model) {
	q.input = input
}

// HasTextInput returns true when quick edit mode is active.
func (q *QuickEditComponent) HasTextInput() bool {
	return q.mode
}

// ShouldCaptureEsc returns true when quick edit needs esc for internal handling.
func (q *QuickEditComponent) ShouldCaptureEsc() bool {
	return q.mode
}
