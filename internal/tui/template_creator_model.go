package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

// SubtaskFormState holds the form state for a single subtask (Epic/Story only).
type SubtaskFormState struct {
	titleInput textinput.Model
	typeCursor int // 0=Task, 1=Bug, 2=Idea
	optional   bool
}

// subtaskTypes lists available types for subtasks.
var subtaskTypes = []models.TaskType{
	models.TaskTypeTask,
	models.TaskTypeBug,
	models.TaskTypeIdea,
}

const (
	templateTabFields = iota
	templateTabChecklist
	templateTabSubtasks
)

var templateTabNames = []string{"Fields", "Checklist", "Subtasks"}

// TaskTemplateCreatorModel handles creating a new task template via TUI.
type TaskTemplateCreatorModel struct {
	config        *config.Config
	configManager *config.Manager

	// Form fields
	nameInput      textinput.Model
	descInput      textarea.Model
	tagsInput      textinput.Model
	sectionInput   textinput.Model
	typeCursor     int
	types          []models.TaskType
	categoryCursor int
	categories     []string
	priorityCursor int

	// Checklist items (simple strings, available for ALL task types)
	checklist []textinput.Model

	// Subtasks (Epic/Story only, with type and optional flag)
	subtasks []SubtaskFormState

	// Focus tracking
	// 0=name, 1=type, 2=category, 3=priority, 4=section, 5=tags, 6=description
	// 7...N = checklist items (all types)
	// N+1...M = subtask items (Epic/Story only)
	focusIndex int

	// Tab navigation
	activeTab    int // 0=Fields, 1=Checklist, 2=Subtasks
	scrollOffset int // Scroll offset for view content

	// For editing existing templates
	editingTemplate *template.TaskTemplate

	// Autocomplete caches (gathered from all projects)
	sectionCache []string
	tagCache     []string

	// Autocomplete state
	sectionSuggestions []string
	tagSuggestions     []string
	suggestionCursor   int
	showSuggestions    bool

	width  int
	height int
	err    error
}

// NewTaskTemplateCreatorModel creates a new task template creator.
func NewTaskTemplateCreatorModel(cfg *config.Config, manager *config.Manager) *TaskTemplateCreatorModel {
	factory := DefaultFormFieldFactory()

	nameInput := factory.NewTextInput("Template name")
	nameInput.Focus()

	descInput := textarea.New()
	descInput.Placeholder = "Description (optional)"
	descInput.SetHeight(3)
	descInput.CharLimit = 500
	descInput.ShowLineNumbers = false
	applyThemedTextarea(&descInput)

	tagsInput := factory.NewTextInput("Tags (comma-separated)")
	sectionInput := factory.NewTextInput("Section (optional)")

	types := []models.TaskType{
		models.TaskTypeTask,
		models.TaskTypeStory,
		models.TaskTypeEpic,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}

	categories := []string{"", "bug", "feature", "process"}

	m := &TaskTemplateCreatorModel{
		config:         cfg,
		configManager:  manager,
		nameInput:      nameInput,
		descInput:      descInput,
		tagsInput:      tagsInput,
		sectionInput:   sectionInput,
		typeCursor:     0,
		types:          types,
		categoryCursor: 0,
		categories:     categories,
		priorityCursor: 0,
		checklist:      nil,
		subtasks:       nil,
		focusIndex:     0,
		width:          80,
		height:         24,
	}
	m.buildCaches()
	return m
}

// NewTaskTemplateEditorModel creates a model for editing an existing template.
func NewTaskTemplateEditorModel(cfg *config.Config, manager *config.Manager, t template.TaskTemplate) *TaskTemplateCreatorModel {
	m := NewTaskTemplateCreatorModel(cfg, manager)
	m.editingTemplate = &t

	m.nameInput.SetValue(t.Name)

	for i, tp := range m.types {
		if tp == t.Type {
			m.typeCursor = i
			break
		}
	}

	for i, cat := range m.categories {
		if cat == t.Category {
			m.categoryCursor = i
			break
		}
	}

	m.priorityCursor = t.Priority
	m.sectionInput.SetValue(t.Section)

	if len(t.Tags) > 0 {
		m.tagsInput.SetValue(joinTags(t.Tags))
	}

	if t.Description != "" {
		m.descInput.SetValue(t.Description)
	}

	if len(t.Checklist) > 0 {
		m.checklist = make([]textinput.Model, len(t.Checklist))
		for i, item := range t.Checklist {
			factory := DefaultFormFieldFactory()
			input := factory.NewTextInput("Checklist item")
			input.SetValue(item)
			m.checklist[i] = input
		}
	}

	if len(t.Children) > 0 && t.Type.CanHaveChildren() {
		m.subtasks = make([]SubtaskFormState, len(t.Children))
		for i, child := range t.Children {
			factory := DefaultFormFieldFactory()
			input := factory.NewTextInput("Subtask title")
			input.SetValue(child.Title)

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

	return m
}

// joinTags joins tags with commas.
func joinTags(tags []string) string {
	var result strings.Builder
	for i, tag := range tags {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(tag)
	}
	return result.String()
}

// SetSize sets the view dimensions.
func (m *TaskTemplateCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextInput(&m.nameInput, width)
	formFactory.ResizeTextarea(&m.descInput, width)
	formFactory.ResizeTextInput(&m.tagsInput, width)
	formFactory.ResizeTextInput(&m.sectionInput, width)
	for i := range m.checklist {
		formFactory.ResizeTextInput(&m.checklist[i], width)
	}
	for i := range m.subtasks {
		formFactory.ResizeTextInput(&m.subtasks[i].titleInput, width)
	}
}

// Init initializes the model.
func (m *TaskTemplateCreatorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *TaskTemplateCreatorModel) StatusBarLabel() string { return "New Template" }
