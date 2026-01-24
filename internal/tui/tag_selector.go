package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// TagSelectorModel allows multi-selecting tags and adding new ones.
type TagSelectorModel struct {
	*BasePicker[TagInfo]
	task         *models.Task
	project      *models.Project
	originalTags []string

	// Add tag mode
	addTagMode        bool
	addTagJustEntered bool
	addTagInput       textinput.Model
	addTagError       string
}

// ShowTagSelectorMsg is sent to open the tag selector.
type ShowTagSelectorMsg struct {
	Task        *models.Task
	Project     *models.Project
	CurrentTags []string
}

// TagSelectorDoneMsg is sent when the tag selector is done.
type TagSelectorDoneMsg struct {
	Task         *models.Task
	Project      *models.Project
	SelectedTags []string
	Cancelled    bool
}

// ShowBulkTagSelectorMsg is sent to open the tag selector for multiple tasks.
type ShowBulkTagSelectorMsg struct {
	Tasks   []*models.Task
	Project *models.Project
}

// BulkTagSelectorDoneMsg is sent when the bulk tag selector is done.
type BulkTagSelectorDoneMsg struct {
	Tasks        []*models.Task
	Project      *models.Project
	SelectedTags []string
	Cancelled    bool
}

// BulkTagSelectorModel allows multi-selecting tags for multiple tasks.
type BulkTagSelectorModel struct {
	*BasePicker[TagInfo]
	tasks        []*models.Task
	project      *models.Project
	originalTags map[string][]string // taskID -> original tags

	// Add tag mode
	addTagMode        bool
	addTagJustEntered bool
	addTagInput       textinput.Model
	addTagError       string
}

