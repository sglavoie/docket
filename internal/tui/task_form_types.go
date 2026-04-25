package tui

import (
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

// formAttrsLabelWidth is the fixed label column width for type-attr rows.
// "Reproducible:" (13 chars) is the widest; +1 pad = 14.
const formAttrsLabelWidth = 14

// formSubtaskDraft is a draft subtask entry on the Children tab.
// Only used in FormModeCreate for Epic/Story types.
type formSubtaskDraft struct {
	title    string
	taskType models.TaskType
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
	taskType   models.TaskType
	priority   int
	parentID   *string
	section    string
	pinned     bool
	active     bool
	dimmed     bool
	tags       []string
	dueDate    *time.Time
	dueInput   string
	hasTime    bool
	recurrence *models.RecurrencePattern
	desc       string
	url        string
	source     string

	// Type-specific attributes (only one set is active per taskType).
	bugAttrs   models.BugAttrs
	ideaAttrs  models.IdeaAttrs
	taskAttrs  models.TaskAttrs
	storyAttrs models.StoryAttrs
	epicAttrs  models.EpicAttrs

	// Children tab data.
	checklist []models.ChecklistItem
	subtasks  []formSubtaskDraft
}

// TaskFormModel drives both Create and Edit flows (D1, D11).
type TaskFormModel struct {
	mode          FormMode
	project       *models.Project
	task          *models.Task // non-nil in FormModeEdit
	draft         taskFormDraft
	config        *config.Config
	configManager *config.Manager
	fromTemplate  *template.TaskTemplate

	// Title input (Basics tab, field 0)
	titleInput textarea.Model

	// Due date input (Schedule tab, field 0)
	dueTextInput textinput.Model

	// URL and Source inputs (Details tab, fields 0 and 1)
	urlTextInput    textinput.Model
	sourceTextInput textinput.Model

	// Inline overlays (Basics and Schedule tabs)
	activeOverlay  formOverlayKind
	typePicker     *formTypePicker
	priorityPicker *formPriorityPicker
	flagsPicker    *formFlagsPicker
	duePicker      *formDueDatePicker
	attrPicker     *formAttrPicker

	// Children tab state
	checklistExpanded bool
	subtasksExpanded  bool
	childrenEditIdx   int  // which checklist/subtask item is being edited
	childrenAddingNew bool // true when the edit overlay was opened for a brand-new item

	// Inline edit inputs for Children tab overlays
	checklistItemInput textinput.Model
	subtaskItemInput   textinput.Model
	subtaskTypeCursor  int // cursor within formSubtaskTypes[] for subtask edit overlay

	// Navigation state
	activeTab  int
	focusIndex int

	// Dimensions
	width  int
	height int

	// UI state
	err            error
	hasChanges     bool
	confirmDiscard bool
	confirmOverlay ConfirmDialog
}

// childFocusKind identifies what the Children-tab focus points at.
type childFocusKind int

const (
	childFocusNone          childFocusKind = iota
	childFocusChecklistItem                // a checklist item (idx valid)
	childFocusSubtaskItem                  // a subtask draft  (idx valid)
)

// formSubtaskTypes lists the available child types selectable in the subtask edit overlay.
var formSubtaskTypes = []models.TaskType{
	models.TaskTypeTask,
	models.TaskTypeBug,
	models.TaskTypeIdea,
}
