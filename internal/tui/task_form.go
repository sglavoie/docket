package tui

import (
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

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

	// URL and Source inputs (Details tab, fields 1 and 2)
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

// NewTaskFormModelCreate creates a TaskFormModel for creating a new task.
func NewTaskFormModelCreate(
	project *models.Project,
	cfg *config.Config,
	mgr *config.Manager,
	suggestedParent *models.Task,
) *TaskFormModel {
	draft := taskFormDraft{
		taskType: models.TaskTypeTask,
		priority: 3,
		section:  models.SectionNext,
	}
	if suggestedParent != nil {
		// Walk up to find the nearest ancestor that can accept this child type
		candidate := suggestedParent
		for candidate != nil {
			if candidate.CanAddChild(draft.taskType) {
				id := candidate.ID
				draft.parentID = &id
				break
			}
			candidate = candidate.Parent
		}
	}

	m := &TaskFormModel{
		mode:          FormModeCreate,
		project:       project,
		draft:         draft,
		config:        cfg,
		configManager: mgr,
		activeTab:     formTabBasics,
		focusIndex:    0,
		width:         80,
		height:        24,
	}
	m.initTitleInput("")
	m.initDueInput("")
	m.initURLInput("")
	m.initSourceInput("")
	m.initChecklistItemInput("")
	m.initSubtaskItemInput("")
	return m
}

// NewTaskFormModelCreateFromTemplate creates a TaskFormModel pre-filled from a template.
func NewTaskFormModelCreateFromTemplate(
	project *models.Project,
	cfg *config.Config,
	mgr *config.Manager,
	suggestedParent *models.Task,
	tmpl *template.TaskTemplate,
) *TaskFormModel {
	m := NewTaskFormModelCreate(project, cfg, mgr, suggestedParent)
	m.fromTemplate = tmpl

	if tmpl == nil {
		return m
	}

	m.draft.taskType = tmpl.Type
	if tmpl.Priority > 0 {
		m.draft.priority = tmpl.Priority
	}
	if len(tmpl.Tags) > 0 {
		m.draft.tags = append([]string{}, tmpl.Tags...)
	}
	if tmpl.Section != "" {
		project.EnsureSection(tmpl.Section)
		m.draft.section = tmpl.Section
	}
	if tmpl.Description != "" {
		m.draft.desc = tmpl.Description
	}

	return m
}

// NewTaskFormModelEdit creates a TaskFormModel for editing an existing task.
func NewTaskFormModelEdit(
	project *models.Project,
	task *models.Task,
	cfg *config.Config,
	mgr *config.Manager,
) *TaskFormModel {
	draft := taskFormDraft{
		taskType:   task.Type,
		priority:   task.Priority,
		parentID:   task.ParentID,
		section:    task.Section,
		pinned:     task.Pinned,
		active:     task.Active,
		dimmed:     task.Dimmed,
		tags:       append([]string{}, task.Tags...),
		dueDate:    cloneTime(task.DueDate),
		dueInput:   formatDateInput(task.DueDate),
		recurrence: cloneRecurrence(task.Recurrence),
		desc:       task.Description,
		url:        task.URL,
		source:     task.Source,
		title:      task.Title,
		checklist:  append([]models.ChecklistItem(nil), task.Checklist...),
	}

	// Load type-specific attrs from the task.
	if task.BugAttrs != nil {
		draft.bugAttrs = *task.BugAttrs
	}
	if task.IdeaAttrs != nil {
		draft.ideaAttrs = *task.IdeaAttrs
	}
	if task.TaskAttrs != nil {
		draft.taskAttrs = *task.TaskAttrs
	}
	if task.StoryAttrs != nil {
		draft.storyAttrs = *task.StoryAttrs
	}
	if task.EpicAttrs != nil {
		draft.epicAttrs = *task.EpicAttrs
	}

	m := &TaskFormModel{
		mode:          FormModeEdit,
		project:       project,
		task:          task,
		draft:         draft,
		config:        cfg,
		configManager: mgr,
		activeTab:     formTabBasics,
		focusIndex:    0,
		width:         80,
		height:        24,
	}
	m.initTitleInput(draft.title)
	m.initDueInput(draft.dueInput)
	m.initURLInput(draft.url)
	m.initSourceInput(draft.source)
	m.initChecklistItemInput("")
	m.initSubtaskItemInput("")
	return m
}

// ── tea.Model ─────────────────────────────────────────────────────────────────

func (m *TaskFormModel) Init() tea.Cmd { return nil }

// ── View interface ────────────────────────────────────────────────────────────

func (m *TaskFormModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// ── ViewCapabilities ──────────────────────────────────────────────────────────

// ShouldCaptureEsc returns true so the form handles Esc internally (dirty-check).
func (m *TaskFormModel) ShouldCaptureEsc() bool { return true }

// HasTextInput returns true because the form always has text inputs.
func (m *TaskFormModel) HasTextInput() bool { return true }

// IsFullHeight returns false; the form renders its own footer but not the
// entire terminal (same as TaskCreatorModel).
func (m *TaskFormModel) IsFullHeight() bool { return false }

// ── Misc interface implementations ───────────────────────────────────────────

func (m *TaskFormModel) StatusBarLabel() string {
	if m.mode == FormModeCreate {
		return "New Task"
	}
	return "Edit Task"
}

// GetConfirmDialog returns the active confirm overlay (required by shell).
func (m *TaskFormModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}

// SupportsHelpShortcut is false while the form is a scaffold.
func (m *TaskFormModel) SupportsHelpShortcut() bool { return false }

// HandledKeys returns all keys TaskFormModel's Update method can process.
// Must stay in sync with keybindings_task_form.go and the switch cases in Update().
func (m *TaskFormModel) HandledKeys() []string {
	return []string{
		"[", "]",
		"tab", "shift+tab",
		"enter",
		"ctrl+s",
		"ctrl+v",
		"ctrl+e",
		"d",
		"c",
		"S",
		"ctrl+a",
		"ctrl+t",
		"ctrl+d",
		"ctrl+j",
		"ctrl+k",
		"ctrl+o",
		"space",
	}
}

// ── Internal navigation helpers ───────────────────────────────────────────────

func (m *TaskFormModel) nextTab() {
	m.activeTab = (m.activeTab + 1) % formTabCount
	m.focusIndex = 0
}

func (m *TaskFormModel) prevTab() {
	m.activeTab = (m.activeTab + formTabCount - 1) % formTabCount
	m.focusIndex = 0
}

// fieldCountForActiveTab returns the number of focusable fields on the current tab.
// Basics tab includes dynamic type-attr rows. Children tab is computed dynamically.
func (m *TaskFormModel) fieldCountForActiveTab() int {
	if m.activeTab == formTabBasics {
		return formBasicsFieldCount + formAttrFieldCount(m.draft.taskType)
	}
	if m.activeTab == formTabChildren {
		return m.fieldCountForChildren()
	}
	return formStaticFieldCount(m.activeTab)
}

// fieldCountForChildren returns the dynamic field count for the Children tab.
func (m *TaskFormModel) fieldCountForChildren() int {
	count := 1 // checklist section header (always present)
	if m.checklistExpanded {
		count += len(m.draft.checklist)
	}
	if m.mode == FormModeCreate && formTypeCanHaveSubtasks(m.draft.taskType) {
		count++ // subtasks section header
		if m.subtasksExpanded {
			count += len(m.draft.subtasks)
		}
	}
	return count
}

// childrenSubtasksHeaderIdx returns the focusIndex of the Subtasks section header.
func (m *TaskFormModel) childrenSubtasksHeaderIdx() int {
	idx := 1
	if m.checklistExpanded {
		idx += len(m.draft.checklist)
	}
	return idx
}

// clampChildrenFocusIndex ensures focusIndex is in range after section toggling.
func (m *TaskFormModel) clampChildrenFocusIndex() {
	count := m.fieldCountForChildren()
	if count == 0 {
		m.focusIndex = 0
		return
	}
	if m.focusIndex >= count {
		m.focusIndex = count - 1
	}
}

func (m *TaskFormModel) focusNext() {
	count := m.fieldCountForActiveTab()
	if count == 0 {
		return
	}
	m.focusIndex = (m.focusIndex + 1) % count
}

func (m *TaskFormModel) focusPrev() {
	count := m.fieldCountForActiveTab()
	if count == 0 {
		return
	}
	m.focusIndex = (m.focusIndex + count - 1) % count
}

// ── Title input helpers ───────────────────────────────────────────────────────

// initTitleInput creates and configures the title textarea with an optional initial value.
func (m *TaskFormModel) initTitleInput(value string) {
	factory := DefaultFormFieldFactory()
	m.titleInput = factory.NewTitleInputWithValue(value)
	// Focus by default (Basics tab, field 0 is selected on open)
	m.titleInput.Focus()
}

// syncInputFocus ensures all tab-specific text inputs have the correct focus state.
// Call this after any navigation change (tab switch, focus move).
func (m *TaskFormModel) syncInputFocus() {
	m.focusTitleInput()
	m.focusDueInput()
	m.focusURLInput()
	m.focusSourceInput()
}

// focusTitleInput focuses the title textarea when field 0 is active.
func (m *TaskFormModel) focusTitleInput() {
	if m.activeTab == formTabBasics && m.focusIndex == 0 {
		m.titleInput.Focus()
	} else {
		m.titleInput.Blur()
	}
}

// ── Due textinput helpers ──────────────────────────────────────────────────────

// initDueInput creates and configures the due date textinput with an optional initial value.
func (m *TaskFormModel) initDueInput(value string) {
	m.dueTextInput = textinput.New()
	m.dueTextInput.Placeholder = "tomorrow 5pm, +3d 14:30, fri, 2026-01-15"
	m.dueTextInput.CharLimit = 40
	m.dueTextInput.SetWidth(40)
	applyThemedTextInput(&m.dueTextInput)
	if value != "" {
		m.dueTextInput.SetValue(value)
	}
}

// focusDueInput focuses the due textinput when field 0 on the Schedule tab is active.
func (m *TaskFormModel) focusDueInput() {
	if m.activeTab == formTabSchedule && m.focusIndex == 0 {
		m.dueTextInput.Focus()
	} else {
		m.dueTextInput.Blur()
	}
}

// commitDueInput syncs the dueTextInput value back into the draft.
func (m *TaskFormModel) commitDueInput() {
	m.draft.dueInput = m.dueTextInput.Value()
	m.dueTextInput.Blur()
}

// ── URL textinput helpers ──────────────────────────────────────────────────────

// initURLInput creates and configures the URL textinput with an optional initial value.
func (m *TaskFormModel) initURLInput(value string) {
	m.urlTextInput = textinput.New()
	m.urlTextInput.Placeholder = "https://..."
	m.urlTextInput.CharLimit = 500
	m.urlTextInput.SetWidth(50)
	applyThemedTextInput(&m.urlTextInput)
	if value != "" {
		m.urlTextInput.SetValue(value)
	}
}

// focusURLInput focuses the URL textinput when field 1 on the Details tab is active.
func (m *TaskFormModel) focusURLInput() {
	if m.activeTab == formTabDetails && m.focusIndex == 1 {
		m.urlTextInput.Focus()
	} else {
		m.urlTextInput.Blur()
	}
}

// commitURLInput syncs the URL textinput value back into the draft.
func (m *TaskFormModel) commitURLInput() {
	m.draft.url = m.urlTextInput.Value()
	m.urlTextInput.Blur()
}

// ── Source textinput helpers ───────────────────────────────────────────────────

// initSourceInput creates and configures the Source textinput with an optional initial value.
func (m *TaskFormModel) initSourceInput(value string) {
	m.sourceTextInput = textinput.New()
	m.sourceTextInput.Placeholder = "source reference..."
	m.sourceTextInput.CharLimit = 500
	m.sourceTextInput.SetWidth(50)
	applyThemedTextInput(&m.sourceTextInput)
	if value != "" {
		m.sourceTextInput.SetValue(value)
	}
}

// focusSourceInput focuses the Source textinput when field 2 on the Details tab is active.
func (m *TaskFormModel) focusSourceInput() {
	if m.activeTab == formTabDetails && m.focusIndex == 2 {
		m.sourceTextInput.Focus()
	} else {
		m.sourceTextInput.Blur()
	}
}

// commitSourceInput syncs the Source textinput value back into the draft.
func (m *TaskFormModel) commitSourceInput() {
	m.draft.source = m.sourceTextInput.Value()
	m.sourceTextInput.Blur()
}

// ── Children tab input helpers ────────────────────────────────────────────────

// initChecklistItemInput creates and configures the checklist item edit textinput.
func (m *TaskFormModel) initChecklistItemInput(value string) {
	m.checklistItemInput = textinput.New()
	m.checklistItemInput.Placeholder = "Checklist item text..."
	m.checklistItemInput.CharLimit = 200
	m.checklistItemInput.SetWidth(50)
	applyThemedTextInput(&m.checklistItemInput)
	if value != "" {
		m.checklistItemInput.SetValue(value)
	}
}

// initSubtaskItemInput creates and configures the subtask title edit textinput.
func (m *TaskFormModel) initSubtaskItemInput(value string) {
	m.subtaskItemInput = textinput.New()
	m.subtaskItemInput.Placeholder = "Subtask title..."
	m.subtaskItemInput.CharLimit = 200
	m.subtaskItemInput.SetWidth(50)
	applyThemedTextInput(&m.subtaskItemInput)
	if value != "" {
		m.subtaskItemInput.SetValue(value)
	}
}

// formTypeCanHaveSubtasks reports whether a task type can have subtask children.
func formTypeCanHaveSubtasks(t models.TaskType) bool {
	return t == models.TaskTypeEpic || t == models.TaskTypeStory
}

// formSubtaskTypes lists the available child types selectable in the subtask edit overlay.
var formSubtaskTypes = []models.TaskType{
	models.TaskTypeTask,
	models.TaskTypeBug,
	models.TaskTypeIdea,
}

// formSubtaskTypeCursorFor returns the cursor index in formSubtaskTypes for the given type.
func formSubtaskTypeCursorFor(t models.TaskType) int {
	for i, st := range formSubtaskTypes {
		if st == t {
			return i
		}
	}
	return 0
}

// ── Attr row helper ───────────────────────────────────────────────────────────

// attrRowFocused returns true when the active tab is Basics and focus is on
// the attr row at the given local index (0-based within the type-attr section).
func (m *TaskFormModel) attrRowFocused(localIdx int) bool {
	return m.activeTab == formTabBasics && m.focusIndex == formBasicsFieldCount+localIdx
}

// cloneTime returns a deep copy of a *time.Time, or nil if the input is nil.
func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}

// cloneRecurrence returns a deep copy of a *models.RecurrencePattern, or nil.
func cloneRecurrence(value *models.RecurrencePattern) *models.RecurrencePattern {
	if value == nil {
		return nil
	}
	cp := *value
	return &cp
}

// formatDateInput formats a *time.Time as a "YYYY-MM-DD" string for text inputs.
func formatDateInput(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}