// NewBulkTagSelectorModel creates a new bulk tag selector.
func NewBulkTagSelectorModel(tasks []*models.Task, project *models.Project) *BulkTagSelectorModel {
	tags := buildTagListForSelector(project)

	// Find common tags (present on all tasks)
	commonTags := findCommonTags(tasks)

	title := fmt.Sprintf("Tags for %d tasks", len(tasks))

	picker := NewBasePicker(tags, PickerConfig{
		Title:          title,
		EmptyMessage:   "No tags in project (press 'a' to add)",
		NoMatchMessage: "No matching tags (press 'a' to add)",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	// Pre-select common tags
	for _, tag := range commonTags {
		picker.PreSelectByID(tag)
	}

	// Position cursor on first common tag if any
	if len(commonTags) > 0 {
		picker.SetCursorToID(commonTags[0])
	}

	// Create add tag input
	addInput := textinput.New()
	addInput.Placeholder = "Enter tag name"
	addInput.CharLimit = 50

	// Save original tags for each task
	originalTags := make(map[string][]string)
	for _, task := range tasks {
		originalTags[task.ID] = append([]string{}, task.Tags...)
	}

	return &BulkTagSelectorModel{
		BasePicker:   picker,
		tasks:        tasks,
		project:      project,
		originalTags: originalTags,
		addTagInput:  addInput,
	}
}

// findCommonTags returns tags that are present on all tasks.
func findCommonTags(tasks []*models.Task) []string {
	if len(tasks) == 0 {
		return nil
	}

	// Count tag occurrences
	tagCount := make(map[string]int)
	for _, task := range tasks {
		for _, tag := range task.Tags {
			tagCount[tag]++
		}
	}

	// Find tags present on all tasks
	var common []string
	for tag, count := range tagCount {
		if count == len(tasks) {
			common = append(common, tag)
		}
	}
	sort.Strings(common)
	return common
}

// SetSize sets the view dimensions.
func (m *BulkTagSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *BulkTagSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *BulkTagSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Handle add tag mode first
		if m.addTagMode {
			return m.handleAddTagMode(msg)
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return BulkTagSelectorDoneMsg{
					Tasks:     m.tasks,
					Project:   m.project,
					Cancelled: true,
				}
			}

		case "enter":
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return BulkTagSelectorDoneMsg{
					Tasks:        m.tasks,
					Project:      m.project,
					SelectedTags: selectedTags,
					Cancelled:    false,
				}
			}

		case "a":
			// Enter add tag mode
			m.addTagMode = true
			m.addTagJustEntered = true
			m.addTagError = ""
			m.addTagInput.Reset()
			m.addTagInput.SetValue("")
			m.addTagInput.Focus()
			return m, textinput.Blink

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// handleAddTagMode handles keyboard input when in add tag mode.
func (m *BulkTagSelectorModel) handleAddTagMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Skip the first keystroke (the 'a' that triggered add mode)
	if m.addTagJustEntered {
		m.addTagJustEntered = false
		return m, nil
	}

	switch msg.String() {
	case "esc":
		// Cancel add mode
		m.addTagMode = false
		m.addTagError = ""
		m.addTagInput.Blur()
		return m, nil

	case "enter":
		// Add the tag
		tagName := strings.TrimSpace(m.addTagInput.Value())

		// Validate
		if tagName == "" {
			m.addTagError = "Tag name cannot be empty"
			return m, nil
		}

		// Check for duplicate (case-sensitive)
		isDuplicate := false
		for _, item := range m.Items {
			if item.Name == tagName {
				isDuplicate = true
				break
			}
		}

		if isDuplicate {
			// Auto-select the existing tag
			m.PreSelectByID(tagName)
			m.SetCursorToID(tagName)
			m.addTagError = fmt.Sprintf("Tag '%s' already exists - selected it", tagName)
			m.addTagMode = false
			m.addTagInput.Blur()
			return m, nil
		}

		// Add new tag to the list
		newTag := TagInfo{Name: tagName, TaskCount: 0}
		m.Items = append(m.Items, newTag)

		// Re-sort items
		sort.Slice(m.Items, func(i, j int) bool {
			return m.Items[i].Name < m.Items[j].Name
		})

		// Update filtered list and select the new tag
		m.Filter(m.SearchQuery())
		m.PreSelectByID(tagName)
		m.SetCursorToID(tagName)

		// Exit add mode
		m.addTagMode = false
		m.addTagError = ""
		m.addTagInput.Blur()
		return m, nil

	default:
		// Update text input
		var cmd tea.Cmd
		m.addTagInput, cmd = m.addTagInput.Update(msg)
		return m, cmd
	}
}

// HasActiveInput returns true when in add tag mode.
func (m *BulkTagSelectorModel) HasActiveInput() bool {
	return m.addTagMode
}

// HasTextInput returns true when the add-tag input is active.
func (m *BulkTagSelectorModel) HasTextInput() bool {
	return m.HasActiveInput()
}

// ShouldCaptureEsc returns true when the add-tag input is active.
func (m *BulkTagSelectorModel) ShouldCaptureEsc() bool {
	return m.HasActiveInput()
}

// View renders the bulk tag selector.
func (m *BulkTagSelectorModel) View() string {
	// If in add tag mode, show add tag UI
	if m.addTagMode {
		return m.renderAddTagMode()
	}

	// Build custom view with 'a' key support
	boxWidth := 60
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	// Search bar
	content.WriteString(m.RenderSearchBar())

	// List
	maxVisible := 15
	if m.Height() > 0 {
		maxVisible = m.Height() - 14
	}
	content.WriteString(m.RenderList(maxVisible))

	// Custom footer with 'a' key
	content.WriteString("\n")
	content.WriteString(m.renderCustomFooter())

	// Build subtitle from selection count
	selectedCount := len(m.Selected)
	var subtitle string
	if selectedCount == 0 {
		subtitle = "No tags selected"
	} else {
		subtitle = fmt.Sprintf("%d tag%s selected", selectedCount, pluralize(selectedCount, "", "s"))
	}

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    m.Config.Title,
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}

// renderCustomFooter renders the footer with 'a' key for adding tags.
func (m *BulkTagSelectorModel) renderCustomFooter() string {
	if m.IsSearchMode() {
		return MutedStyle.Render("enter done | esc clear")
	}

	if len(m.Filtered) == 0 {
		return MutedStyle.Render("a add tag | esc cancel")
	}

	return MutedStyle.Render("space toggle | a add tag | / search | c clear | enter apply | esc cancel")
}

// renderAddTagMode renders the add tag input UI.
func (m *BulkTagSelectorModel) renderAddTagMode() string {
	var content strings.Builder

	// Instructions
	content.WriteString(MutedStyle.Render("Enter a tag name (max 50 characters)"))
	content.WriteString("\n\n")

	// Input field
	content.WriteString(m.addTagInput.View())
	content.WriteString("\n\n")

	// Error message if any
	if m.addTagError != "" {
		errorStyle := BoldStyle.Foreground(ColorError)
		content.WriteString(errorStyle.Render(m.addTagError))
		content.WriteString("\n\n")
	}

	// Footer
	content.WriteString(MutedStyle.Render("enter add tag | esc cancel"))

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Add New Tag",
		Width:    50,
		Centered: true,
	})
}

