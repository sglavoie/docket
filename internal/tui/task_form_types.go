package tui

import (
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// formSubtaskDraft is a draft subtask entry on the Children tab.
// Only used in FormModeCreate.
type formSubtaskDraft struct {
	title    string
	optional bool
}

// FormMode controls whether TaskFormModel is creating or editing a task (D1).
type FormMode int

const (
	FormModeCreate FormMode = iota
	FormModeEdit
)

// taskFormDraft holds all editable state for the form, independent of mode.
// Populated from a new task (create) or an existing task (edit).
type taskFormDraft struct {
	title      string
	priority   int
	parentID   *string
	state      models.WorkflowState
	pinned     bool
	tags       []string
	dueDate    *time.Time
	dueInput   string
	recurrence *models.RecurrencePattern
	desc       string

	// Children tab data.
	checklist []models.ChecklistItem
	subtasks  []formSubtaskDraft
}

// TaskFormModel drives both Create and Edit flows (D1, D11).
type TaskFormModel struct {
	mode       FormMode
	project    *models.Project
	task       *models.Task // non-nil in FormModeEdit
	draft      taskFormDraft
	runtime    *config.RuntimeState
	taskForm   TaskFormCoordinator
	presetPlan app.PlanningArtifactBuilder
	hostTitle  string

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

	// Navigation state
	activeTab         int
	focusIndex        int
	collapsedBasics   bool // true = show only Title/Priority + expand row
	collapsedSchedule bool // true = show only Due + expand row

	// Dimensions
	width  int
	height int

	// Open-after state: set when a subtask overlay confirms with Ctrl+O/Ctrl+D,
	// triggering auto-save and subsequent navigation to the created subtask.
	subtaskOpenIdx     int
	subtaskOpenDetail  bool
	subtaskOpenContent bool

	// UI state
	err        error
	hasChanges bool
	discardConfirmState
}

// childFocusKind identifies what the Children-tab focus points at.
type childFocusKind int

const (
	childFocusNone          childFocusKind = iota
	childFocusChecklistItem                // a checklist item (idx valid)
	childFocusSubtaskItem                  // a subtask draft  (idx valid)
)
