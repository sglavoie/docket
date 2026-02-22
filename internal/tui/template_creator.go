package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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

// Template creator tab constants
const (
	templateTabFields    = 0
	templateTabChecklist = 1
	templateTabSubtasks  = 2
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
		priorityCursor: 0, // 0 means default priority
		checklist:      nil,
		subtasks:       nil,
		focusIndex:     0,
		width:          80,
		height:         24,
	}
	m.buildCaches()
	return m
}

// buildCaches gathers sections and tags from all projects for autocomplete.
func (m *TaskTemplateCreatorModel) buildCaches() {
	sectionSet := make(map[string]struct{})
	tagSet := make(map[string]struct{})

	for _, proj := range m.config.Projects {
		// Load project to get tasks with tags
		if err := storage.LoadProject(proj); err == nil {
			// Add sections from project
			for _, section := range proj.Sections {
				sectionSet[section] = struct{}{}
			}
			// Add tags from all tasks
			for _, task := range proj.Tasks {
				for _, tag := range task.Tags {
					tagSet[tag] = struct{}{}
				}
			}
		}
	}

	// Convert sets to sorted slices
	m.sectionCache = make([]string, 0, len(sectionSet))
	for section := range sectionSet {
		m.sectionCache = append(m.sectionCache, section)
	}
	sort.Strings(m.sectionCache)

	m.tagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		m.tagCache = append(m.tagCache, tag)
	}
	sort.Strings(m.tagCache)
}