// NewTagSelectorModel creates a new tag selector.
func NewTagSelectorModel(task *models.Task, project *models.Project, currentTags []string) *TagSelectorModel {
	tags := buildTagListForSelector(project)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          fmt.Sprintf("Tags for #%d: %s", task.ShortID, task.Title),
		EmptyMessage:   "No tags in project (press 'a' to add)",
		NoMatchMessage: "No matching tags (press 'a' to add)",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	// Pre-select current tags
	for _, tag := range currentTags {
		picker.PreSelectByID(tag)
	}

	// Position cursor on first selected tag if any
	if len(currentTags) > 0 {
		picker.SetCursorToID(currentTags[0])
	}

	// Create add tag input
	addInput := textinput.New()
	addInput.Placeholder = "Enter tag name"
	addInput.CharLimit = 50

	return &TagSelectorModel{
		BasePicker:   picker,
		task:         task,
		project:      project,
		originalTags: append([]string{}, currentTags...), // Copy
		addTagInput:  addInput,
	}
}

// buildTagListForSelector scans all tasks and builds a sorted list of tags with counts.
func buildTagListForSelector(project *models.Project) []TagInfo {
	tagCounts := make(map[string]int)
	for _, task := range project.Tasks {
		for _, tag := range task.Tags {
			tagCounts[tag]++
		}
	}

	tags := make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		tags = append(tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}

// SetSize sets the view dimensions.
func (m *TagSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *TagSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TagSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Handle add tag mode first
		if m.addTagMode {
			return m.handleAddTagMode(msg)
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return TagSelectorDoneMsg{
					Task:      m.task,
					Project:   m.project,
					Cancelled: true,
				}
			}

		case "enter":
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return TagSelectorDoneMsg{
					Task:         m.task,
					Project:      m.project,
					SelectedTags: selectedTags,
					Cancelled:    false,
				}
			}

		case "a":
			// Enter add tag mode
			m.addTagMode = true
			m.addTagJustEntered = true
			m.addTagError = ""
			m.addTagInput.Reset()
			m.addTagInput.SetValue("")
			m.addTagInput.Focus()
			return m, textinput.Blink

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// handleAddTagMode handles keyboard input when in add tag mode.
func (m *TagSelectorModel) handleAddTagMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Skip the first keystroke (the 'a' that triggered add mode)
	if m.addTagJustEntered {
		m.addTagJustEntered = false
		return m, nil
	}

	switch msg.String() {
	case "esc":
		// Cancel add mode
		m.addTagMode = false
		m.addTagError = ""
		m.addTagInput.Blur()
		return m, nil

	case "enter":
		// Add the tag
		tagName := strings.TrimSpace(m.addTagInput.Value())

		// Validate
		if tagName == "" {
			m.addTagError = "Tag name cannot be empty"
			return m, nil
		}

		// Check for duplicate (case-sensitive)
		isDuplicate := false
		for _, item := range m.Items {
			if item.Name == tagName {
				isDuplicate = true
				break
			}
		}

		if isDuplicate {
			// Auto-select the existing tag
			m.PreSelectByID(tagName)
			m.SetCursorToID(tagName)
			m.addTagError = fmt.Sprintf("Tag '%s' already exists - selected it", tagName)
			m.addTagMode = false
			m.addTagInput.Blur()
			return m, nil
		}

		// Add new tag to the list
		newTag := TagInfo{Name: tagName, TaskCount: 0}
		m.Items = append(m.Items, newTag)

		// Re-sort items
		sort.Slice(m.Items, func(i, j int) bool {
			return m.Items[i].Name < m.Items[j].Name
		})

		// Update filtered list and select the new tag
		m.Filter(m.SearchQuery())
		m.PreSelectByID(tagName)
		m.SetCursorToID(tagName)

		// Exit add mode
		m.addTagMode = false
		m.addTagError = ""
		m.addTagInput.Blur()
		return m, nil

	default:
		// Update text input
		var cmd tea.Cmd
		m.addTagInput, cmd = m.addTagInput.Update(msg)
		return m, cmd
	}
}

