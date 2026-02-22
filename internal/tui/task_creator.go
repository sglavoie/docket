package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

// formFactory is a shared factory instance for resizing form fields.
var formFactory = DefaultFormFieldFactory()

// Task creator tab constants
const (
	creatorTabFields    = 0
	creatorTabChecklist = 1
	creatorTabAttrs     = 2
	creatorTabSubtasks  = 3
)

var creatorTabNames = []string{"Fields", "Checklist", "Attributes", "Subtasks"}

// creatorFieldCount is the number of fields in the Fields tab.
const creatorFieldCount = 10 // title, type, parent, priority, section, due date, flags, recurrence, tags, description

// TaskCreatorModel handles creating a new task via TUI.
type TaskCreatorModel struct {
	project       *models.Project
	config        *config.Config
	configManager *config.Manager

	// Form fields
	titleInput textarea.Model
	descValue  string
	tags       []string

	// Selectable fields
	taskType         models.TaskType
	typeCursor       int
	types            []models.TaskType
	parentID         *string
	parentCursor     int
	validParents     []*models.Task
	priority         int
	section          string
	sectionCursor    int
	sections         []string
	dueDate          *time.Time
	dueDateInput     string
	pinned           bool
	active           bool
	flagCursor       int // 0=None, 1=Pin, 2=Active (mutually exclusive)
	recurrence       *models.RecurrencePattern
	recurrenceCursor int
	recurrenceOpts   []string

	// Tab navigation
	activeTab    int // 0=Fields, 1=Checklist, 2=Subtasks
	scrollOffset int // Scroll offset for view content

	// Checklist items (editable before save)
	checklist []textinput.Model

	// Subtasks (Epic/Story only)
	subtasks []SubtaskFormState // reuse SubtaskFormState from template_creator.go

	// Type-specific attributes
	// Bug attributes
	bugReproducibleCursor int
	bugSeverityCursor     int
	bugWorkaround         string

	// Idea attributes
	ideaCategoryCursor int
	ideaEffortCursor   int
	ideaValueCursor    int
	ideaValidated      bool

	// Task attributes
	taskEffortCursor   int
	taskWorkTypeCursor int

	// Story attributes
	storyPointsCursor int
	storyPersona      string

	// Epic attributes
	epicScopeCursor    int
	epicMilestone      string
	epicSuccessMetrics string

	// Type change confirmation
	confirmTypeChange bool
	pendingTypeCursor int
	pendingTaskType   models.TaskType

	// Form state
	focusIndex int // Tracks focus across all tabs
	width      int
	height     int
	err        error

	// Discard confirmation state
	confirmDiscard bool

	// Template support
	fromTemplate *template.TaskTemplate // Template used to create this task (for children creation)
}

// NewTaskCreatorModel creates a new task creator.
func NewTaskCreatorModel(project *models.Project, cfg *config.Config, manager *config.Manager, suggestedParent *models.Task) *TaskCreatorModel {
	factory := DefaultFormFieldFactory()

	titleInput := factory.NewTitleInput("Enter task title...")
	titleInput.Focus()
	titleInput.SetPromptFunc(0, func(lineIdx int) string { return "" })

	types := []models.TaskType{
		models.TaskTypeTask,
		models.TaskTypeStory,
		models.TaskTypeEpic,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}

	// Get available sections from project
	sections := project.GetSections()

	// Recurrence options: None, daily, weekly, biweekly, monthly, yearly, custom
	recurrenceOpts := []string{"None", "Daily", "Weekly", "Biweekly", "Monthly", "Yearly", "Custom..."}

	m := &TaskCreatorModel{
		project:          project,
		config:           cfg,
		configManager:    manager,
		titleInput:       titleInput,
		descValue:        "",
		tags:             nil,
		taskType:         models.TaskTypeTask,
		typeCursor:       0,
		types:            types,
		parentID:         nil,
		parentCursor:     0,
		priority:         3,
		section:          models.SectionNext,
		sectionCursor:    0,
		sections:         sections,
		dueDate:          nil,
		dueDateInput:     "",
		pinned:           false,
		active:           false,
		flagCursor:       0, // Default to None
		recurrence:       nil,
		recurrenceCursor: 0,
		recurrenceOpts:   recurrenceOpts,
		activeTab:        creatorTabFields,
		checklist:        nil,
		subtasks:         nil,
		focusIndex:       0,
		width:            80,
		height:           24,
	}

	// Set initial parent by finding nearest valid ancestor
	if suggestedParent != nil {
		// Walk up the parent chain to find a task that can accept this child type
		candidate := suggestedParent
		for candidate != nil {
			if candidate.CanAddChild(m.taskType) {
				m.parentID = &candidate.ID
				break
			}
			candidate = candidate.Parent
		}
	}

	m.updateValidParents()

	return m
}

