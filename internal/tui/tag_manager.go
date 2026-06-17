package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TagInfo holds a tag name, its usage count, and project-manager pin state.
type TagInfo struct {
	Name      string
	TaskCount int
	Pinned    bool
}

// TagManagerModel is the model for the tag management view.
type TagManagerModel struct {
	projectTree   *ProjectTreeModel
	project       *models.Project // Used directly when projectTree is nil
	tags          []TagInfo
	cursor        int
	width         int
	height        int
	renameMode    bool
	renameInput   textinput.Model
	renameTarget  string
	addTagMode    bool
	addTagInput   textinput.Model
	errorMsg      string
	confirmAction string
	discardConfirmState
}

// ShowTagManagerMsg is sent to open the tag manager view.
type ShowTagManagerMsg struct {
	ProjectTree *ProjectTreeModel
	Project     *models.Project // Used when ProjectTree is nil (e.g., from outliner)
}

// CloseTagManagerMsg is sent when the tag manager should close.
type CloseTagManagerMsg struct{ PopViewMsg }

// TagRenamedMsg is sent after a tag has been renamed.
type TagRenamedMsg struct {
	OldName string
	NewName string
}

// TagAddedMsg is sent after a project catalog tag has been added.
type TagAddedMsg struct {
	Name string
}

// TagRemovedMsg is sent after a project catalog tag has been removed.
type TagRemovedMsg struct {
	Name string
}

// TagPinnedMsg is sent after a project tag has been pinned or unpinned.
type TagPinnedMsg struct {
	Name   string
	Pinned bool
}

// TagManagerErrorMsg reports an error that should stay visible in the manager.
type TagManagerErrorMsg struct {
	Err error
}

// NewTagManagerModel creates a new tag manager model.
func NewTagManagerModel(pt *ProjectTreeModel) *TagManagerModel {
	return newTagManagerModel(pt, nil)
}

// NewTagManagerModelForProject creates a tag manager backed by a standalone project.
func NewTagManagerModelForProject(project *models.Project) *TagManagerModel {
	return newTagManagerModel(nil, project)
}

func newTagManagerModel(pt *ProjectTreeModel, project *models.Project) *TagManagerModel {
	renameInput := textinput.New()
	renameInput.Placeholder = "new tag name"
	renameInput.CharLimit = 50
	renameInput.SetWidth(30)
	applyThemedTextInput(&renameInput)

	addTagInput := textinput.New()
	addTagInput.Placeholder = "tag name"
	addTagInput.CharLimit = 50
	addTagInput.SetWidth(30)
	applyThemedTextInput(&addTagInput)

	m := &TagManagerModel{
		projectTree: pt,
		project:     project,
		cursor:      0,
		renameInput: renameInput,
		addTagInput: addTagInput,
	}
	if pt != nil {
		m.project = pt.project
	}
	m.buildTagList()
	return m
}