// HasActiveInput returns true when in add tag mode.
func (m *TagSelectorModel) HasActiveInput() bool {
	return m.addTagMode
}

// HasTextInput returns true when the add-tag input is active.
func (m *TagSelectorModel) HasTextInput() bool {
	return m.HasActiveInput()
}

// ShouldCaptureEsc returns true when the add-tag input is active.
func (m *TagSelectorModel) ShouldCaptureEsc() bool {
	return m.HasActiveInput()
}

// View renders the tag selector.
func (m *TagSelectorModel) View() string {
	// If in add tag mode, show add tag UI
	if m.addTagMode {
		return m.renderAddTagMode()
	}

	// Build custom view with 'a' key support
	boxWidth := 60
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	// Search bar
	content.WriteString(m.RenderSearchBar())

	// List
	maxVisible := 15
	if m.Height() > 0 {
		maxVisible = m.Height() - 14
	}
	content.WriteString(m.RenderList(maxVisible))

	// Custom footer with 'a' key
	content.WriteString("\n")
	content.WriteString(m.renderCustomFooter())

	// Build subtitle from selection count
	selectedCount := len(m.Selected)
	var subtitle string
	if selectedCount == 0 {
		subtitle = "No tags selected"
	} else {
		subtitle = fmt.Sprintf("%d tag%s selected", selectedCount, pluralize(selectedCount, "", "s"))
	}

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    m.Config.Title,
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}

// renderCustomFooter renders the footer with 'a' key for adding tags.
func (m *TagSelectorModel) renderCustomFooter() string {
	if m.IsSearchMode() {
		return MutedStyle.Render("enter done | esc clear")
	}

	if len(m.Filtered) == 0 {
		return MutedStyle.Render("a add tag | esc cancel")
	}

	return MutedStyle.Render("space toggle | a add tag | / search | c clear | enter apply | esc cancel")
}

// renderAddTagMode renders the add tag input UI.
func (m *TagSelectorModel) renderAddTagMode() string {
	var content strings.Builder

	// Instructions
	content.WriteString(MutedStyle.Render("Enter a tag name (max 50 characters)"))
	content.WriteString("\n\n")

	// Input field
	content.WriteString(m.addTagInput.View())
	content.WriteString("\n\n")

	// Error message if any
	if m.addTagError != "" {
		errorStyle := BoldStyle.Foreground(ColorError)
		content.WriteString(errorStyle.Render(m.addTagError))
		content.WriteString("\n\n")
	}

	// Footer
	content.WriteString(MutedStyle.Render("enter add tag | esc cancel"))

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Add New Tag",
		Width:    50,
		Centered: true,
	})
}
