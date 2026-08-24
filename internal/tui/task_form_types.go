package tui

import (
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// formSubtaskDraft is a draft subtask entry on the Children tab.
// Used for child tasks created alongside the parent task.
type formSubtaskDraft struct {
	title string
}

// formAttachmentDraft is a draft attachment entry on the Extras tab. It holds
// the raw &link value verbatim; classification, path resolution, and the
// staged file copy all happen in the application layer at save time, so the
// form never has to know a URL from a code reference from a file.
type formAttachmentDraft struct {
	value string
}

// formCommentDraft is a draft comment entry on the Extras tab. Like the
// attachment draft it holds the raw text verbatim; normalization happens in
// the application layer at save time, on the same path a ;comment; takes.
type formCommentDraft struct {
	text string
}

// taskFormDraft holds all editable state for the form, independent of mode.
// Populated from a new task (create) or an existing task (edit).
type taskFormDraft struct {
	title    string
	priority int
	parentID *string
	state    models.WorkflowState
	pinned   bool
	tags     []string
	dueDate  *time.Time
	dueInput string
	desc     string

	// Children tab data.
	checklist []models.ChecklistItem
	subtasks  []formSubtaskDraft

	// Extras tab data.
	attachments []formAttachmentDraft
	comments    []formCommentDraft
}

// TaskFormModel drives expanded task creation.
type TaskFormModel struct {
	workspacePulseState
	project           *models.Project
	draft             taskFormDraft
	taskForm          TaskFormCoordinator
	presetDestination app.CaptureDestination
	hostTitle         string

	// Title input (Basics tab, field 0)
	titleInput textarea.Model

	// Due date input (Schedule tab, field 0)
	dueTextInput textinput.Model

	// Inline overlays (Basics and Schedule tabs)
	activeOverlay  formOverlayKind
	priorityPicker *formPriorityPicker
	statePicker    *formStatePicker
	flagsPicker    *formFlagsPicker
	duePicker      *formDueDatePicker

	// Children tab state
	checklistExpanded bool
	subtasksExpanded  bool
	childrenEditIdx   int  // which checklist/subtask item is being edited
	childrenAddingNew bool // true when the edit overlay was opened for a brand-new item

	// Inline edit inputs for Children tab overlays
	checklistItemInput textinput.Model
	subtaskItemInput   textinput.Model

	// Extras tab state
	attachmentsExpanded bool
	commentsExpanded    bool
	extrasEditIdx       int  // which attachment or comment draft is being edited
	extrasAddingNew     bool // true when the edit overlay was opened for a brand-new draft

	// Inline edit inputs for Extras tab overlays
	attachmentItemInput textinput.Model
	commentItemInput    textinput.Model

	// Navigation state
	activeTab       int
	focusIndex      int
	collapsedBasics bool // true = show only Title/Priority + expand row

	// Dimensions
	width  int
	height int

	// Edit-after state: set when a subtask overlay confirms with Ctrl+D,
	// triggering auto-save and subsequent description editing for the child.
	subtaskOpenIdx     int
	subtaskOpenContent bool

	// UI state
	err                error
	hasChanges         bool
	submissionInFlight bool
	discardConfirmState
}

// childFocusKind identifies what the Children-tab focus points at.
type childFocusKind int

const (
	childFocusNone          childFocusKind = iota
	childFocusChecklistItem                // a checklist item (idx valid)
	childFocusSubtaskItem                  // a subtask draft  (idx valid)
)

// extrasFocusKind identifies what the Extras-tab focus points at. It mirrors
// childFocusKind so the two dynamic tabs resolve focus the same way.
type extrasFocusKind int

const (
	extrasFocusNone           extrasFocusKind = iota
	extrasFocusAttachmentItem                 // an attachment draft (idx valid)
	extrasFocusCommentItem                    // a comment draft     (idx valid)
)