// SetSize sets the view dimensions.
func (m *TagManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildTagList builds a catalog-aware sorted list of tags with task-use counts.
func (m *TagManagerModel) buildTagList() {
	m.tags = BuildProjectTagList(m.project, false)
	if m.project != nil {
		for i := range m.tags {
			m.tags[i].Pinned = m.project.IsTagPinned(m.tags[i].Name)
		}
	}

	// Clamp cursor
	if m.cursor >= len(m.tags) {
		m.cursor = len(m.tags) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *TagManagerModel) selectTag(name string) {
	for i, tag := range m.tags {
		if tag.Name == name {
			m.cursor = i
			return
		}
	}
}

// renameTag renames a tag in the project catalog and task usages.
func (m *TagManagerModel) renameTag(oldName, newName string) tea.Cmd {
	project := m.project
	return func() tea.Msg {
		if project == nil {
			return TagManagerErrorMsg{Err: fmt.Errorf("project is unavailable")}
		}
		newName = models.NormalizeTag(newName)
		if newName == "" || newName == oldName {
			return nil
		}

		changed, err := project.RenameTag(oldName, newName)
		if err != nil {
			return TagManagerErrorMsg{Err: err}
		}
		if !changed {
			return nil
		}

		if err := saveProjectSync(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TagRenamedMsg{OldName: oldName, NewName: newName}
	}
}

func (m *TagManagerModel) addProjectTag(tagName string) tea.Cmd {
	project := m.project
	return func() tea.Msg {
		if project == nil {
			return TagManagerErrorMsg{Err: fmt.Errorf("project is unavailable")}
		}
		tagName = models.NormalizeTag(tagName)
		added, err := project.AddTag(tagName)
		if err != nil {
			return TagManagerErrorMsg{Err: err}
		}
		if !added {
			return TagManagerErrorMsg{Err: fmt.Errorf("tag '%s' already exists", tagName)}
		}

		if err := saveProjectSync(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TagAddedMsg{Name: tagName}
	}
}

func (m *TagManagerModel) removeProjectTag(tagName string) tea.Cmd {
	project := m.project
	return func() tea.Msg {
		if project == nil {
			return TagManagerErrorMsg{Err: fmt.Errorf("project is unavailable")}
		}
		tagName = models.NormalizeTag(tagName)
		result, err := project.DeleteTag(tagName)
		if err != nil {
			return TagManagerErrorMsg{Err: err}
		}
		if !result.Changed() {
			return TagManagerErrorMsg{Err: fmt.Errorf("tag '%s' does not exist", tagName)}
		}

		if err := saveProjectSync(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TagRemovedMsg{Name: result.Name}
	}
}

func (m *TagManagerModel) togglePinnedTag(tagName string) tea.Cmd {
	project := m.project
	return func() tea.Msg {
		if project == nil {
			return TagManagerErrorMsg{Err: fmt.Errorf("project is unavailable")}
		}
		tagName = models.NormalizeTag(tagName)
		pinned, err := project.TogglePinnedTag(tagName)
		if err != nil {
			return TagManagerErrorMsg{Err: err}
		}

		if err := saveProjectSync(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TagPinnedMsg{Name: tagName, Pinned: pinned}
	}
}

// filterByTag sets the project tree filter and closes the tag manager.
// When projectTree is nil (outliner), it just closes without applying filters.
func (m *TagManagerModel) filterByTag(tagName string) tea.Cmd {
	if m.projectTree != nil {
		m.projectTree.filter.Tags = map[string]bool{tagName: true}
		m.projectTree.updateVisibility()
		m.projectTree.clampCursor()
	}
	return func() tea.Msg {
		return CloseTagManagerMsg{}
	}
}

// Init initializes the model.
func (m *TagManagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TagManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case TagRenamedMsg:
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		m.buildTagList()
		m.selectTag(msg.NewName)
		return m, nil

	case TagAddedMsg:
		m.addTagMode = false
		m.addTagInput.SetValue("")
		m.addTagInput.Blur()
		m.buildTagList()
		m.selectTag(msg.Name)
		return m, nil

	case TagRemovedMsg:
		m.buildTagList()
		return m, nil

	case TagPinnedMsg:
		m.buildTagList()
		m.selectTag(msg.Name)
		return m, nil

	case TagManagerErrorMsg:
		if msg.Err != nil {
			m.errorMsg = msg.Err.Error()
		}
		return m, nil

	case tea.KeyPressMsg:
		// Handle discard confirmation
		if m.confirmDiscard {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				switch m.confirmAction {
				case "discard-rename":
					m.renameMode = false
					m.renameTarget = ""
					m.renameInput.SetValue("")
				case "discard-add":
					m.addTagMode = false
					m.addTagInput.SetValue("")
					m.addTagInput.Blur()
				}
				m.confirmAction = ""
				return m, nil
			case "n", "N", "esc":
				m.confirmDiscard = false
				m.confirmAction = ""
				return m, nil
			}
			return m, nil
		}

		// Clear error on any key press
		m.errorMsg = ""

		if m.renameMode {
			return m.handleRenameMode(msg)
		}

		if m.addTagMode {
			return m.handleAddTagMode(msg)
		}

		switch msg.String() {
		case "esc", "ctrl+t":
			return m, func() tea.Msg {
				return CloseTagManagerMsg{}
			}

		case "j", "down":
			if len(m.tags) > 0 {
				m.cursor++
				if m.cursor >= len(m.tags) {
					m.cursor = 0
				}
			}
			return m, nil

		case "k", "up":
			if len(m.tags) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.tags) - 1
				}
			}
			return m, nil

		case "enter":
			if len(m.tags) > 0 && m.cursor < len(m.tags) {
				return m, m.filterByTag(m.tags[m.cursor].Name)
			}
			return m, nil

		case "r":
			if len(m.tags) > 0 && m.cursor < len(m.tags) {
				m.renameMode = true
				m.renameTarget = m.tags[m.cursor].Name
				m.renameInput.SetValue(m.renameTarget)
				m.renameInput.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case "d":
			if len(m.tags) > 0 && m.cursor < len(m.tags) {
				return m, m.removeProjectTag(m.tags[m.cursor].Name)
			}
			return m, nil

		case "p":
			if len(m.tags) > 0 && m.cursor < len(m.tags) {
				return m, m.togglePinnedTag(m.tags[m.cursor].Name)
			}
			return m, nil

		case "a":
			// Enter add tag mode
			m.addTagMode = true
			m.addTagInput.SetValue("")
			m.addTagInput.Focus()
			return m, textinput.Blink
		}
	}

	return m, nil
}

// handleRenameMode handles keyboard input while in rename mode.
func (m *TagManagerModel) handleRenameMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if strings.TrimSpace(m.renameInput.Value()) != m.renameTarget {
			m.confirmDiscard = true
			m.confirmAction = "discard-rename"
			return m, nil
		}
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, nil

	case "enter":
		newName := models.NormalizeTag(m.renameInput.Value())
		if newName != "" && newName != m.renameTarget {
			return m, m.renameTag(m.renameTarget, newName)
		}
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, nil

	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// handleAddTagMode handles keyboard input while in add tag mode.
func (m *TagManagerModel) handleAddTagMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if strings.TrimSpace(m.addTagInput.Value()) != "" {
			m.confirmDiscard = true
			m.confirmAction = "discard-add"
			return m, nil
		}
		m.addTagMode = false
		m.addTagInput.SetValue("")
		m.addTagInput.Blur()
		return m, nil

	case "enter":
		tagName := models.NormalizeTag(m.addTagInput.Value())
		if tagName == "" {
			m.errorMsg = "Tag name cannot be empty"
			return m, nil
		}
		if err := models.ValidateTag(tagName); err != nil {
			m.errorMsg = err.Error()
			return m, nil
		}

		if m.project != nil && m.project.HasTag(tagName) {
			m.errorMsg = fmt.Sprintf("Tag '%s' already exists", tagName)
			return m, nil
		}

		return m, m.addProjectTag(tagName)

	default:
		var cmd tea.Cmd
		m.addTagInput, cmd = m.addTagInput.Update(msg)
		return m, cmd
	}
}