// NewTaskCreatorModelWithTemplate creates a new task creator pre-filled from a template.
func NewTaskCreatorModelWithTemplate(project *models.Project, cfg *config.Config, manager *config.Manager, suggestedParent *models.Task, tmpl *template.TaskTemplate) *TaskCreatorModel {
	m := NewTaskCreatorModel(project, cfg, manager, suggestedParent)

	if tmpl != nil {
		m.fromTemplate = tmpl

		// Pre-fill type
		m.taskType = tmpl.Type
		for i, t := range m.types {
			if t == tmpl.Type {
				m.typeCursor = i
				break
			}
		}

		// Pre-fill priority
		if tmpl.Priority > 0 {
			m.priority = tmpl.Priority
		}

		// Pre-fill tags
		if len(tmpl.Tags) > 0 {
			m.tags = append([]string{}, tmpl.Tags...)
		}

		// Pre-fill section
		if tmpl.Section != "" {
			project.EnsureSection(tmpl.Section)
			m.section = tmpl.Section
			// Update cursor
			for i, s := range m.sections {
				if s == tmpl.Section {
					m.sectionCursor = i
					break
				}
			}
		}

		// Pre-fill description
		if tmpl.Description != "" {
			m.descValue = tmpl.Description
		}

		// Populate checklist items from template
		if len(tmpl.Checklist) > 0 {
			factory := DefaultFormFieldFactory()
			m.checklist = make([]textinput.Model, len(tmpl.Checklist))
			for i, item := range tmpl.Checklist {
				input := factory.NewTextInput("Checklist item")
				input.SetValue(item)
				m.checklist[i] = input
			}
		}

		// Populate subtasks from template (Epic/Story only)
		if len(tmpl.Children) > 0 && tmpl.Type.CanHaveChildren() {
			factory := DefaultFormFieldFactory()
			m.subtasks = make([]SubtaskFormState, len(tmpl.Children))
			for i, child := range tmpl.Children {
				input := factory.NewTextInput("Subtask title")
				input.SetValue(child.Title)

				// Find type cursor
				typeCursor := 0
				for j, st := range subtaskTypes {
					if st == child.Type || (child.Type == "" && st == models.TaskTypeTask) {
						typeCursor = j
						break
					}
				}

				m.subtasks[i] = SubtaskFormState{
					titleInput: input,
					typeCursor: typeCursor,
					optional:   child.Optional,
				}
			}
		}

		// Update valid parents for the new type
		m.updateValidParents()
	}

	return m
}

// updateValidParents filters tasks that can be parents of the current type.
func (m *TaskCreatorModel) updateValidParents() {
	m.validParents = []*models.Task{}

	for _, task := range m.project.Tasks {
		if task.CanAddChild(m.taskType) {
			m.validParents = append(m.validParents, task)
		}
	}

	// Reset parent selection if current parent is no longer valid
	if m.parentID != nil {
		found := false
		for i, p := range m.validParents {
			if p.ID == *m.parentID {
				found = true
				m.parentCursor = i + 1 // +1 because 0 is "None"
				break
			}
		}
		if !found {
			m.parentID = nil
			m.parentCursor = 0
		}
	}
}