// NewTaskTemplateEditorModel creates a model for editing an existing template.
func NewTaskTemplateEditorModel(cfg *config.Config, manager *config.Manager, t template.TaskTemplate) *TaskTemplateCreatorModel {
	m := NewTaskTemplateCreatorModel(cfg, manager)
	m.editingTemplate = &t

	// Populate name
	m.nameInput.SetValue(t.Name)

	// Populate type
	for i, tp := range m.types {
		if tp == t.Type {
			m.typeCursor = i
			break
		}
	}

	// Populate category
	for i, cat := range m.categories {
		if cat == t.Category {
			m.categoryCursor = i
			break
		}
	}

	// Populate priority (0-5, where 0 means default)
	m.priorityCursor = t.Priority

	// Populate section
	m.sectionInput.SetValue(t.Section)

	// Populate tags
	if len(t.Tags) > 0 {
		m.tagsInput.SetValue(joinTags(t.Tags))
	}

	// Populate description
	if t.Description != "" {
		m.descInput.SetValue(t.Description)
	}

	// Populate checklist items
	if len(t.Checklist) > 0 {
		m.checklist = make([]textinput.Model, len(t.Checklist))
		for i, item := range t.Checklist {
			factory := DefaultFormFieldFactory()
			input := factory.NewTextInput("Checklist item")
			input.SetValue(item)
			m.checklist[i] = input
		}
	}

	// Populate subtasks (from Children, for Epic/Story only)
	if len(t.Children) > 0 && t.Type.CanHaveChildren() {
		m.subtasks = make([]SubtaskFormState, len(t.Children))
		for i, child := range t.Children {
			factory := DefaultFormFieldFactory()
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

	return m
}

// joinTags joins tags with commas.
func joinTags(tags []string) string {
	result := ""
	for i, tag := range tags {
		if i > 0 {
			result += ", "
		}
		result += tag
	}
	return result
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

// focusFieldCount returns total number of focusable fields.
// Base fields: name(1) + type(1) + category(1) + priority(1) + section(1) + tags(1) + description(1) = 7
// Plus checklist items (all types) plus subtasks (Epic/Story only)
func (m *TaskTemplateCreatorModel) focusFieldCount() int {
	count := 7 + len(m.checklist)
	if m.currentTypeCanHaveChildren() {
		count += len(m.subtasks)
	}
	return count
}

// currentTypeCanHaveChildren returns true if the currently selected type can have children.
func (m *TaskTemplateCreatorModel) currentTypeCanHaveChildren() bool {
	return m.types[m.typeCursor].CanHaveChildren()
}

// checklistStartIndex returns the focus index where checklist items begin.
func (m *TaskTemplateCreatorModel) checklistStartIndex() int {
	return 7
}

// subtaskStartIndex returns the focus index where subtask items begin.
func (m *TaskTemplateCreatorModel) subtaskStartIndex() int {
	return 7 + len(m.checklist)
}

// getFirstFocusIndexForTab returns the starting focus index for a given tab.
func (m *TaskTemplateCreatorModel) getFirstFocusIndexForTab(tab int) int {
	switch tab {
	case templateTabFields:
		return 0
	case templateTabChecklist:
		return m.checklistStartIndex()
	case templateTabSubtasks:
		return m.subtaskStartIndex()
	}
	return 0
}

// getTabForFocusIndex returns which tab the given focus index belongs to.
func (m *TaskTemplateCreatorModel) getTabForFocusIndex(idx int) int {
	if idx < m.checklistStartIndex() {
		return templateTabFields
	}
	if idx < m.subtaskStartIndex() {
		return templateTabChecklist
	}
	return templateTabSubtasks
}

// getFocusCountForTab returns the number of focusable fields in a tab.
func (m *TaskTemplateCreatorModel) getFocusCountForTab(tab int) int {
	switch tab {
	case templateTabFields:
		return 7 // name, type, category, priority, section, tags, description
	case templateTabChecklist:
		return len(m.checklist)
	case templateTabSubtasks:
		if m.currentTypeCanHaveChildren() {
			return len(m.subtasks)
		}
		return 0
	}
	return 0
}

// getFocusSection returns which section the current focus is in.
// Returns section name and index within that section (-1 if not applicable).
func (m *TaskTemplateCreatorModel) getFocusSection() (section string, idx int) {
	switch m.focusIndex {
	case 0:
		return "name", -1
	case 1:
		return "type", -1
	case 2:
		return "category", -1
	case 3:
		return "priority", -1
	case 4:
		return "section", -1
	case 5:
		return "tags", -1
	case 6:
		return "description", -1
	default:
		checklistStart := m.checklistStartIndex()
		subtaskStart := m.subtaskStartIndex()

		if m.focusIndex >= checklistStart && m.focusIndex < subtaskStart {
			return "checklist", m.focusIndex - checklistStart
		}
		if m.currentTypeCanHaveChildren() && m.focusIndex >= subtaskStart {
			subtaskIdx := m.focusIndex - subtaskStart
			if subtaskIdx < len(m.subtasks) {
				return "subtask", subtaskIdx
			}
		}
		return "name", -1
	}
}

// HasTextInput returns true to block : and ? keys in the template creator.
// This prevents the command palette and help from opening while editing.
func (m *TaskTemplateCreatorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true to let the creator handle esc internally.
func (m *TaskTemplateCreatorModel) ShouldCaptureEsc() bool {
	return true
}

func (m *TaskTemplateCreatorModel) SupportsHelpShortcut() bool { return false }

// updateSectionSuggestions filters section cache based on current input.
func (m *TaskTemplateCreatorModel) updateSectionSuggestions() {
	query := strings.ToLower(strings.TrimSpace(m.sectionInput.Value()))
	if query == "" {
		m.sectionSuggestions = nil
		m.showSuggestions = false
		return
	}

	m.sectionSuggestions = nil
	for _, section := range m.sectionCache {
		if strings.Contains(strings.ToLower(section), query) && strings.ToLower(section) != query {
			m.sectionSuggestions = append(m.sectionSuggestions, section)
		}
	}
	m.showSuggestions = len(m.sectionSuggestions) > 0
	m.suggestionCursor = 0
}

// updateTagSuggestions filters tag cache based on current tag being typed.
func (m *TaskTemplateCreatorModel) updateTagSuggestions() {
	// Get the last tag being typed (after the last comma)
	value := m.tagsInput.Value()
	parts := strings.Split(value, ",")
	currentTag := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))

	if currentTag == "" {
		m.tagSuggestions = nil
		m.showSuggestions = false
		return
	}

	// Get already-entered tags to exclude them
	existingTags := make(map[string]struct{})
	for i := 0; i < len(parts)-1; i++ {
		tag := strings.ToLower(strings.TrimSpace(parts[i]))
		if tag != "" {
			existingTags[tag] = struct{}{}
		}
	}

	m.tagSuggestions = nil
	for _, tag := range m.tagCache {
		tagLower := strings.ToLower(tag)
		// Match if contains query and not already added and not exact match
		if strings.Contains(tagLower, currentTag) && tagLower != currentTag {
			if _, exists := existingTags[tagLower]; !exists {
				m.tagSuggestions = append(m.tagSuggestions, tag)
			}
		}
	}
	m.showSuggestions = len(m.tagSuggestions) > 0
	m.suggestionCursor = 0
}

// acceptSectionSuggestion accepts the currently selected section suggestion.
func (m *TaskTemplateCreatorModel) acceptSectionSuggestion() {
	if m.suggestionCursor < len(m.sectionSuggestions) {
		m.sectionInput.SetValue(m.sectionSuggestions[m.suggestionCursor])
		m.sectionInput.SetCursor(len(m.sectionInput.Value()))
	}
	m.showSuggestions = false
	m.sectionSuggestions = nil
}

// acceptTagSuggestion accepts the currently selected tag suggestion.
func (m *TaskTemplateCreatorModel) acceptTagSuggestion() {
	if m.suggestionCursor < len(m.tagSuggestions) {
		// Replace the current tag being typed with the suggestion
		value := m.tagsInput.Value()
		parts := strings.Split(value, ",")
		parts[len(parts)-1] = " " + m.tagSuggestions[m.suggestionCursor]
		if len(parts) == 1 {
			parts[0] = m.tagSuggestions[m.suggestionCursor]
		}
		newValue := strings.Join(parts, ",")
		m.tagsInput.SetValue(newValue)
		m.tagsInput.SetCursor(len(newValue))
	}
	m.showSuggestions = false
	m.tagSuggestions = nil
}

// hideSuggestions hides the suggestions dropdown.
func (m *TaskTemplateCreatorModel) hideSuggestions() {
	m.showSuggestions = false
	m.sectionSuggestions = nil
	m.tagSuggestions = nil
	m.suggestionCursor = 0
}

// getCurrentSuggestions returns the current suggestion list based on focused field.
func (m *TaskTemplateCreatorModel) getCurrentSuggestions() []string {
	section, _ := m.getFocusSection()
	switch section {
	case "section":
		return m.sectionSuggestions
	case "tags":
		return m.tagSuggestions
	}
	return nil
}

// showAllSectionSuggestions populates sectionSuggestions with all cached sections.
func (m *TaskTemplateCreatorModel) showAllSectionSuggestions() {
	currentValue := strings.ToLower(strings.TrimSpace(m.sectionInput.Value()))
	m.sectionSuggestions = nil
	for _, section := range m.sectionCache {
		if strings.ToLower(section) != currentValue {
			m.sectionSuggestions = append(m.sectionSuggestions, section)
		}
	}
	if len(m.sectionSuggestions) > 0 {
		m.showSuggestions = true
		m.suggestionCursor = 0
	}
}

// showAllTagSuggestions populates tagSuggestions with all cached tags (excluding already entered).
func (m *TaskTemplateCreatorModel) showAllTagSuggestions() {
	// Get already-entered tags to exclude them
	value := m.tagsInput.Value()
	parts := strings.Split(value, ",")
	existingTags := make(map[string]struct{})
	for _, part := range parts {
		tag := strings.ToLower(strings.TrimSpace(part))
		if tag != "" {
			existingTags[tag] = struct{}{}
		}
	}

	m.tagSuggestions = nil
	for _, tag := range m.tagCache {
		if _, exists := existingTags[strings.ToLower(tag)]; !exists {
			m.tagSuggestions = append(m.tagSuggestions, tag)
		}
	}
	if len(m.tagSuggestions) > 0 {
		m.showSuggestions = true
		m.suggestionCursor = 0
	}
}