// View renders the tag manager view.
func (m *TagManagerModel) View() tea.View {
	// Build content
	var content strings.Builder

	// Show add tag input if in add mode
	if m.addTagMode {
		content.WriteString(MutedStyle.Render("Add new tag:"))
		content.WriteString("\n")
		content.WriteString(m.addTagInput.View())
		content.WriteString("\n\n")
	}

	if len(m.tags) == 0 {
		content.WriteString(MutedStyle.Render("No tags in project"))
		content.WriteString("\n")
	} else {
		for i, tag := range m.tags {
			cursor := "  "
			style := TagManagerItemStyle
			if i == m.cursor {
				cursor = "  "
				style = TagManagerItemSelectedStyle
			}

			// Check if this is the tag being renamed
			if m.renameMode && tag.Name == m.renameTarget {
				content.WriteString(cursor)
				content.WriteString("Rename to: ")
				content.WriteString(m.renameInput.View())
				content.WriteString("\n")
			} else {
				pinMarker := "  "
				if tag.Pinned {
					pinMarker = "* "
				}
				line := fmt.Sprintf("%s%s%s ", cursor, pinMarker, tag.Name)
				content.WriteString(style.Render(line))
				countStr := fmt.Sprintf("(%d)", tag.TaskCount)
				content.WriteString(TagManagerCountStyle.Render(countStr))
				content.WriteString("\n")
			}
		}
	}

	// Error message
	if m.errorMsg != "" {
		content.WriteString("\n")
		content.WriteString(TagManagerErrorStyle.Render(m.errorMsg))
		content.WriteString("\n")
	}

	// Build subtitle
	subtitle := ""
	if len(m.tags) > 0 {
		subtitle = fmt.Sprintf("%d tags", len(m.tags))
	}

	// Footer
	var helpText string
	if m.addTagMode {
		helpText = "enter add tag · esc cancel"
	} else if m.renameMode {
		helpText = "enter confirm · esc cancel"
	} else if len(m.tags) > 0 {
		helpText = "j/k nav · enter filter · p pin · a add · r rename · d del · Esc close"
	} else {
		helpText = "a add tag · Esc close"
	}

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Tag Manager", subtitle))
	out.WriteString("\n\n")
	out.WriteString(content.String())
	out.WriteString("\n")
	out.WriteString(HelpStyle.Render(renderWrappingHelpLine(helpText, m.width)))

	return tea.NewView(out.String())
}

func (m *TagManagerModel) HasTextInput() bool { return m.renameMode || m.addTagMode }
func (m *TagManagerModel) ShouldCaptureEsc() bool {
	return m.renameMode || m.addTagMode || m.confirmDiscard
}
func (m *TagManagerModel) IsFullHeight() bool { return false }

func (m *TagManagerModel) StatusBarLabel() string { return "Tag Manager" }