// SetSize sets the view dimensions.
func (m *TaskCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextarea(&m.titleInput, width)
	for i := range m.checklist {
		formFactory.ResizeTextInput(&m.checklist[i], width)
	}
	for i := range m.subtasks {
		formFactory.ResizeTextInput(&m.subtasks[i].titleInput, width)
	}
}

func (m *TaskCreatorModel) Init() tea.Cmd {
	return textarea.Blink
}

// HasTextInput returns true because the task creator always has inputs.
func (m *TaskCreatorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true when confirmation is active.
func (m *TaskCreatorModel) ShouldCaptureEsc() bool {
	return m.confirmDiscard || m.confirmTypeChange
}

func (m *TaskCreatorModel) SupportsHelpShortcut() bool { return false }

// currentTypeCanHaveChildren returns true if the currently selected type can have children.
func (m *TaskCreatorModel) currentTypeCanHaveChildren() bool {
	return m.types[m.typeCursor].CanHaveChildren()
}

// checklistStartIndex returns the focus index where checklist items begin.
func (m *TaskCreatorModel) checklistStartIndex() int {
	return creatorFieldCount
}

// attrsStartIndex returns the focus index where attribute fields begin.
func (m *TaskCreatorModel) attrsStartIndex() int {
	return creatorFieldCount + len(m.checklist)
}

// getAttrFieldCount returns the number of focusable attribute fields for the current task type.
func (m *TaskCreatorModel) getAttrFieldCount() int {
	switch m.taskType {
	case models.TaskTypeBug:
		return 3 // reproducible, severity, workaround
	case models.TaskTypeIdea:
		return 4 // category, effort, value, validated
	case models.TaskTypeTask:
		return 2 // effort, workType
	case models.TaskTypeStory:
		return 2 // storyPoints, persona
	case models.TaskTypeEpic:
		return 3 // scope, milestone, successMetrics
	}
	return 0
}

// subtaskStartIndex returns the focus index where subtask items begin.
func (m *TaskCreatorModel) subtaskStartIndex() int {
	return m.attrsStartIndex() + m.getAttrFieldCount()
}

// getFirstFocusIndexForTab returns the starting focus index for a given tab.
func (m *TaskCreatorModel) getFirstFocusIndexForTab(tab int) int {
	switch tab {
	case creatorTabFields:
		return 0
	case creatorTabChecklist:
		return m.checklistStartIndex()
	case creatorTabAttrs:
		return m.attrsStartIndex()
	case creatorTabSubtasks:
		return m.subtaskStartIndex()
	}
	return 0
}

// getTabForFocusIndex returns which tab the given focus index belongs to.
func (m *TaskCreatorModel) getTabForFocusIndex(idx int) int {
	if idx < m.checklistStartIndex() {
		return creatorTabFields
	}
	if idx < m.attrsStartIndex() {
		return creatorTabChecklist
	}
	if idx < m.subtaskStartIndex() {
		return creatorTabAttrs
	}
	return creatorTabSubtasks
}

// getFocusCountForTab returns the number of focusable fields in a tab.
func (m *TaskCreatorModel) getFocusCountForTab(tab int) int {
	switch tab {
	case creatorTabFields:
		return creatorFieldCount
	case creatorTabChecklist:
		return len(m.checklist)
	case creatorTabAttrs:
		return m.getAttrFieldCount()
	case creatorTabSubtasks:
		if m.currentTypeCanHaveChildren() {
			return len(m.subtasks)
		}
		return 0
	}
	return 0
}

// focusFieldCount returns total number of focusable fields.
func (m *TaskCreatorModel) focusFieldCount() int {
	count := creatorFieldCount + len(m.checklist) + m.getAttrFieldCount()
	if m.currentTypeCanHaveChildren() {
		count += len(m.subtasks)
	}
	return count
}

// hasNonEmptyAttrs returns true if the current task type has any non-empty attribute values.
func (m *TaskCreatorModel) hasNonEmptyAttrs() bool {
	switch m.taskType {
	case models.TaskTypeBug:
		return m.bugReproducibleCursor > 0 || m.bugSeverityCursor > 0 || m.bugWorkaround != ""
	case models.TaskTypeIdea:
		return m.ideaCategoryCursor > 0 || m.ideaEffortCursor > 0 || m.ideaValueCursor > 0 || m.ideaValidated
	case models.TaskTypeTask:
		return m.taskEffortCursor > 0 || m.taskWorkTypeCursor > 0
	case models.TaskTypeStory:
		return m.storyPointsCursor > 0 || m.storyPersona != ""
	case models.TaskTypeEpic:
		return m.epicScopeCursor > 0 || m.epicMilestone != "" || m.epicSuccessMetrics != ""
	}
	return false
}

// clearCurrentAttrs resets all attribute values for the current task type.
func (m *TaskCreatorModel) clearCurrentAttrs() {
	switch m.taskType {
	case models.TaskTypeBug:
		m.bugReproducibleCursor = 0
		m.bugSeverityCursor = 0
		m.bugWorkaround = ""
	case models.TaskTypeIdea:
		m.ideaCategoryCursor = 0
		m.ideaEffortCursor = 0
		m.ideaValueCursor = 0
		m.ideaValidated = false
	case models.TaskTypeTask:
		m.taskEffortCursor = 0
		m.taskWorkTypeCursor = 0
	case models.TaskTypeStory:
		m.storyPointsCursor = 0
		m.storyPersona = ""
	case models.TaskTypeEpic:
		m.epicScopeCursor = 0
		m.epicMilestone = ""
		m.epicSuccessMetrics = ""
	}
}

// getAllowedSubtaskTypes returns the subtask types allowed for the current task type.
// Epic allows Task, Bug, Idea. Story allows only Task, Bug.
func (m *TaskCreatorModel) getAllowedSubtaskTypes() []models.TaskType {
	allowedChildren := m.taskType.AllowedChildren()
	var result []models.TaskType
	for _, childType := range allowedChildren {
		// Only include leaf types (Task, Bug, Idea) - not Epic or Story
		if !childType.CanHaveChildren() {
			result = append(result, childType)
		}
	}
	return result
}

// getFocusSection returns which section the current focus is in.
// Returns section name and index within that section (-1 if not applicable).
func (m *TaskCreatorModel) getFocusSection() (section string, idx int) {
	switch m.focusIndex {
	case 0:
		return "title", -1
	case 1:
		return "type", -1
	case 2:
		return "parent", -1
	case 3:
		return "priority", -1
	case 4:
		return "section", -1
	case 5:
		return "dueDate", -1
	case 6:
		return "flags", -1
	case 7:
		return "recurrence", -1
	case 8:
		return "tags", -1
	case 9:
		return "description", -1
	default:
		checklistStart := m.checklistStartIndex()
		attrsStart := m.attrsStartIndex()
		subtaskStart := m.subtaskStartIndex()

		if m.focusIndex >= checklistStart && m.focusIndex < attrsStart {
			return "checklist", m.focusIndex - checklistStart
		}
		if m.focusIndex >= attrsStart && m.focusIndex < subtaskStart {
			return "attrs", m.focusIndex - attrsStart
		}
		if m.currentTypeCanHaveChildren() && m.focusIndex >= subtaskStart {
			subtaskIdx := m.focusIndex - subtaskStart
			if subtaskIdx < len(m.subtasks) {
				return "subtask", subtaskIdx
			}
		}
		return "title", -1
	}
}

func (m *TaskCreatorModel) StatusBarLabel() string { return "New Task" }
