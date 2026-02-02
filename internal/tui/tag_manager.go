package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// TagInfo holds a tag name and its usage count.
type TagInfo struct {
	Name      string
	TaskCount int
}

// TagManagerModel is the model for the tag management view.
type TagManagerModel struct {
	projectTree  *ProjectTreeModel
	tags         []TagInfo
	cursor       int
	width        int
	height       int
	renameMode   bool
	renameInput  textinput.Model
	renameTarget string
	addTagMode   bool
	addTagInput  textinput.Model
	errorMsg     string
}

// ShowTagManagerMsg is sent to open the tag manager view.
type ShowTagManagerMsg struct {
	ProjectTree *ProjectTreeModel
}

// CloseTagManagerMsg is sent when the tag manager should close.
type CloseTagManagerMsg struct{}

// TagRenamedMsg is sent after a tag has been renamed.
type TagRenamedMsg struct {
	OldName string
	NewName string
}

// NewTagManagerModel creates a new tag manager model.
func NewTagManagerModel(pt *ProjectTreeModel) *TagManagerModel {
	renameInput := textinput.New()
	renameInput.Placeholder = "new tag name"
	renameInput.CharLimit = 50
	renameInput.Width = 30

	addTagInput := textinput.New()
	addTagInput.Placeholder = "tag name"
	addTagInput.CharLimit = 50
	addTagInput.Width = 30

	m := &TagManagerModel{
		projectTree: pt,
		cursor:      0,
		renameInput: renameInput,
		addTagInput: addTagInput,
	}
	m.buildTagList()
	return m
}

// SetSize sets the view dimensions.
func (m *TagManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildTagList scans all tasks and builds a sorted list of tags with counts.
func (m *TagManagerModel) buildTagList() {
	tagCounts := make(map[string]int)
	for _, task := range m.projectTree.project.Tasks {
		for _, tag := range task.Tags {
			tagCounts[tag]++
		}
	}

	m.tags = make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		m.tags = append(m.tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(m.tags, func(i, j int) bool {
		return m.tags[i].Name < m.tags[j].Name
	})

	// Clamp cursor
	if m.cursor >= len(m.tags) {
		m.cursor = len(m.tags) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// renameTag renames a tag across all tasks.
func (m *TagManagerModel) renameTag(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		newName = strings.TrimSpace(newName)
		if newName == "" || newName == oldName {
			return nil
		}

		for _, task := range m.projectTree.project.Tasks {
			for i, tag := range task.Tags {
				if tag == oldName {
					task.Tags[i] = newName
					break
				}
			}
		}

		if err := saveProjectSync(m.projectTree.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TagRenamedMsg{OldName: oldName, NewName: newName}
	}
}

// filterByTag sets the project tree filter and closes the tag manager.
func (m *TagManagerModel) filterByTag(tagName string) tea.Cmd {
	return func() tea.Msg {
		m.projectTree.filter.Tags = map[string]bool{tagName: true}
		m.projectTree.updateVisibility()
		m.projectTree.clampCursor()
		return CloseTagManagerMsg{}
	}
}

// Init initializes the model.
func (m *TagManagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TagManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case TagRenamedMsg:
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		// Close the tag manager after successful rename
		return m, func() tea.Msg {
			return CloseTagManagerMsg{}
		}

	case tea.KeyMsg:
		// Clear error on any key press
		m.errorMsg = ""

		if m.renameMode {
			return m.handleRenameMode(msg)
		}

		if m.addTagMode {
			return m.handleAddTagMode(msg)
		}

		switch msg.String() {
		case "q", "esc", "ctrl+t":
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
				tag := m.tags[m.cursor]
				if tag.TaskCount > 0 {
					m.errorMsg = fmt.Sprintf("Cannot delete: tag has %d task(s)", tag.TaskCount)
				} else {
					// Tag has 0 tasks, just rebuild to remove it
					m.buildTagList()
				}
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
func (m *TagManagerModel) handleRenameMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, nil

	case "enter":
		newName := strings.TrimSpace(m.renameInput.Value())
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
func (m *TagManagerModel) handleAddTagMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.addTagMode = false
		m.addTagInput.SetValue("")
		m.addTagInput.Blur()
		return m, nil

	case "enter":
		tagName := strings.TrimSpace(m.addTagInput.Value())
		if tagName == "" {
			m.errorMsg = "Tag name cannot be empty"
			return m, nil
		}

		// Check for duplicate (case-sensitive)
		for _, tag := range m.tags {
			if tag.Name == tagName {
				m.errorMsg = fmt.Sprintf("Tag '%s' already exists", tagName)
				return m, nil
			}
		}

		// Add the new tag to the list
		m.tags = append(m.tags, TagInfo{Name: tagName, TaskCount: 0})

		// Re-sort tags
		sort.Slice(m.tags, func(i, j int) bool {
			return m.tags[i].Name < m.tags[j].Name
		})

		// Find and select the newly added tag
		for i, tag := range m.tags {
			if tag.Name == tagName {
				m.cursor = i
				break
			}
		}

		// Exit add mode
		m.addTagMode = false
		m.addTagInput.SetValue("")
		m.addTagInput.Blur()
		return m, nil

	default:
		var cmd tea.Cmd
		m.addTagInput, cmd = m.addTagInput.Update(msg)
		return m, cmd
	}
}

// View renders the tag manager view.
func (m *TagManagerModel) View() string {
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
				cursor = "> "
				style = TagManagerItemSelectedStyle
			}

			// Check if this is the tag being renamed
			if m.renameMode && tag.Name == m.renameTarget {
				content.WriteString(cursor)
				content.WriteString("Rename to: ")
				content.WriteString(m.renameInput.View())
				content.WriteString("\n")
			} else {
				line := fmt.Sprintf("%s%s ", cursor, tag.Name)
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
		helpText = "enter add tag | esc cancel"
	} else if m.renameMode {
		helpText = "enter confirm | esc cancel"
	} else if len(m.tags) > 0 {
		helpText = "j/k nav | enter filter | a add | r rename | d del | q close"
	} else {
		helpText = "a add tag | q close"
	}

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Tag Manager", subtitle))
	out.WriteString("\n\n")
	out.WriteString(content.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(HelpStyle.Render(helpText), "", m.width))

	return out.String()
}
